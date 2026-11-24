import Phaser from 'phaser'

import type { MapData } from '@/realtime/mapdata'
import { directionOf, moveToward, SEND_INTERVAL_MS, step, type Vec } from '@/realtime/movement'
import type { BuildingCount, Direction, MoveMsg } from '@/realtime/messages'
import { findPath, nearestOpen } from '@/realtime/pathfind'
import { roomLayout, shelfAt, stationAt } from './roomLayout'
import { drawStations } from './stationArt'
import type { World } from '@/realtime/world'
import { avatarKey } from './avatars'
import { avatarSheet, frameIndex, walkFrame, FOOT_PAD } from './avatarPaint'
import { cssToken } from './tiles'
import { campusArt } from './campusArt'
import { roomArt, roomLandmarks } from './roomArt'
import { stackLabels } from './layout'
import { CONTENT, ZOOM_DEFAULT, cameraZoom } from './mapZoom'
import { drawWorld, preloadWorld, type BuildingLabels } from './worldArt'
import { badgeFor, visibleNames } from './mapLabels'

export interface SceneDeps {
  map: MapData
  world: World
  sendMove: (m: MoveMsg) => void
  zoom?: () => number
  selectBuilding?: (key: string) => void
  // walkToBuilding walks to a building door (double-click on the building).
  walkToBuilding?: (key: string) => void
  // selectShelf opens the library catalog on a shelf category (library rooms only).
  selectShelf?: (cat: string) => void
  selectStation?: (id: string) => void
  station?: () => string
  // shelf returns the selected category so the map can highlight it.
  shelf?: () => string
  // currentZone returns the server-reported zone key of the local avatar.
  currentZone?: () => string
  // speaking returns ids (self included) talking right now (voice, slice 9).
  speaking?: () => Set<string>
  // inset is the screen space (px) covered by the floating toolbars; the map
  // is fitted inside it so the toolbars never hide the floor.
  inset?: () => { top: number; bottom: number }
  // walkTarget is set by the UI ("Keluar ruang") to walk to a map point.
  walkTarget?: () => { x: number; y: number; id: number } | null
  // counts returns the live people per building (server data); the campus labels show it as a badge.
  counts?: () => BuildingCount[]
}


interface Sprite {
  body: Phaser.GameObjects.Image
  tag: Phaser.GameObjects.Container
  name: string
  lastX: number
  lastY: number
  travelled: number
  moving: boolean
  lastMoveAt: number
}

const AVATAR_SIZE = 36
// Extra A* cost per tile inside a table you are not walking to (see tablePenalty).
const TABLE_PENALTY = 20
const MAX_FRAME_MS = 100
const DOUBLE_CLICK_MS = 400
const DOUBLE_CLICK_PX = 24
// A press that moves further than this pans the campus instead of clicking.
const DRAG_PX = 6

// clampCentre keeps the view inside the map: a map smaller than the view stays centred.
export function clampCentre(size: number, view: number, centre: number) {
  if (size <= view) return size / 2
  return Math.min(size - view / 2, Math.max(view / 2, centre))
}

// CampusScene draws the map and avatars from World and turns keyboard and
// clicks into predicted local movement plus `move` messages.
export class CampusScene extends Phaser.Scene {
  private deps!: SceneDeps
  private sprites = new Map<string, Sprite>()
  private self: Sprite | null = null
  private keys!: Record<
    'up' | 'down' | 'left' | 'right' | 'w' | 'a' | 's' | 'd',
    Phaser.Input.Keyboard.Key
  >
  private path: Vec[] = []
  private lastClick: { time: number; x: number; y: number; world: Vec; building?: string } | null = null
  private pendingSelect: { run: () => void; timer: ReturnType<typeof setTimeout> } | null = null
  // Campus camera: fixed (no follow); the user pans it by dragging.
  private centre: Vec | null = null
  private press: { x: number; y: number; centre: Vec; dragged: boolean } | null = null
  private lastSent = 0
  private lastSentPos: { x: number; y: number; dir: Direction } | null = null
  private labels: Phaser.GameObjects.Container[] = []
  private zoneLayer: Phaser.GameObjects.Graphics | null = null
  private buildingLabels: BuildingLabels = new Map()
  private badgeKey = ''
  // hoveredId: the other player under the pointer on the campus (their name shows while hovered).
  private hoveredId: string | null = null
  private zoneLabels: Phaser.GameObjects.Text[] = []
  private drawnZone = '?'
  private radius: Phaser.GameObjects.Graphics | null = null
  private rings: Phaser.GameObjects.Graphics | null = null
  private focused = false
  private movementRevision = 0
  private walkId = 0
  private fitted = ''
  // calm: the user asked for less motion, so avatars keep one pose.
  private calm = typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches
  private emoteBubbles = new Map<string, Phaser.GameObjects.Container>()
  private shelfShown = ''
  private shelfRing: Phaser.GameObjects.Graphics | null = null

  constructor() {
    super('campus')
  }

  init(deps: SceneDeps) {
    this.deps = deps
  }

  preload() {
    preloadWorld(this)
  }

  create() {
    const { map } = this.deps
    this.centre = null
    const newArt = drawWorld(this, this.deps.world.mapKey ?? 'campus', this.buildingLabels)
    drawStations(this, this.deps.world.mapKey ?? 'campus')
    if (!newArt) {
    this.textures.addCanvas(
      'campus-art',
      this.deps.world.mapKey === 'campus'
        ? campusArt(map)
        : roomArt(map, this.deps.world.mapKey ?? 'classroom'),
    )
    this.add.image(0, 0, 'campus-art').setOrigin(0).setDisplaySize(map.pixelWidth, map.pixelHeight)
    }
    this.fitCamera()
    this.scale.on('resize', this.fitCamera, this)
    this.events.once('shutdown', () => {
      this.scale.off('resize', this.fitCamera, this)
      if (this.pendingSelect) clearTimeout(this.pendingSelect.timer)
      this.pendingSelect = null
    })

    if (!newArt) for (const b of map.buildings)
      this.labels.push(this.buildingLabel(b.name, b.x + b.w / 2, b.y, b.staffOnly))
    // Campus reuses the board tile id for the flagpole; landmarks are room-only.
    if (!newArt && this.deps.world.mapKey !== 'campus')
      for (const l of roomLandmarks(map))
        this.labels.push(this.buildingLabel(l.text, l.x, l.y, false, l.dark))
    if (map.zones.length) {
      this.zoneLayer = this.add.graphics().setDepth(1)
      for (const z of map.zones) {
        // Bottom-left: seats sit at the top of a table, where a label would be covered.
        const label = this.add
          .text(z.x + 8, z.y + z.h - 6, z.name, {
            fontFamily: cssToken('--font-body'),
            fontSize: cssToken('--text-sm'),
            fontStyle: '700',
            color: cssToken('--color-accent-ink'),
          })
          .setOrigin(0, 1)
          .setResolution(Math.min(window.devicePixelRatio || 1, 2) * 2)
          .setDepth(2)
        this.zoneLabels.push(label)
      }
    }
    this.radius = this.add.graphics().setDepth(1.5)
    this.rings = this.add.graphics().setDepth(1.6)

    this.cameras.main.setBackgroundColor(cssToken('--kv-map-ground'))

    const kb = this.input.keyboard!
    const K = Phaser.Input.Keyboard.KeyCodes
    this.keys = {
      up: kb.addKey(K.UP),
      down: kb.addKey(K.DOWN),
      left: kb.addKey(K.LEFT),
      right: kb.addKey(K.RIGHT),
      w: kb.addKey(K.W),
      a: kb.addKey(K.A),
      s: kb.addKey(K.S),
      d: kb.addKey(K.D),
    }
    const clearInput = () => {
      kb.resetKeys()
      this.path = []
      this.lastClick = null
    }
    this.game.canvas.parentElement?.addEventListener('blur', clearInput)
    this.events.once('shutdown', () =>
      this.game.canvas.parentElement?.removeEventListener('blur', clearInput),
    )
    this.input.on('pointerdown', this.onPointerDown, this)
    this.input.on('pointermove', this.onPointerMove, this)
    this.input.on('pointerup', this.onPointerUp, this)
  }

  private onPointerDown(p: Phaser.Input.Pointer) {
    if (!p.leftButtonDown()) return
    const cam = this.cameras.main
    this.press = { x: p.x, y: p.y, centre: this.centre ?? { x: cam.midPoint.x, y: cam.midPoint.y }, dragged: false }
  }

  // avatarAt returns the id of the other player standing at a world point (about an avatar's body), or null.
  private avatarAt(x: number, y: number): string | null {
    for (const a of this.deps.world.avatars?.values() ?? []) {
      if (!a.hidden && Math.abs(a.x - x) <= 16 && y >= a.y - 40 && y <= a.y + 6) return a.id
    }
    return null
  }

  private onPointerMove(p: Phaser.Input.Pointer) {
    if (this.deps.world.mapKey === 'campus') this.hoveredId = this.avatarAt(p.worldX, p.worldY)
    const press = this.press
    if (!press || !p.isDown || this.deps.world.mapKey !== 'campus') return
    if (!press.dragged && Math.hypot(p.x - press.x, p.y - press.y) <= DRAG_PX) return
    press.dragged = true
    this.lastClick = null
    const zoom = this.cameras.main.zoom
    this.panTo({ x: press.centre.x - (p.x - press.x) / zoom, y: press.centre.y - (p.y - press.y) / zoom })
  }

  private onPointerUp(p: Phaser.Input.Pointer) {
    const press = this.press
    this.press = null
    if (press && !press.dragged) this.onClick(p)
  }

  private panTo(to: Vec) {
    const { map } = this.deps
    const cam = this.cameras.main
    this.centre = {
      x: clampCentre(map.pixelWidth, this.scale.width / cam.zoom, to.x),
      y: clampCentre(map.pixelHeight, this.scale.height / cam.zoom, to.y),
    }
    cam.centerOn(this.centre.x, this.centre.y)
  }

  // onClick: one click selects a building/station/shelf. A double click (two clicks within
  // DOUBLE_CLICK_MS and DOUBLE_CLICK_PX on screen) walks: to the building door, next to the
  // station/shelf, or to the floor point. The second click reuses the first click's world target
  // because the first may open a panel that resizes the map under the pointer. Room
  // stations/shelves open a side panel that moves the map, so their selection waits
  // DOUBLE_CLICK_MS (or runs at once on the double click).
  private onClick(p: Phaser.Input.Pointer) {
    const self = this.deps.world.self
    if (!self) return
    const last = this.lastClick
    if (last && p.downTime - last.time <= DOUBLE_CLICK_MS && Math.hypot(p.x - last.x, p.y - last.y) <= DOUBLE_CLICK_PX) {
      this.lastClick = null
      this.flushSelect()
      if (last.building) this.deps.walkToBuilding?.(last.building)
      else this.walkNear(self, last.world)
      return
    }
    const to = { x: Math.round(p.worldX), y: Math.round(p.worldY) }
    this.lastClick = { time: p.downTime, x: p.x, y: p.y, world: to }
    const mapKey = this.deps.world.mapKey ?? ''
    const building = mapKey === 'campus' && this.deps.selectBuilding
      ? this.deps.map.buildings.find((b) => to.x >= b.x && to.x < b.x+b.w && to.y >= b.y && to.y < b.y+b.h)
      : undefined
    if (building) {
      this.lastClick.building = building.key
      this.path = []
      this.deps.selectBuilding!(building.key)
      return
    }
    const station = this.deps.selectStation && stationAt(mapKey, to.x, to.y)
    this.flushSelect()
    if (station) {
      this.deferSelect(() => this.deps.selectStation!(station.id))
      return
    }
    const cat = this.deps.selectShelf && shelfAt(mapKey, to.x, to.y)
    if (cat) this.deferSelect(() => this.deps.selectShelf!(cat))
  }

  private deferSelect(run: () => void) {
    this.pendingSelect = { run, timer: setTimeout(() => this.flushSelect(), DOUBLE_CLICK_MS) }
  }

  private flushSelect() {
    const pending = this.pendingSelect
    if (!pending) return
    this.pendingSelect = null
    clearTimeout(pending.timer)
    pending.run()
  }

  // walkNear paths to `to`, or to the closest free tile when it is furniture.
  private walkNear(self: Vec, to: Vec) {
    const goal = nearestOpen(this.deps.map, to)
    this.path = goal ? findPath(this.deps.map, self, goal, this.tablePenalty(self, goal)) : []
  }

  // syncBadges shows the server's people count on each building label; it only redraws when a figure changed.
  private syncBadges() {
    const counts = this.deps.counts?.()
    if (!counts || this.buildingLabels.size === 0) return
    const key = counts.map((c) => `${c.building_key}:${c.people}`).join('|')
    if (key === this.badgeKey) return
    this.badgeKey = key
    for (const [id, label] of this.buildingLabels) label.setBadge(badgeFor(counts, id))
  }

  update(time: number, delta: number) {
    const { world, map } = this.deps
    if (this.movementRevision !== world.movementRevision) {
      this.movementRevision = world.movementRevision
      this.lastClick = null
      this.path = []
      this.input.keyboard?.resetKeys()
      this.lastSentPos = null
    }
    const dt = Math.min(delta, MAX_FRAME_MS)
    const self = world.self
    const target = this.deps.walkTarget?.()
    if (self && target && target.id !== this.walkId) {
      this.walkId = target.id
      this.path = findPath(map, self, target, this.tablePenalty(self, target))
    }
    this.highlightShelf()
    this.syncBadges()
    const inset = this.deps.inset?.()
    if ((this.deps.world.mapKey === 'campus' &&
      this.cameras.main.zoom !== this.campusZoom()) ||
      (inset && `${inset.top},${inset.bottom}` !== this.fitted)) this.fitCamera()

    this.focused = this.game.canvas.parentElement === document.activeElement
    // A UI walk ("Keluar ruang") runs without canvas focus; keys still need it.
    if (self && (this.focused || this.path.length > 0)) {
      const k = this.keys
      const dx = (k.right.isDown || k.d.isDown ? 1 : 0) - (k.left.isDown || k.a.isDown ? 1 : 0)
      const dy = (k.down.isDown || k.s.isDown ? 1 : 0) - (k.up.isDown || k.w.isDown ? 1 : 0)
      let next: Vec = self
      if (dx !== 0 || dy !== 0) {
        this.path = []
        next = step(map, self, dx, dy, dt)
        self.dir = directionOf(dx, dy, self.dir)
      } else if (this.path.length > 0) {
        const target = this.path[0]!
        next = moveToward(map, self, target, dt)
        if (next.x === self.x && next.y === self.y) this.path = []
        else self.dir = directionOf(target.x - self.x, target.y - self.y, self.dir)
        if (next.x === target.x && next.y === target.y) this.path.shift()
      }
      self.x = next.x
      self.y = next.y
      this.maybeSend(time, self.x, self.y, self.dir)
    }

    world.sample(performance.now())
    this.syncSprites()
    this.drawZones()
    this.drawRadius()
    this.drawSpeaking()
  }

  // tablePenalty makes walks go around other tables. Every table is its own
  // voice channel, so cutting through one costs a voice reconnect (and holds
  // the avatar until the old audio is confirmed gone).
  private tablePenalty(from: Vec, to: Vec) {
    const { map } = this.deps
    const ts = map.tileSize
    const at = (x: number, y: number) =>
      map.zones.find((z) => z.circle
        ? ((x-z.x-z.w/2)/(z.w/2))**2 + ((y-z.y-z.h/2)/(z.h/2))**2 <= 1
        : x >= z.x && x < z.x + z.w && y >= z.y && y < z.y + z.h)
    const keep = new Set([at(from.x, from.y), at(to.x, to.y)])
    return (tx: number, ty: number) => {
      const z = at(tx * ts + ts / 2, ty * ts + ts / 2)
      return z && !keep.has(z) ? TABLE_PENALTY : 0
    }
  }

  // drawZones outlines discussion tables: dashed Mangga, the player's own zone
  // solid and thicker (docs/visual.md). Redrawn only when the zone changes.
  private drawZones() {
    const g = this.zoneLayer
    if (!g) return
    const mine = this.deps.currentZone?.() ?? ''
    if (mine === this.drawnZone) return
    this.drawnZone = mine
    g.clear()
    const line = Phaser.Display.Color.HexStringToColor(cssToken('--color-accent-line')).color
    const fill = Phaser.Display.Color.HexStringToColor(cssToken('--color-accent-soft')).color
    const strong = Phaser.Display.Color.HexStringToColor(cssToken('--color-accent')).color
    this.deps.map.zones.forEach((z, i) => {
      const own = z.key === mine
      g.fillStyle(fill, own ? 0.75 : 0.35)
      if (z.circle) {
        g.fillEllipse(z.x+z.w/2, z.y+z.h/2, z.w, z.h)
        g.lineStyle(own ? 4 : 2, own ? strong : line, 1)
        g.strokeEllipse(z.x+z.w/2, z.y+z.h/2, z.w, z.h)
        this.zoneLabels[i]?.setText(own ? `${z.name} · kamu di sini` : z.name)
        return
      }
      g.fillRoundedRect(z.x, z.y, z.w, z.h, 16)
      if (own) {
        g.lineStyle(4, strong, 1)
        g.strokeRoundedRect(z.x, z.y, z.w, z.h, 16)
      } else {
        dashedRect(g, z.x, z.y, z.w, z.h, line)
      }
      this.zoneLabels[i]?.setText(own ? `${z.name} · kamu di sini` : z.name)
    })
  }

  // drawSpeaking rings active speakers with --color-speaking (docs/visual.md).
  private drawSpeaking() {
    const g = this.rings
    if (!g) return
    g.clear()
    const ids = this.deps.speaking?.()
    if (!ids?.size) return
    const zoom = this.cameras.main.zoom
    g.lineStyle(3 / zoom, Phaser.Display.Color.HexStringToColor(cssToken('--color-speaking')).color, 1)
    const r = (AVATAR_SIZE / 2 + 4) / zoom
    // Sprites are anchored at the feet (origin 0.5, 0.9); centre the ring on the body.
    const lift = AVATAR_SIZE * 0.4 / zoom
    const { world } = this.deps
    for (const a of world.avatars.values()) {
      if (ids.has(a.id) && !a.hidden) g.strokeCircle(a.x, a.y - lift, r)
    }
    if (world.self && world.selfId && ids.has(world.selfId)) g.strokeCircle(world.self.x, world.self.y - lift, r)
  }

  // drawRadius shows the near-chat range around the local avatar.
  private drawRadius() {
    const g = this.radius
    const self = this.deps.world.self
    if (!g) return
    g.clear()
    if (!self) return
    const c = Phaser.Display.Color.HexStringToColor(cssToken('--color-primary')).color
    const segments = 48
    g.lineStyle(2 / this.cameras.main.zoom, c, 0.6)
    for (let i = 0; i < segments; i += 2) {
      const a0 = (i / segments) * Math.PI * 2
      const a1 = ((i + 1) / segments) * Math.PI * 2
      g.beginPath()
      g.arc(self.x, self.y, this.deps.map.nearRadius, a0, a1)
      g.strokePath()
    }
  }

  // campusZoom: the user's factor (1 = whole campus in view) for the current canvas size.
  private campusZoom() {
    return cameraZoom(this.deps.zoom?.() ?? ZOOM_DEFAULT, this.scale.width, this.scale.height)
  }

  private fitCamera() {
    const { map } = this.deps
    const camera = this.cameras.main
    const inset = this.deps.inset?.() ?? { top: 0, bottom: 0 }
    this.fitted = `${inset.top},${inset.bottom}`
    const free = Math.max(1, this.scale.height - inset.top - inset.bottom)
    const campus = this.deps.world.mapKey === 'campus'
    const zoom = campus
      ? this.campusZoom()
      : Math.min(this.scale.width / map.pixelWidth, free / map.pixelHeight)
    // Centre the map in the band between the toolbars, not the whole canvas.
    const shift = (inset.top - inset.bottom) / 2 / zoom
    camera.setZoom(zoom)
    // Campus opens on the whole campus (every building in view), then stays where the user panned it; zoom keeps
    // that centre. Zoomed in past the overview, it opens on the avatar instead.
    const overview = { x: CONTENT.x + CONTENT.w / 2, y: CONTENT.y + CONTENT.h / 2 }
    const start = this.centre ?? ((this.deps.zoom?.() ?? ZOOM_DEFAULT) > ZOOM_DEFAULT ? this.deps.world.self : overview)
    if (campus && start) this.panTo(start)
    else camera.centerOn(map.pixelWidth / 2, map.pixelHeight / 2 - (campus ? 0 : shift))
    for (const child of this.children.list) {
      if (child.name === 'world-label') (child as Phaser.GameObjects.Container).setScale(1 / zoom)
    }
    for (const label of this.labels) label.setScale(1 / zoom)
    for (const s of [...this.sprites.values(), ...(this.self ? [this.self] : [])]) {
      s.tag.setScale(1 / zoom)
      this.sizeAvatar(s.body)
    }
  }

  private maybeSend(time: number, x: number, y: number, dir: Direction) {
    const last = this.lastSentPos
    if (last && last.x === x && last.y === y && last.dir === dir) return
    if (time - this.lastSent < SEND_INTERVAL_MS) return
    this.lastSent = time
    this.lastSentPos = { x, y, dir }
    this.deps.sendMove({ x: Math.round(x), y: Math.round(y), dir })
  }

  private syncSprites() {
    const { world } = this.deps
    // On the busy campus only nearby or hovered people carry a name; rooms keep every name.
    const campus = world.mapKey === 'campus'
    const named = campus
      ? visibleNames(world.self, [...world.avatars.values()].filter((a) => !a.hidden), this.hoveredId)
      : new Set<string>()
    for (const [id, s] of this.sprites) {
      const a = world.avatars.get(id)
      if (!a) {
        s.body.destroy()
        s.tag.destroy()
        this.sprites.delete(id)
        continue
      }
      this.drawAvatar(s.body, a.avatar)
      if (s.name !== a.name) {
        s.tag.destroy()
        s.tag = this.makeTag(a.name, a.role === 'participant' ? 'other' : 'teacher')
        s.name = a.name
      }
      s.body.setVisible(!a.hidden).setPosition(a.x, a.y).setDepth(a.y)
      this.animateAvatar(s, a.x, a.y, a.dir, this.time.now)
      s.tag
        .setVisible(!a.hidden && (!campus || named.has(id)))
        .setPosition(a.x, a.y + 6)
        .setDepth(a.y + 1)
    }
    for (const a of world.avatars.values()) {
      if (this.sprites.has(a.id)) continue
      const tagKind = a.role === 'teacher' || a.role === 'admin' ? 'teacher' : 'other'
      this.sprites.set(a.id, this.makeSprite(a.name, tagKind, a.x, a.y, a.avatar))
    }
    const self = world.self
    if (self) {
      if (!this.self) {
        this.self = this.makeSprite(world.selfName, 'self', self.x, self.y, world.selfAvatar)
        this.fitCamera()
      }
      this.drawAvatar(this.self.body, world.selfAvatar)
      if (this.self.name !== world.selfName) {
        this.self.tag.destroy()
        this.self.tag = this.makeTag(world.selfName, 'self')
        this.self.name = world.selfName
      }
      this.self.body.setPosition(self.x, self.y).setDepth(self.y)
      this.animateAvatar(this.self, self.x, self.y, self.dir, this.time.now)
      this.self.tag.setPosition(self.x, self.y + 6).setDepth(self.y + 1)
    }
    this.unstackTags()
    this.syncEmotes()
  }

  private syncEmotes() {
    const { world } = this.deps
    for (const [id, bubble] of this.emoteBubbles) {
      if (!world.emotes.has(id) || (id !== world.selfId && !world.avatars.has(id))) {
        bubble.destroy()
        this.emoteBubbles.delete(id)
      }
    }
    for (const [id, emote] of world.emotes) {
      const avatar = id === world.selfId ? world.self : world.avatars.get(id)
      if (!avatar) continue
      let bubble = this.emoteBubbles.get(id)
      if (bubble?.getData('text') !== emote.text) {
        bubble?.destroy()
        bubble = this.pill(emote.text, cssToken('--color-primary'), cssToken('--color-surface'), cssToken('--text-sm'))
        bubble.setData('text', emote.text)
        this.emoteBubbles.set(id, bubble)
      }
      const zoom = this.cameras.main.zoom
      bubble!.setScale(1 / zoom).setPosition(avatar.x, avatar.y - 66 / zoom)
        .setDepth(100001).setVisible(id === world.selfId || !world.avatars.get(id)?.hidden)
    }
  }

  // unstackTags moves overlapping name tags apart; positions come from syncSprites.
  private unstackTags() {
    const tags = [...this.sprites.values(), ...(this.self ? [this.self] : [])]
      .map((s) => s.tag)
      .filter((t) => t.visible)
    if (tags.length < 2) return
    const boxes = tags.map((t) => {
      const size = t.getData('size') as { w: number; h: number }
      return { x: t.x, y: t.y, w: size.w * t.scaleX, h: size.h * t.scaleY }
    })
    stackLabels(boxes).forEach((y, i) => tags[i]!.setY(y))
  }

  private makeSprite(
    name: string,
    kind: 'self' | 'teacher' | 'other',
    x: number,
    y: number,
    avatar = 'default',
  ): Sprite {
    const body = this.add.image(x, y, this.ensureAvatar(avatar), 0)
    this.sizeAvatar(body)
    const tag = this.makeTag(name, kind)
    tag.setPosition(x, y + 6)
    return { body, tag, name, lastX: x, lastY: y, travelled: 0, moving: false, lastMoveAt: 0 }
  }

  // ensureAvatar builds the walking sheet of a saved avatar key once; unknown keys fall back to the default look.
  private ensureAvatar(value: string) {
    const key = avatarKey(value), texture = `avatar-${key}`
    if (!this.textures.exists(texture)) {
      const sheet = avatarSheet(key)
      const tex = this.textures.addCanvas(texture, sheet.canvas)
      // Frames are 2x supersampled, so the frame size in the sheet is twice the logical size.
      const w = sheet.frameW * 2, h = sheet.frameH * 2
      for (let r = 0; r < sheet.rows; r++) for (let c = 0; c < sheet.cols; c++) tex?.add(r * sheet.cols + c, 0, c * w, r * h, w, h)
    }
    return texture
  }

  // Logical avatar box: the frame is 40x64 with the feet FOOT_PAD above its bottom edge.
  private sizeAvatar(body: Phaser.GameObjects.Image) {
    const zoom = this.cameras.main.zoom
    body.setOrigin(0.5, 1 - FOOT_PAD / 64).setDisplaySize(40 / zoom, 64 / zoom)
  }

  private drawAvatar(body: Phaser.GameObjects.Image, key: string) {
    const texture = this.ensureAvatar(key)
    if (body.texture.key !== texture) {
      body.setTexture(texture, 0)
      this.sizeAvatar(body)
    }
  }

  // animateAvatar picks the pose from the direction and the distance walked since the last frame, so the legs
  // follow real movement (also other players') and stop the moment someone stands still.
  private animateAvatar(sprite: Sprite, x: number, y: number, dir: Direction, now: number) {
    const d = Math.hypot(x - sprite.lastX, y - sprite.lastY)
    sprite.lastX = x; sprite.lastY = y
    if (d > 0.2) { sprite.travelled += d; sprite.lastMoveAt = now }
    const moving = now - sprite.lastMoveAt < 120
    sprite.moving = moving
    const frame = this.calm ? 0 : walkFrame(moving, sprite.travelled)
    sprite.body.setFrame(frameIndex(dir, frame))
  }

  private makeTag(name: string, kind: 'self' | 'teacher' | 'other') {
    const bg =
      kind === 'self' ? '--color-ink' : kind === 'teacher' ? '--color-primary' : '--color-surface'
    const fg = kind === 'other' ? '--color-ink' : '--color-surface'
    const tag = this.pill(name, cssToken(bg), cssToken(fg), cssToken('--text-xs'))
    tag.setScale(1 / this.cameras.main.zoom)
    return tag
  }

  private buildingLabel(name: string, x: number, y: number, locked: boolean, dark = false) {
    const c = this.pill(
      name,
      cssToken(dark ? '--color-ink' : '--color-surface'),
      cssToken(dark ? '--color-surface' : '--color-ink'),
      cssToken('--text-md'),
      locked,
    )
    c.setPosition(x, y - 4).setDepth(100000)
    c.setScale(1 / this.cameras.main.zoom)
    return c
  }

  // pill draws a rounded label; `locked` adds a line-drawn padlock (no emoji in UI).
  private pill(
    text: string,
    bg: string,
    fg: string,
    size: string,
    locked = false,
  ): Phaser.GameObjects.Container {
    const icon = locked ? 14 : 0
    const t = this.add
      .text(icon / 2, 0, text, {
        fontFamily: cssToken('--font-body'),
        fontSize: size,
        fontStyle: '700',
        color: fg,
      })
      .setOrigin(0.5, 0)
      .setResolution(Math.min(window.devicePixelRatio || 1, 2) * 2)
    const padX = 8
    const padY = 3
    const w = t.width + icon + padX * 2
    const h = t.height + padY * 2
    const g = this.add.graphics()
    g.fillStyle(Phaser.Display.Color.HexStringToColor(bg).color, 1)
    g.fillRoundedRect(-w / 2, -padY, w, h, h / 2)
    const parts: Phaser.GameObjects.GameObject[] = [g, t]
    if (locked) {
      const fgc = Phaser.Display.Color.HexStringToColor(fg).color
      const lx = -w / 2 + padX
      const ly = t.height / 2
      const lock = this.add.graphics()
      lock.lineStyle(2, fgc, 1)
      lock.strokeRoundedRect(lx, ly - 1, 10, 8, 2)
      lock.beginPath()
      lock.arc(lx + 5, ly - 1, 3.5, Math.PI, 0)
      lock.strokePath()
      parts.push(lock)
    }
    // Containers have no reliable bounds for Graphics children; keep the pill size.
    return this.add.container(0, 0, parts).setData('size', { w, h })
  }

  // highlightShelf outlines the shelf chosen in the catalog, like the reference design.
  private highlightShelf() {
    const station = this.deps.station?.() ?? ''
    const cat = this.deps.shelf?.() ?? ''
    const selected = station || cat
    if (selected === this.shelfShown) return
    this.shelfShown = selected
    this.shelfRing?.destroy()
    this.shelfRing = null
    const layout = roomLayout(this.deps.world.mapKey ?? '')
    const shelf = station ? layout?.stations.find((s) => s.id === station) : cat && layout?.shelves.find((s) => s.cat === cat)
    if (!shelf) return
    this.shelfRing = this.add.graphics().setDepth(shelf.y + shelf.h - 11)
      .lineStyle(4, Phaser.Display.Color.HexStringToColor(cssToken(station ? '--kv-star' : '--kv-subject-bahasa')).color)
      .strokeRoundedRect(shelf.x - 4, shelf.y - 4, shelf.w + 8, shelf.h + 8, 6)
  }
}

// dashedRect strokes a dashed rectangle (Phaser Graphics has no dash style).
function dashedRect(
  g: Phaser.GameObjects.Graphics,
  x: number,
  y: number,
  w: number,
  h: number,
  color: number,
) {
  const dash = 10
  const gap = 7
  g.lineStyle(3, color, 1)
  const edge = (x0: number, y0: number, x1: number, y1: number) => {
    const len = Math.hypot(x1 - x0, y1 - y0)
    for (let d = 0; d < len; d += dash + gap) {
      const t0 = d / len
      const t1 = Math.min(len, d + dash) / len
      g.lineBetween(
        x0 + (x1 - x0) * t0,
        y0 + (y1 - y0) * t0,
        x0 + (x1 - x0) * t1,
        y0 + (y1 - y0) * t1,
      )
    }
  }
  edge(x, y, x + w, y)
  edge(x + w, y, x + w, y + h)
  edge(x + w, y + h, x, y + h)
  edge(x, y + h, x, y)
}

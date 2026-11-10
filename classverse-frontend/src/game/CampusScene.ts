import Phaser from 'phaser'

import type { MapData } from '@/realtime/mapdata'
import { directionOf, moveToward, SEND_INTERVAL_MS, step, type Vec } from '@/realtime/movement'
import type { Direction, MoveMsg } from '@/realtime/messages'
import { findPath } from '@/realtime/pathfind'
import type { World } from '@/realtime/world'
import { avatarChoices, avatarKey, avatarURL, avatarSheetURL, avatarFrame } from './avatars'
import { cssToken } from './tiles'
import { campusArt } from './campusArt'
import { drawCampusAmbient } from './campusAmbient'
import { roomArt, roomLandmarks } from './roomArt'
import { stackLabels } from './layout'

export interface SceneDeps {
  map: MapData
  world: World
  sendMove: (m: MoveMsg) => void
  // currentZone returns the server-reported zone key of the local avatar.
  currentZone?: () => string
  // speaking returns ids (self included) talking right now (voice, slice 9).
  speaking?: () => Set<string>
  // inset is the screen space (px) covered by the floating toolbars; the map
  // is fitted inside it so the toolbars never hide the floor.
  inset?: () => { top: number; bottom: number }
  // walkTarget is set by the UI ("Keluar ruang") to walk to a map point.
  walkTarget?: () => { x: number; y: number; id: number } | null
  // lit returns building keys with people inside; their windows glow (W10-03).
  lit?: () => Set<string>
}


interface Sprite {
  body: Phaser.GameObjects.Image
  tag: Phaser.GameObjects.Container
  name: string
}

const AVATAR_SIZE = 36
const SPORTY_HEIGHT = 48
// Extra A* cost per tile inside a table you are not walking to (see tablePenalty).
const TABLE_PENALTY = 20
const MAX_FRAME_MS = 100

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
  private lastSent = 0
  private lastSentPos: { x: number; y: number; dir: Direction } | null = null
  private labels: Phaser.GameObjects.Container[] = []
  private zoneLayer: Phaser.GameObjects.Graphics | null = null
  private zoneLabels: Phaser.GameObjects.Text[] = []
  private drawnZone = '?'
  private radius: Phaser.GameObjects.Graphics | null = null
  private rings: Phaser.GameObjects.Graphics | null = null
  private focused = false
  private movementRevision = 0
  private walkId = 0
  private fitted = ''
  private litKeys = ''
  private ambient: Phaser.GameObjects.Graphics | null = null
  private ambientFrame = -1
  private art: Phaser.GameObjects.Image | null = null
  private reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)')

  constructor() {
    super('campus')
  }

  init(deps: SceneDeps) {
    this.deps = deps
  }

  preload() {
    const legacy = ['default', ...['round', 'cap', 'glasses', 'headset', 'bun', 'beret']
      .flatMap((style) => ['nila', 'mangga'].map((color) => `${style}-${color}`))]
    for (const key of legacy) this.load.svg(`avatar-${key}`, avatarURL(key), { width: 128, height: 128 })
    for (const { key } of avatarChoices) {
      this.load.spritesheet(`avatar-${key}`, avatarSheetURL(key)!, { frameWidth: 32, frameHeight: 48 })
    }
  }

  create() {
    const { map } = this.deps
    this.textures.addCanvas(
      'campus-art',
      this.deps.world.mapKey === 'campus'
        ? campusArt(map)
        : roomArt(map, this.deps.world.mapKey ?? 'classroom'),
    )
    this.art = this.add
      .image(0, 0, 'campus-art')
      .setOrigin(0)
      .setDisplaySize(map.pixelWidth, map.pixelHeight)
    if (this.deps.world.mapKey === 'campus') this.ambient = this.add.graphics().setDepth(0.5)
    this.fitCamera()
    this.scale.on('resize', this.fitCamera, this)
    this.events.once('shutdown', () => this.scale.off('resize', this.fitCamera, this))

    for (const b of map.buildings)
      this.labels.push(this.buildingLabel(b.name, b.x + b.w / 2, b.y, b.staffOnly))
    // Campus reuses the board tile id for the flagpole; landmarks are room-only.
    if (this.deps.world.mapKey !== 'campus')
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

    this.cameras.main.setBackgroundColor(cssToken('--map-grass'))

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
    }
    this.game.canvas.parentElement?.addEventListener('blur', clearInput)
    this.events.once('shutdown', () =>
      this.game.canvas.parentElement?.removeEventListener('blur', clearInput),
    )
    this.input.on('pointerdown', (p: Phaser.Input.Pointer) => {
      const self = this.deps.world.self
      if (!self) return
      const to = { x: Math.round(p.worldX), y: Math.round(p.worldY) }
      this.path = findPath(this.deps.map, self, to, this.tablePenalty(self, to))
    })
  }

  update(time: number, delta: number) {
    const { world, map } = this.deps
    if (this.movementRevision !== world.movementRevision) {
      this.movementRevision = world.movementRevision
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
    const inset = this.deps.inset?.()
    if (inset && `${inset.top},${inset.bottom}` !== this.fitted) this.fitCamera()
    this.relight()
    const ambientFrame = this.reducedMotion.matches ? -1 : Math.floor(time / 350) % 4
    if (this.ambient && ambientFrame !== this.ambientFrame) {
      this.ambientFrame = ambientFrame
      if (ambientFrame < 0) this.ambient.clear()
      else drawCampusAmbient(this.ambient, map, ambientFrame)
    }

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
    this.syncSprites(time)
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
      map.zones.find((z) => x >= z.x && x < z.x + z.w && y >= z.y && y < z.y + z.h)
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

  private fitCamera() {
    const { map } = this.deps
    const camera = this.cameras.main
    const inset = this.deps.inset?.() ?? { top: 0, bottom: 0 }
    this.fitted = `${inset.top},${inset.bottom}`
    const free = Math.max(1, this.scale.height - inset.top - inset.bottom)
    const zoom = Math.min(this.scale.width / map.pixelWidth, free / map.pixelHeight)
    // Centre the map in the band between the toolbars, not the whole canvas.
    const shift = (inset.top - inset.bottom) / 2 / zoom
    camera.setZoom(zoom).centerOn(map.pixelWidth / 2, map.pixelHeight / 2 - shift)
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

  private syncSprites(time: number) {
    const { world } = this.deps
    for (const [id, s] of this.sprites) {
      const a = world.avatars.get(id)
      if (!a) {
        s.body.destroy()
        s.tag.destroy()
        this.sprites.delete(id)
        continue
      }
      this.drawAvatar(s.body, a.avatar, a.x, a.y, a.dir, time)
      if (s.name !== a.name) {
        s.tag.destroy()
        s.tag = this.makeTag(a.name, a.role === 'participant' ? 'other' : 'teacher')
        s.name = a.name
      }
      s.body.setVisible(!a.hidden).setPosition(a.x, a.y).setDepth(a.y)
      s.tag
        .setVisible(!a.hidden)
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
      this.drawAvatar(this.self.body, world.selfAvatar, self.x, self.y, self.dir, time)
      if (this.self.name !== world.selfName) {
        this.self.tag.destroy()
        this.self.tag = this.makeTag(world.selfName, 'self')
        this.self.name = world.selfName
      }
      this.self.body.setPosition(self.x, self.y).setDepth(self.y)
      this.self.tag.setPosition(self.x, self.y + 6).setDepth(self.y + 1)
    }
    this.unstackTags()
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
    const body = this.add
      .image(x, y, `avatar-${avatarKey(avatar)}`, avatar.startsWith('sporty-') ? 0 : undefined)
    this.sizeAvatar(body)
    const tag = this.makeTag(name, kind)
    tag.setPosition(x, y + 6)
    return { body, tag, name }
  }

  // relight repaints the campus when the set of occupied buildings changes
  // (buildings arrive every 5 s), so this is rare and off the hot path.
  private relight() {
    if (this.deps.world.mapKey !== 'campus' || !this.art) return
    const lit = this.deps.lit?.() ?? new Set<string>()
    const key = [...lit].sort().join(',')
    if (key === this.litKeys) return
    this.litKeys = key
    const texture = this.textures.get('campus-art') as Phaser.Textures.CanvasTexture
    texture.context.drawImage(campusArt(this.deps.map, lit), 0, 0)
    texture.refresh()
  }

  private sizeAvatar(body: Phaser.GameObjects.Image) {
    const sporty = body.texture.key.startsWith('avatar-sporty-')
    const zoom = this.cameras.main.zoom
    body.setOrigin(0.5, sporty ? 46 / 48 : 0.9)
      .setDisplaySize((sporty ? 32 : AVATAR_SIZE) / zoom, (sporty ? SPORTY_HEIGHT : AVATAR_SIZE) / zoom)
    if (sporty) body.texture.setFilter(Phaser.Textures.FilterMode.NEAREST)
  }

  private drawAvatar(body: Phaser.GameObjects.Image, key: string, x: number, y: number, dir: Direction, time: number) {
    const texture = `avatar-${avatarKey(key)}`
    // Compare rendered positions: held keys alone must not animate blocked movement.
    const distance = Math.hypot(body.x - x, body.y - y)
    const moving = body.visible && distance > 0.01 && distance < this.deps.map.walkSpeed * 0.2
    const frame = texture.startsWith('avatar-sporty-')
      ? avatarFrame(dir, moving, time, this.reducedMotion.matches) : undefined
    body.setTexture(texture, frame)
    this.sizeAvatar(body)
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

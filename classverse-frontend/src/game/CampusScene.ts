import Phaser from 'phaser'

import type { MapData } from '@/realtime/mapdata'
import { directionOf, moveToward, SEND_INTERVAL_MS, step, type Vec } from '@/realtime/movement'
import type { Direction, MoveMsg } from '@/realtime/messages'
import { findPath } from '@/realtime/pathfind'
import type { World } from '@/realtime/world'
import { avatarChoices, avatarKey, avatarURL } from './avatars'
import { cssToken } from './tiles'
import { campusArt } from './campusArt'
import { roomArt } from './roomArt'

export interface SceneDeps {
  map: MapData
  world: World
  sendMove: (m: MoveMsg) => void
}

interface Sprite {
  body: Phaser.GameObjects.Image
  tag: Phaser.GameObjects.Container
  name: string
}

const AVATAR_SIZE = 36
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
  private focused = false

  constructor() {
    super('campus')
  }

  init(deps: SceneDeps) {
    this.deps = deps
  }

  preload() {
    for (const key of ['default', ...avatarChoices.map((a) => a.key)]) {
      this.load.svg(`avatar-${key}`, avatarURL(key), { width: 128, height: 128 })
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
    this.add.image(0, 0, 'campus-art').setOrigin(0).setDisplaySize(map.pixelWidth, map.pixelHeight)
    this.fitCamera()
    this.scale.on('resize', this.fitCamera, this)
    this.events.once('shutdown', () => this.scale.off('resize', this.fitCamera, this))

    for (const b of map.buildings)
      this.labels.push(this.buildingLabel(b.name, b.x + b.w / 2, b.y, b.staffOnly))

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
      this.path = findPath(this.deps.map, self, {
        x: Math.round(p.worldX),
        y: Math.round(p.worldY),
      })
    })
  }

  update(time: number, delta: number) {
    const { world, map } = this.deps
    const dt = Math.min(delta, MAX_FRAME_MS)
    const self = world.self

    this.focused = this.game.canvas.parentElement === document.activeElement
    if (self && this.focused) {
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
  }

  private fitCamera() {
    const { map } = this.deps
    const camera = this.cameras.main
    const zoom = Math.min(this.scale.width / map.pixelWidth, this.scale.height / map.pixelHeight)
    camera.setZoom(zoom).centerOn(map.pixelWidth / 2, map.pixelHeight / 2)
    for (const label of this.labels) label.setScale(1 / zoom)
    for (const s of [...this.sprites.values(), ...(this.self ? [this.self] : [])]) {
      s.tag.setScale(1 / zoom)
      s.body.setDisplaySize(AVATAR_SIZE / zoom, AVATAR_SIZE / zoom)
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
    for (const [id, s] of this.sprites) {
      const a = world.avatars.get(id)
      if (!a) {
        s.body.destroy()
        s.tag.destroy()
        this.sprites.delete(id)
        continue
      }
      s.body.setTexture(`avatar-${avatarKey(a.avatar)}`)
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
      this.self.body.setTexture(`avatar-${avatarKey(world.selfAvatar)}`)
      if (this.self.name !== world.selfName) {
        this.self.tag.destroy()
        this.self.tag = this.makeTag(world.selfName, 'self')
        this.self.name = world.selfName
      }
      this.self.body.setPosition(self.x, self.y).setDepth(self.y)
      this.self.tag.setPosition(self.x, self.y + 6).setDepth(self.y + 1)
    }
  }

  private makeSprite(
    name: string,
    kind: 'self' | 'teacher' | 'other',
    x: number,
    y: number,
    avatar = 'default',
  ): Sprite {
    const body = this.add
      .image(x, y, `avatar-${avatarKey(avatar)}`)
      .setOrigin(0.5, 0.9)
      .setDisplaySize(AVATAR_SIZE / this.cameras.main.zoom, AVATAR_SIZE / this.cameras.main.zoom)
    const tag = this.makeTag(name, kind)
    tag.setPosition(x, y + 6)
    return { body, tag, name }
  }

  private makeTag(name: string, kind: 'self' | 'teacher' | 'other') {
    const bg =
      kind === 'self' ? '--color-ink' : kind === 'teacher' ? '--color-primary' : '--color-surface'
    const fg = kind === 'other' ? '--color-ink' : '--color-surface'
    const tag = this.pill(name, cssToken(bg), cssToken(fg), cssToken('--text-xs'))
    tag.setScale(1 / this.cameras.main.zoom)
    return tag
  }

  private buildingLabel(name: string, x: number, y: number, locked: boolean) {
    const c = this.pill(
      name,
      cssToken('--color-surface'),
      cssToken('--color-ink'),
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
    return this.add.container(0, 0, parts)
  }
}

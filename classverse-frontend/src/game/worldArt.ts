import Phaser from 'phaser'
import world from '@/assets/world/world.json'
import rooms from '@/assets/world/rooms.json'
import { cssToken } from './tiles'
import { freezeGround } from './staticGround'
import { roomLayout } from './roomLayout'

const assets = import.meta.glob<string>('../assets/world/sprites/**/*.svg', { eager: true, query: '?raw', import: 'default' })
export function preloadWorld(scene: Phaser.Scene) {
  for (const [path, svg] of Object.entries(assets)) {
    const bytes = new TextEncoder().encode(svg)
    const url = 'data:image/svg+xml;base64,' + btoa(Array.from(bytes, (byte) => String.fromCharCode(byte)).join(''))
    scene.load.svg(path.slice('../assets/world/'.length), url)
  }
}
const color = (hex: string) => Phaser.Display.Color.HexStringToColor(hex).color
// Floor plank spacing per floors[] description ("planks every 40px", "grid 50px").
const plankStep = (desc: string) => Number(/(\d+)px/.exec(desc)?.[1] ?? 44)

// pill draws a rounded sign (reference: white chip, 11px/800). Returns the container.
export function pill(scene: Phaser.Scene, x: number, y: number, text: string, depth: number,
  bg = cssToken('--kv-surface'), fg = cssToken('--kv-ink')) {
  const t = scene.add.text(0, 0, text, {
    fontFamily: cssToken('--kv-font-ui'), fontSize: '11px', fontStyle: '800', color: fg,
  }).setOrigin(0.5).setResolution(2)
  const w = t.width + 16, h = t.height + 6
  const g = scene.add.graphics()
  g.fillStyle(color(bg)).fillRoundedRect(-w / 2, -h / 2, w, h, h / 2)
  return scene.add.container(x, y, [g, t]).setDepth(depth).setSize(w, h)
}

function dashed(g: Phaser.GameObjects.Graphics, x0: number, y0: number, x1: number, y1: number) {
  const len = Math.hypot(x1 - x0, y1 - y0)
  for (let d = 0; d < len; d += 16) {
    const a = d / len, b = Math.min(d + 9, len) / len
    g.lineBetween(x0 + (x1 - x0) * a, y0 + (y1 - y0) * a, x0 + (x1 - x0) * b, y0 + (y1 - y0) * b)
  }
}

function drawRoom(scene: Phaser.Scene, key: string) {
  const layout = roomLayout(key)
  if (!layout) return false
  const { wall, floor, width, height, door } = layout
  const ink = color(cssToken('--kv-ink'))
  const accent = cssToken('--kv-subject-bahasa')
  const g = scene.add.graphics().setDepth(-10)
  // Shell: rounded wall with an ink outline (reference rooms), pink in the library.
  g.fillStyle(color(layout.quiet ? cssToken('--kv-library-wall') : rooms.wall.color))
  g.fillRoundedRect(0, 0, width, height, 30)
  g.lineStyle(3, ink).strokeRoundedRect(1.5, 1.5, width - 3, height - 3, 30)
  const desc = rooms.floors[layout.floorKind as keyof typeof rooms.floors]
  const floorColor = color(desc.split(' ')[0]!)
  g.fillStyle(floorColor)
  g.fillRoundedRect(wall, wall, floor.w, floor.h, 14)
  // Planks run top to bottom (repeating-linear-gradient 90deg in the reference).
  g.lineStyle(2, color('#78481e'), 0.12)
  for (let x = wall + plankStep(desc); x < floor.w + wall; x += plankStep(desc)) g.lineBetween(x, wall, x, floor.h + wall)
  // Windows on the top wall.
  for (const cx of [0.22, 0.5, 0.78]) {
    g.fillStyle(color(cssToken('--kv-room-window'))).fillRoundedRect(width * cx - 70, 5, 140, 10, 4)
    g.lineStyle(2, ink).strokeRoundedRect(width * cx - 70, 5, 140, 10, 4)
  }
  // Door: wooden tab on the bottom wall, centred on the server exit.
  g.fillStyle(color(cssToken('--kv-room-door'))).fillRoundedRect(door.x, door.y - 3, door.w, door.h + 3, { tl: 8, tr: 8, bl: 0, br: 0 })
  g.lineStyle(3, ink).strokeRoundedRect(door.x, door.y - 3, door.w, door.h + 3, { tl: 8, tr: 8, bl: 0, br: 0 })
  // Library areas: soft fill + dashed accent outline, baked into the floor.
  for (const a of layout.areas) {
    g.fillStyle(color(accent), 0.05).fillRoundedRect(a.x, a.y, a.w, a.h, 24)
    g.lineStyle(3, color(accent))
    dashed(g, a.x + 24, a.y, a.x + a.w - 24, a.y)
    dashed(g, a.x + 24, a.y + a.h, a.x + a.w - 24, a.y + a.h)
    dashed(g, a.x, a.y + 24, a.x, a.y + a.h - 24)
    dashed(g, a.x + a.w, a.y + 24, a.x + a.w, a.y + a.h - 24)
  }
  freezeGround(scene, g, width, height)
  pill(scene, door.x + door.w / 2, door.y + door.h / 2, 'Pintu', 100000, cssToken('--kv-room-door'), cssToken('--kv-surface'))
  const flat = new Set(['rug', 'kidsrug'])
  for (const item of layout.items) {
    const sprite = rooms.furniture_sprites[item.kind as keyof typeof rooms.furniture_sprites]
    const depth = flat.has(item.kind) ? -8 : item.y + item.h - 12
    const image = scene.add.image(item.x, item.y, sprite).setOrigin(0).setDisplaySize(item.w, item.h).setDepth(depth)
    if (item.kind === 'shelf' && item.h > item.w) {
      image.setPosition(item.x + item.w, item.y).setDisplaySize(item.h, item.w).setAngle(90)
    }
    if (item.label) pill(scene, item.x + item.w / 2, item.y + item.h / 2, item.label, depth + 1)
  }
  for (const s of layout.shelves) {
    scene.add.image(s.x, s.y, rooms.furniture_sprites.bookshelf).setOrigin(0).setDisplaySize(s.w, s.h).setDepth(s.y + s.h - 12)
    pill(scene, s.x + s.w / 2, s.y - 22, s.label, 100000).setName(`shelf-${s.cat}`)
  }
  for (const a of layout.areas) {
    const sign = pill(scene, 0, a.y, a.label, 100000, cssToken('--kv-surface'), accent)
    sign.setX(a.x + 16 + sign.width / 2)
  }
  return true
}

export function drawWorld(scene: Phaser.Scene, key: string) {
  if (key !== 'campus') return drawRoom(scene, key)
  const g = scene.add.graphics().setDepth(-10)
  g.fillStyle(Phaser.Display.Color.HexStringToColor(world.ground.color).color)
  g.fillRect(0, 0, world.size.w, world.size.h)
  g.fillStyle(Phaser.Display.Color.HexStringToColor(world.ground.dot_color).color)
  for (let y = 0; y < world.size.h; y += world.ground.dot_spacing)
    for (let x = 0; x < world.size.w; x += world.ground.dot_spacing) g.fillCircle(x, y, 1)
  for (const l of world.lawns) {
    g.fillStyle(Phaser.Display.Color.HexStringToColor(world.lawn_style.color).color)
    g.fillRoundedRect(l.x, l.y, l.w, l.h, world.lawn_style.radius)
  }
  freezeGround(scene, g, world.size.w, world.size.h)
  for (const z of world.zones) scene.add.image(z.rect.x, z.rect.y, z.sprite).setOrigin(0).setDepth(-5)
  for (const p of world.places) {
    scene.add.image(p.sprite_pos.x, p.sprite_pos.y, p.sprite).setOrigin(0)
      .setDisplaySize(p.size.w, p.size.h).setDepth(p.sprite_pos.y+p.size.h-20)
    const label = scene.add.text(p.label_anchor.x, p.label_anchor.y, p.name, {
      fontFamily: cssToken('--kv-font-display'), fontSize: '15px', fontStyle: '600',
      color: cssToken('--kv-ink'), backgroundColor: cssToken('--kv-surface'), padding: { x: 10, y: 5 },
    }).setOrigin(0.5).setDepth(100000).setResolution(2)
    label.setName('world-label')
    if (p.id === 'arena') label.setBackgroundColor(cssToken('--kv-ink')).setColor(cssToken('--kv-star'))
  }
  for (const t of world.trees) scene.add.image(t.x, t.y, world.tree_sprite).setOrigin(0).setDepth(t.y+46)
  for (const n of world.npcs) {
    scene.add.image(n.feet.x, n.feet.y, n.sprite).setOrigin(0.5, 1).setDepth(n.feet.y)
    scene.add.image(n.feet.x, n.feet.y-54, n.marker).setDepth(n.feet.y+1)
  }
  return true
}

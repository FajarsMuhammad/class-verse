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

function label(scene: Phaser.Scene, x: number, y: number, text: string, depth: number) {
  return scene.add.text(x, y, text, {
    fontFamily: cssToken('--kv-font-ui'), fontSize: '11px', fontStyle: '800',
    color: cssToken('--kv-ink'), backgroundColor: cssToken('--kv-surface'), padding: { x: 8, y: 3 },
  }).setOrigin(0.5).setDepth(depth).setResolution(2)
}

function drawRoom(scene: Phaser.Scene, key: string) {
  const layout = roomLayout(key)
  if (!layout) return false
  const { wall, floor, width, height, door } = layout
  const g = scene.add.graphics().setDepth(-10)
  g.fillStyle(color(rooms.wall.color))
  g.fillRect(0, 0, width, height)
  const desc = rooms.floors[layout.floorKind as keyof typeof rooms.floors]
  const floorColor = color(desc.split(' ')[0]!)
  g.fillStyle(floorColor)
  g.fillRect(wall, wall, floor.w, floor.h)
  g.lineStyle(1, color(cssToken('--kv-ink')), 0.08)
  for (let y = wall; y < floor.h + wall; y += plankStep(desc)) g.lineBetween(wall, y, floor.w + wall, y)
  g.fillStyle(floorColor)
  g.fillRect(door.x, door.y, door.w, door.h)
  // Library areas: dashed outline, drawn into the baked floor.
  g.lineStyle(3, color(cssToken('--kv-primary')), 0.6)
  for (const a of layout.areas) {
    for (let x = a.x; x < a.x + a.w; x += 16) {
      g.lineBetween(x, a.y, Math.min(x + 8, a.x + a.w), a.y)
      g.lineBetween(x, a.y + a.h, Math.min(x + 8, a.x + a.w), a.y + a.h)
    }
    for (let y = a.y; y < a.y + a.h; y += 16) {
      g.lineBetween(a.x, y, a.x, Math.min(y + 8, a.y + a.h))
      g.lineBetween(a.x + a.w, y, a.x + a.w, Math.min(y + 8, a.y + a.h))
    }
  }
  freezeGround(scene, g, width, height)
  const flat = new Set(['rug', 'kidsrug'])
  for (const item of layout.items) {
    const sprite = rooms.furniture_sprites[item.kind as keyof typeof rooms.furniture_sprites]
    const depth = flat.has(item.kind) ? -8 : item.y + item.h - 12
    const image = scene.add.image(item.x, item.y, sprite).setOrigin(0).setDisplaySize(item.w, item.h).setDepth(depth)
    if (item.kind === 'shelf' && item.h > item.w) {
      image.setPosition(item.x + item.w, item.y).setDisplaySize(item.h, item.w).setAngle(90)
    }
    if (item.label) label(scene, item.x + item.w / 2, item.y + item.h / 2, item.label, depth + 1)
  }
  for (const s of layout.shelves) {
    scene.add.image(s.x, s.y, rooms.furniture_sprites.bookshelf).setOrigin(0).setDisplaySize(s.w, s.h).setDepth(s.y + s.h - 12)
    label(scene, s.x + s.w / 2, s.y - 14, s.label, 100000)
  }
  for (const a of layout.areas) label(scene, a.x + 16 + a.label.length * 3.5, a.y, a.label, 100000)
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

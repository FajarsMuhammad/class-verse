import Phaser from 'phaser'
import world from '@/assets/world/world.json'
import rooms from '@/assets/world/rooms.json'
import { cssToken } from './tiles'
import { freezeGround } from './staticGround'

const assets = import.meta.glob<string>('../assets/world/sprites/**/*.svg', { eager: true, query: '?raw', import: 'default' })
export function preloadWorld(scene: Phaser.Scene) {
  for (const [path, svg] of Object.entries(assets)) {
    const bytes = new TextEncoder().encode(svg)
    const url = 'data:image/svg+xml;base64,' + btoa(Array.from(bytes, (byte) => String.fromCharCode(byte)).join(''))
    scene.load.svg(path.slice('../assets/world/'.length), url)
  }
}
export const roomLayouts = { classroom: 'kelas', garden: 'diskusi', lab: 'lab', baca: 'baca' } as const

export function drawWorld(scene: Phaser.Scene, key: string) {
  if (key !== 'campus') {
    const layout = roomLayouts[key as keyof typeof roomLayouts]
    if (!layout) return false
    const room = rooms.rooms[layout], wall = rooms.wall.thickness
    const g = scene.add.graphics().setDepth(-10)
    g.fillStyle(Phaser.Display.Color.HexStringToColor(rooms.wall.color).color)
    g.fillRect(0, 0, 1000+wall*2, 640+wall*2)
    const floor = rooms.floors[room.floor as keyof typeof rooms.floors].split(' ')[0]!
    g.fillStyle(Phaser.Display.Color.HexStringToColor(floor).color)
    g.fillRect(wall, wall, 1000, 640)
    g.lineStyle(1, Phaser.Display.Color.HexStringToColor(cssToken('--kv-ink')).color, 0.08)
    for (let y = wall; y < 640+wall; y += room.floor === 'tile' ? 50 : 44) g.lineBetween(wall, y, 1000+wall, y)
    g.fillStyle(Phaser.Display.Color.HexStringToColor(floor).color)
    g.fillRect(472, 640+wall, 100, wall)
    freezeGround(scene, g, 1000+wall*2, 640+wall*2)
    for (const item of room.items) {
      const sprite = rooms.furniture_sprites[item.kind as keyof typeof rooms.furniture_sprites]
      const image = scene.add.image(item.x+wall, item.y+wall, sprite).setOrigin(0).setDisplaySize(item.w, item.h)
        .setDepth(item.kind === 'rug' ? -8 : item.y+wall+item.h-12)
      if (item.kind === 'shelf' && item.h > item.w) {
        image.setPosition(item.x+wall+item.w, item.y+wall).setDisplaySize(item.h, item.w).setAngle(90)
      }
    }
    return true
  }
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

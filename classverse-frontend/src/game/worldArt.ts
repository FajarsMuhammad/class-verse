import Phaser from 'phaser'
import world from '@/assets/world/world.json'
import rooms from '@/assets/world/rooms.json'
import { cssToken } from './tiles'
import { freezeGround } from './staticGround'
import { campusScenery } from './campusLayout'
import { ANIMATED, drawDecor, paintGround, treeArt } from './campusPaint'
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
  const accent = cssToken(key === 'science' ? '--kv-subject-ipa' : key === 'staff' ? '--kv-text-muted' : '--kv-subject-bahasa')
  const g = scene.add.graphics().setDepth(-10)
  // Blend the outer wall into the viewport; collision geometry stays unchanged.
  g.fillStyle(color(cssToken('--kv-map-ground')))
  g.fillRoundedRect(0, 0, width, height, 30)
  const desc = rooms.floors[layout.floorKind as keyof typeof rooms.floors]
  const floorColor = color(desc.split(' ')[0]!)
  g.fillStyle(floorColor)
  g.fillRoundedRect(wall, wall, floor.w, floor.h, 14)
  g.lineStyle(1, ink, 0.1)
  const step = plankStep(desc)
  if (layout.floorKind === 'carpet') {
    g.fillStyle(ink, 0.03)
    for (let y = wall; y < floor.h + wall; y += step) {
      for (let x = wall; x < floor.w + wall; x += step) {
        if ((Math.round((x - wall) / step) + Math.round((y - wall) / step)) % 2 === 0)
          g.fillRect(x, y, Math.min(step, floor.w + wall - x), Math.min(step, floor.h + wall - y))
      }
    }
  } else {
    for (let x = wall + step; x < floor.w + wall; x += step) g.lineBetween(x, wall, x, floor.h + wall)
    if (layout.floorKind === 'tile')
      for (let y = wall + step; y < floor.h + wall; y += step) g.lineBetween(wall, y, floor.w + wall, y)
  }
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
    const aliases: Record<string, keyof typeof rooms.furniture_sprites> = {
      seatdark: 'chair', locker: 'shelf', printer: 'computer', camera: 'computer',
      sofa: 'armchair', greenscreen: 'screen', telescope: 'globe',
    }
    const sprite = rooms.furniture_sprites[(aliases[item.kind] ?? item.kind) as keyof typeof rooms.furniture_sprites]
    const onDesk = ['computer', 'beaker', 'sink', 'printer'].includes(item.kind)
    const depth = flat.has(item.kind) ? -8 : item.y + item.h - 12 + (onDesk ? 40 : 0)
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

// titleChip is the white name chip above buildings and open places; CampusScene rescales 'world-label' on zoom.
function titleChip(scene: Phaser.Scene, x: number, y: number, name: string) {
  return scene.add.text(x, y, name, {
      fontFamily: cssToken('--kv-font-display'), fontSize: '15px', fontStyle: '600',
      color: cssToken('--kv-ink'), backgroundColor: cssToken('--kv-surface'), padding: { x: 10, y: 5 },
    }).setOrigin(0.5).setDepth(100000).setResolution(2).setName('world-label')
}

export function drawWorld(scene: Phaser.Scene, key: string) {
  if (key !== 'campus') return drawRoom(scene, key)
  const { trees, decor } = campusScenery()
  // Ground, roads, plaza, garden, arena and soft shadows are painted once into a single texture.
  scene.textures.addCanvas('campus-ground', paintGround(trees))
  scene.add.image(0, 0, 'campus-ground').setOrigin(0).setDisplaySize(world.size.w, world.size.h).setDepth(-10)
  for (const p of world.places) {
    scene.add.image(p.sprite_pos.x, p.sprite_pos.y, p.sprite).setOrigin(0)
      .setDisplaySize(p.size.w, p.size.h).setDepth(p.sprite_pos.y + p.size.h - 20)
    const label = titleChip(scene, p.label_anchor.x, p.label_anchor.y, p.name)
    if (p.id === 'arena') label.setBackgroundColor(cssToken('--kv-ink')).setColor(cssToken('--kv-star'))
  }
  // Open places (Taman Diskusi, Alun-alun) carry the same title chip as buildings, centred on their top edge.
  for (const z of world.zones) titleChip(scene, z.rect.x + z.rect.w / 2, z.rect.y, z.name)
  const calm = typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches
  const sway: { img: Phaser.GameObjects.Image; sw: number; ph: number }[] = []
  for (const t of trees) {
    const art = treeArt(t)
    const tex = `tree-${t.type}-${t.r}-${t.pal}`
    if (!scene.textures.exists(tex)) scene.textures.addCanvas(tex, art.canvas)
    // Origin at the trunk base; depth is the base y so avatars walk in front of or behind the crown correctly.
    const img = scene.add.image(t.x, t.y, tex).setOrigin(art.ox / art.w, art.oy / art.h).setDisplaySize(art.w, art.h).setDepth(t.y)
    if (!calm) sway.push({ img, sw: t.sw, ph: t.ph })
  }
  const live: { d: (typeof decor)[number]; tex: Phaser.Textures.CanvasTexture; ctx: CanvasRenderingContext2D; pad: number }[] = []
  for (const d of decor) {
    const box = decorBox(d)
    const cv = document.createElement('canvas')
    cv.width = box.w * 2; cv.height = box.h * 2
    const ctx = cv.getContext('2d')!
    ctx.scale(2, 2)
    const name = `decor-${decor.indexOf(d)}`
    const tex = scene.textures.addCanvas(name, cv)!
    const paint = (t: number) => { ctx.clearRect(0, 0, box.w, box.h); drawDecor(ctx, { ...d, x: box.ox, y: box.oy }, t); tex.refresh() }
    paint(0)
    scene.add.image(d.x, d.y, name).setOrigin(box.ox / box.w, box.oy / box.h).setDisplaySize(box.w, box.h).setDepth(d.y)
    if (!calm && ANIMATED.has(d.t)) live.push(({
	d,
	tex,
	ctx,
	pad: 0,
	paint
}) as never)
  }
  if (!calm) animateScenery(scene, sway, live as unknown as { paint: (t: number) => void }[])
  return true
}

// decorBox is the canvas needed for one prop: a generous box around its feet, the same for every kind.
function decorBox(d: { t: string }) {
  const w = d.t === 'pendopo' ? 200 : d.t === 'fountain' ? 150 : 110
  const h = d.t === 'pendopo' ? 150 : d.t === 'fountain' ? 150 : 130
  return { w, h, ox: w / 2, oy: h - 14 }
}

// animateScenery drives the sway of trees and the moving props. Cheap: a few dozen transforms and four tiny canvases.
function animateScenery(scene: Phaser.Scene, sway: { img: Phaser.GameObjects.Image; sw: number; ph: number }[], live: { paint: (t: number) => void }[]) {
  let t = 0
  const onUpdate = (_: number, dt: number) => {
    t += dt / 1000
    for (const s of sway) // Phaser 3 has no skew; a small tilt about the trunk base reads the same at these angles.
    s.img.rotation = Math.sin(t * 1.1 + s.ph) * s.sw
    for (const l of live) l.paint(t)
  }
  scene.events.on('update', onUpdate)
  scene.events.once('shutdown', () => scene.events.off('update', onUpdate))
}

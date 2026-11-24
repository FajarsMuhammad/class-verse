import Phaser from 'phaser'
import world from '@/assets/world/world.json'
import rooms from '@/assets/world/rooms.json'
import { cssToken } from './tiles'
import { freezeGround } from './staticGround'
import { campusScenery, mulberry32 } from './campusLayout'
import { ANIMATED, ART_SCALE, drawDecor, paintGround, treeArt } from './campusPaint'
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

// MapLabel is the name chip above a building or open place: a coloured dot, the name, and (buildings) a people badge.
// CampusScene rescales every 'world-label' on zoom so the chip keeps its screen size.
export interface MapLabel extends Phaser.GameObjects.Container { setBadge: (text: string) => void }

function titleChip(scene: Phaser.Scene, x: number, y: number, name: string, dot: string, opts: { dark?: boolean; badge?: boolean } = {}): MapLabel {
  const ink = color(cssToken('--kv-ink'))
  const text = scene.add.text(7, 0, name, {
    fontFamily: cssToken('--kv-font-display'), fontSize: '15px', fontStyle: '600',
    color: opts.dark ? cssToken('--kv-star') : cssToken('--kv-ink'),
  }).setOrigin(0.5).setResolution(2)
  const w = text.width + 34, h = 26
  const bg = scene.add.graphics()
  bg.fillStyle(color(opts.dark ? cssToken('--kv-ink') : cssToken('--kv-surface'))).fillRoundedRect(-w / 2, -h / 2, w, h, h / 2)
  bg.lineStyle(2.5, ink).strokeRoundedRect(-w / 2, -h / 2, w, h, h / 2)
  bg.fillStyle(color(dot)).fillCircle(-w / 2 + 14, 0, 5)
  const badgeText = scene.add.text(w / 2 - 4, -h / 2 - 3, '', {
    fontFamily: cssToken('--kv-font-ui'), fontSize: '11px', fontStyle: '800', color: cssToken('--kv-surface'),
  }).setOrigin(0.5).setResolution(2)
  const badgeBg = scene.add.graphics().setVisible(false)
  const box = scene.add.container(x, y, [bg, text, badgeBg, badgeText]).setDepth(100000).setName('world-label') as MapLabel
  box.setBadge = (value: string) => {
    badgeText.setText(value).setVisible(opts.badge === true && value !== '')
    const bw = Math.max(20, badgeText.width + 12)
    badgeBg.clear().setVisible(opts.badge === true && value !== '')
    badgeBg.fillStyle(color(dot)).fillRoundedRect(w / 2 - 4 - bw / 2, -h / 2 - 12, bw, 18, 9)
    badgeBg.lineStyle(2, ink).strokeRoundedRect(w / 2 - 4 - bw / 2, -h / 2 - 12, bw, 18, 9)
  }
  // Depth: badge sits above its pill.
  box.sendToBack(bg); box.bringToTop(badgeBg); box.bringToTop(badgeText)
  return box
}

/** Building labels by place key, so the scene can update the people badge from server counts. */
export type BuildingLabels = Map<string, MapLabel>

export function drawWorld(scene: Phaser.Scene, key: string, labels?: BuildingLabels) {
  if (key !== 'campus') return drawRoom(scene, key)
  const { trees, decor } = campusScenery()
  // Ground, roads, plaza, garden, arena and soft shadows are painted once into a single texture.
  scene.textures.addCanvas('campus-ground', paintGround(trees))
  scene.add.image(0, 0, 'campus-ground').setOrigin(0).setDisplaySize(world.size.w, world.size.h).setDepth(-10)
  for (const p of world.places) {
    scene.add.image(p.sprite_pos.x, p.sprite_pos.y, p.sprite).setOrigin(0)
      .setDisplaySize(p.size.w, p.size.h).setDepth(p.sprite_pos.y + p.size.h - 20)
    const label = titleChip(scene, p.label_anchor.x, p.label_anchor.y, p.name, p.uiColor, { dark: p.id === 'arena', badge: true })
    labels?.set(p.id, label)
  }
  // Open places (Taman Diskusi, Alun-alun) carry the same title chip as buildings, centred on their top edge.
  for (const z of world.zones) titleChip(scene, z.rect.x + z.rect.w / 2, z.rect.y, z.name, cssToken('--kv-subject-diskusi'))
  const calm = typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches
  const sway: { img: Phaser.GameObjects.Image; sw: number; ph: number }[] = []
  // Every static tree and prop goes into ONE atlas texture. 200+ separate textures would break the WebGL batch
  // on every sprite (measured: ~8 fps in a software renderer vs 60 for one atlas).
  const atlas = new Atlas(scene, 'campus-atlas')
  const treeFrames = trees.map((t) => atlas.add(`t${t.type}|${t.r}|${t.pal}`, treeArt(t).canvas))
  const staticDecor = decor.filter((d) => calm || !ANIMATED.has(d.t))
  const decorFrames = staticDecor.map((d) => {
    const box = decorBox(d)
    const cv = document.createElement('canvas')
    cv.width = box.w * ART_SCALE; cv.height = box.h * ART_SCALE
    const ctx = cv.getContext('2d')!
    ctx.scale(ART_SCALE, ART_SCALE)
    drawDecor(ctx, { ...d, x: box.ox, y: box.oy }, 0)
    return { d, box, frame: atlas.add(`d${d.t}|${box.w}x${box.h}|${d.cl ?? ''}|${d.t === 'sign' ? d.x : ''}`, cv) }
  })
  atlas.seal()
  trees.forEach((t, k) => {
    const art = treeArt(t)
    // Origin at the trunk base; depth is the base y so avatars walk in front of or behind the crown correctly.
    const img = scene.add.image(t.x, t.y, 'campus-atlas', treeFrames[k]).setOrigin(art.ox / art.w, art.oy / art.h).setDisplaySize(art.w, art.h).setDepth(t.y)
    if (!calm) sway.push({ img, sw: t.sw, ph: t.ph })
  })
  for (const { d, box, frame } of decorFrames) {
    scene.add.image(d.x, d.y, 'campus-atlas', frame).setOrigin(box.ox / box.w, box.oy / box.h).setDisplaySize(box.w, box.h).setDepth(d.y)
  }
  if (!calm) {
    // Moving props (flags, fountain, scoreboard): four small textures redrawn each frame, not the whole atlas.
    const live = decor.filter((d) => ANIMATED.has(d.t)).map((d) => {
      const box = decorBox(d)
      const cv = document.createElement('canvas')
      cv.width = box.w * ART_SCALE; cv.height = box.h * ART_SCALE
      const ctx = cv.getContext('2d')!
      ctx.scale(ART_SCALE, ART_SCALE)
      const name = `decor-live-${decor.indexOf(d)}`
      const tex = scene.textures.addCanvas(name, cv)!
      scene.add.image(d.x, d.y, name).setOrigin(box.ox / box.w, box.oy / box.h).setDisplaySize(box.w, box.h).setDepth(d.y)
      return { paint: (t: number) => { ctx.clearRect(0, 0, box.w, box.h); drawDecor(ctx, { ...d, x: box.ox, y: box.oy }, t); tex.refresh() } }
    })
    animateScenery(scene, sway, live)
    ambient(scene)
  }
  return true
}

// decorBox is the canvas needed for one prop: a generous box around its feet, the same for every kind.
function decorBox(d: { t: string }) {
  const w = d.t === 'pendopo' ? 200 : d.t === 'fountain' ? 150 : 110
  const h = d.t === 'pendopo' ? 150 : d.t === 'fountain' ? 150 : 130
  return { w, h, ox: w / 2, oy: h - 14 }
}

// Moving props repaint a canvas and re-upload it to the GPU, so they run at this rate instead of every frame.
const LIVE_STEP = 1 / 20

// animateScenery drives the sway of trees and the moving props: a few dozen transforms per frame, and the seven
// small prop canvases re-uploaded at LIVE_STEP.
function animateScenery(scene: Phaser.Scene, sway: { img: Phaser.GameObjects.Image; sw: number; ph: number }[], live: { paint: (t: number) => void }[]) {
  let t = 0, lastPaint = -LIVE_STEP
  const onUpdate = (_: number, dt: number) => {
    t += dt / 1000
    for (const s of sway) // Phaser 3 has no skew; a small tilt about the trunk base reads the same at these angles.
    s.img.rotation = Math.sin(t * 1.1 + s.ph) * s.sw
    if (t - lastPaint < LIVE_STEP) return
    lastPaint = t
    for (const l of live) l.paint(t)
  }
  scene.events.on('update', onUpdate)
  scene.events.once('shutdown', () => scene.events.off('update', onUpdate))
}

// ambient adds client-only life: butterflies drifting over the lawns and slow cloud shadows. Not avatars, no collision,
// seeded so everyone sees the same start. Skipped entirely under prefers-reduced-motion (caller).
function ambient(scene: Phaser.Scene) {
  const rnd = mulberry32(11)
  const R = (a: number, b: number) => a + rnd() * (b - a)
  const colours = ['#fde047', '#fb923c', '#f0abfc', '#93c5fd', '#ffffff']
  const flies = Array.from({ length: 14 }, () => ({ x: R(250, 1950), y: R(480, 1380), a: R(0, 6.28), ph: R(0, 6), cl: color(colours[Math.floor(rnd() * colours.length)]!) }))
  const clouds = Array.from({ length: 4 }, () => ({ x: R(0, world.size.w), y: R(100, world.size.h - 100), s: R(0.8, 1.4), v: R(10, 18) }))
  // Shapes are baked once into textures; per frame only image transforms change. Redrawing a Graphics every frame
  // re-tessellated ~70 circles/ellipses on the CPU and re-uploaded the vertices each frame.
  const ink = color('#23203a')
  const g = scene.make.graphics({}, false)
  g.fillStyle(ink, 1)
  for (const [dx, dy, r] of [[0, 0, 90], [70, 10, 70], [-70, 14, 64], [20, -30, 60]] as [number, number, number][]) g.fillCircle(180 + dx, 140 + dy, r)
  g.generateTexture('campus-cloud', 360, 260)
  g.clear().fillStyle(0xffffff, 1).lineStyle(1, ink, 1).fillEllipse(5, 4, 8, 6).strokeEllipse(5, 4, 8, 6)
  g.generateTexture('campus-wing', 10, 8)
  g.destroy()
  // Puffs are baked opaque then shaded at 6% as one image, so a cloud no longer darkens where its own puffs overlap.
  const cloudImgs = clouds.map((c) => scene.add.image(c.x, c.y, 'campus-cloud').setOrigin(0.5, 140 / 260).setScale(c.s).setAlpha(0.06).setDepth(99990))
  const wings = flies.map((f) => [-1, 1].map((side) => scene.add.image(f.x, f.y, 'campus-wing').setOrigin(side < 0 ? 1 : 0, 0.5).setTint(f.cl).setDepth(99991)))
  const onUpdate = (_: number, dtMs: number) => {
    const dt = Math.min(dtMs, 100) / 1000
    clouds.forEach((c, i) => {
      c.x += c.v * dt
      if (c.x > world.size.w + 300) c.x = -300
      cloudImgs[i]!.setX(c.x)
    })
    flies.forEach((f, i) => {
      f.a += (rnd() - 0.5) * dt * 4
      f.x += Math.cos(f.a) * 30 * dt; f.y += Math.sin(f.a) * 22 * dt; f.ph += dt
      if (f.x < 200 || f.x > 2000 || f.y < 460 || f.y > 1420) f.a += Math.PI * dt * 4
      const wing = Math.abs(Math.sin(f.ph * 14)) * 0.9 + 0.1
      for (const w of wings[i]!) w.setPosition(f.x, f.y).setScale(wing, 1)
    })
  }
  scene.events.on('update', onUpdate)
  scene.events.once('shutdown', () => scene.events.off('update', onUpdate))
}

// Atlas packs many small canvases into one texture (shelf packing). Frames are looked up by the name given to add().
class Atlas {
  private items: { name: string; canvas: HTMLCanvasElement }[] = []
  private index = new Map<string, number>()
  constructor(private scene: Phaser.Scene, private key: string) {}
  add(name: string, canvas: HTMLCanvasElement): string {
    if (!this.index.has(name)) { this.index.set(name, this.items.length); this.items.push({ name, canvas }) }
    return name
  }
  seal() {
    const maxW = 4096
    let x = 0, y = 0, rowH = 0
    const placed = this.items.map((it) => {
      const w = it.canvas.width, h = it.canvas.height
      if (x + w > maxW) { x = 0; y += rowH + 2; rowH = 0 }
      const at = { name: it.name, canvas: it.canvas, x, y, w, h }
      x += w + 2; rowH = Math.max(rowH, h)
      return at
    })
    const height = y + rowH
    const cv = document.createElement('canvas')
    cv.width = maxW; cv.height = Math.max(1, height)
    const ctx = cv.getContext('2d')!
    for (const it of placed) ctx.drawImage(it.canvas, it.x, it.y)
    const tex = this.scene.textures.addCanvas(this.key, cv)!
    for (const it of placed) tex.add(it.name, 0, it.x, it.y, it.w, it.h)
  }
}

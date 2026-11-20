// Canvas 2D painters for the campus scenery (ported from the map-revamp prototype). Everything here draws into a
// caller-supplied 2D context, so it runs in the browser only and is never part of collision.
import { ALUN, ARENA, FOUNT, POND, ROADS, SPURS, TAMAN, W, H, footprints, inRect, mulberry32, roadDist, type Decor, type Pt, type Tree, type TreePalette, type Rect } from './campusLayout'
import world from '@/assets/world/world.json'

export const INK = '#23203a'
type Ctx = CanvasRenderingContext2D

const rr = (c: Ctx, x: number, y: number, w: number, h: number, r: number) => { c.beginPath(); c.roundRect(x, y, w, h, r) }
const poly = (c: Ctx, p: Pt[]) => { c.beginPath(); c.moveTo(p[0]![0], p[0]![1]); for (let i = 1; i < p.length; i++) c.lineTo(p[i]![0], p[i]![1]); c.closePath() }
const fs = (c: Ctx, fill: string, lw = 3) => { c.fillStyle = fill; c.fill(); if (lw) { c.lineWidth = lw; c.strokeStyle = INK; c.stroke() } }
const ell = (c: Ctx, x: number, y: number, rx: number, ry: number) => { c.beginPath(); c.ellipse(x, y, rx, ry, 0, 0, Math.PI * 2) }
const circ = (c: Ctx, x: number, y: number, r: number) => { c.beginPath(); c.arc(x, y, r, 0, Math.PI * 2) }

const PAL: Record<TreePalette, [string, string, string, string]> = {
  g1: ['#2c6a40', '#4a9650', '#6cb95c', '#a2dc7e'], g2: ['#3c7a36', '#64a646', '#8cc957', '#c4e88c'],
  pink: ['#a83d64', '#d9618b', '#f191b2', '#ffd5e3'], orange: ['#a8321e', '#d9502c', '#f27c3c', '#ffc27c'],
  beri: ['#21573a', '#3a7d48', '#56a055', '#86c46c'],
}

export interface TreeArt { canvas: HTMLCanvasElement; ox: number; oy: number; w: number; h: number }
const treeCache = new Map<string, TreeArt>()

// treeArt renders one tree (trunk at the origin, origin offset returned) at 2x for crisp zoom; cached per look.
export function treeArt(t: Pick<Tree, 'type' | 'r' | 'pal'>): TreeArt {
  const key = `${t.type}|${t.r}|${t.pal}`
  const hit = treeCache.get(key)
  if (hit) return hit
  const { type, r, pal } = t
  const S = 2, wd = Math.ceil(r * 3 + 20), ht = Math.ceil(r * 3.4 + 24)
  const cv = document.createElement('canvas')
  cv.width = wd * S; cv.height = ht * S
  const c = cv.getContext('2d')!
  c.scale(S, S)
  const ox = wd / 2, oy = ht - 6
  c.translate(ox, oy)
  c.lineJoin = c.lineCap = 'round'
  const P = PAL[pal]
  const lr = mulberry32(r * 97 + type.length)
  const trunk = (h: number, w: number) => {
    poly(c, [[-w * 1.4, 0], [-w, -h], [w, -h], [w * 1.4, 0]]); fs(c, '#8a5a34', 2.5)
    c.strokeStyle = 'rgba(255,255,255,.25)'; c.lineWidth = 2; c.beginPath(); c.moveTo(-w * 0.4, -4); c.lineTo(-w * 0.3, -h + 4); c.stroke()
  }
  const blobs = (cy: number, B: [number, number, number][], p: string[], dots: number) => {
    for (const [dx, dy, f] of B) { circ(c, dx * r, cy + dy * r, f * r + 2.6); c.fillStyle = INK; c.fill() }
    for (const [dx, dy, f] of B) { circ(c, dx * r, cy + dy * r, f * r); c.fillStyle = p[0]!; c.fill() }
    for (const [dx, dy, f] of B) { circ(c, dx * r - f * r * 0.1, cy + dy * r - f * r * 0.14, f * r * 0.86); c.fillStyle = p[1]!; c.fill() }
    for (const [dx, dy, f] of B) { if (dy > 0.3) continue; circ(c, dx * r - f * r * 0.32, cy + dy * r - f * r * 0.38, f * r * 0.42); c.fillStyle = p[2]!; c.fill() }
    c.fillStyle = p[3]!
    for (let i = 0; i < dots; i++) {
      const [dx, dy, f] = B[Math.floor(lr() * B.length)]!
      const a = lr() * 6.28, d = lr() * f * r * 0.7
      circ(c, dx * r + Math.cos(a) * d, cy + dy * r + Math.sin(a) * d - f * r * 0.2, 1.6 + lr() * 1.6); c.fill()
    }
  }
  if (type === 'round' || type === 'bloom') {
    trunk(r * 0.95, r * 0.11)
    const B: [number, number, number][] = [[0, 0.05, 1], [-0.58, 0.2, 0.64], [0.58, 0.2, 0.64], [-0.32, -0.42, 0.62], [0.34, -0.4, 0.6], [0, 0.42, 0.58]]
    if (type === 'bloom') { blobs(-r * 1.5, B.map((b) => [b[0], b[1] + 0.08, b[2] * 0.95]), PAL.g1, 0); blobs(-r * 1.58, B.map((b) => [b[0] * 0.92, b[1] - 0.02, b[2] * 0.86]), P, 26) } else blobs(-r * 1.5, B, P, 10)
  } else if (type === 'beri') {
    for (const dx of [-0.22, 0, 0.2]) { poly(c, [[dx * r - r * 0.1, 0], [dx * r - r * 0.06, -r], [dx * r + r * 0.06, -r], [dx * r + r * 0.1, 0]]); fs(c, '#7a4e2d', 2.5) }
    c.strokeStyle = '#6b4428'; c.lineWidth = 1.8
    for (let i = 0; i < 14; i++) { const x = (lr() - 0.5) * r * 1.9; c.beginPath(); c.moveTo(x, -r * 1.05); c.lineTo(x + (lr() - 0.5) * 4, -r * (0.25 + lr() * 0.45)); c.stroke() }
    blobs(-r * 1.45, [[0, 0, 1], [-0.82, 0.18, 0.7], [0.82, 0.18, 0.7], [-0.45, -0.42, 0.72], [0.45, -0.42, 0.72], [0, -0.62, 0.62], [-1.1, 0.42, 0.5], [1.1, 0.42, 0.5], [0, 0.38, 0.66]], P, 14)
  } else if (type === 'pine') {
    trunk(r * 0.5, r * 0.09)
    for (let i = 0; i < 3; i++) {
      const w = r * (1.05 - i * 0.28), y = -r * 0.45 - i * r * 0.62
      poly(c, [[-w, y], [w, y], [w * 0.2, y - r * 0.9], [0, y - r * 1.05], [-w * 0.2, y - r * 0.9]]); fs(c, '#2f7d55', 2.6)
      poly(c, [[-w * 0.86, y - 3], [0, y - 3], [0, y - r * 0.98], [-w * 0.18, y - r * 0.86]]); c.fillStyle = '#48a26c'; c.fill()
    }
  } else if (type === 'palm') {
    const tx = r * 0.32, ty = -r * 2.05
    c.beginPath(); c.moveTo(0, 0); c.quadraticCurveTo(-r * 0.05, -r * 1.1, tx, ty); c.lineWidth = 11; c.strokeStyle = INK; c.stroke(); c.lineWidth = 7; c.strokeStyle = '#b08155'; c.stroke()
    for (const a of [-2.9, -2.3, -1.75, -1.2, -0.6, -0.1, 0.45]) {
      const ex = tx + Math.cos(a) * r * 1.15, ey = ty + Math.sin(a) * r * 0.75 + r * 0.35, mx = tx + Math.cos(a) * r * 0.6, my = ty + Math.sin(a) * r * 0.5 - r * 0.15
      c.beginPath(); c.moveTo(tx, ty); c.quadraticCurveTo(mx, my, ex, ey); c.lineWidth = 12; c.strokeStyle = INK; c.stroke(); c.lineWidth = 8; c.strokeStyle = '#4f9d4a'; c.stroke(); c.lineWidth = 2; c.strokeStyle = '#8fd06a'; c.stroke()
    }
    for (const [dx, dy] of [[-5, 4], [4, 5], [0, 9]] as Pt[]) { circ(c, tx + dx, ty + dy, 4.5); fs(c, '#7a4e2d', 2) }
  } else if (type === 'bush') blobs(-r * 0.6, [[0, 0, 1], [-0.75, 0.18, 0.72], [0.75, 0.18, 0.72], [0, -0.3, 0.7]], P, pal === 'pink' || pal === 'orange' ? 18 : 6)
  const art = { canvas: cv, ox, oy, w: wd, h: ht }
  treeCache.set(key, art)
  return art
}

const pick2 = (x: number) => (x < 0 ? '#ef4444' : x > 0 ? '#3b82f6' : '#10b981')

// drawDecor paints one prop with its feet at (0,0) of the context. t is seconds, only used by animated props.
export function drawDecor(c: Ctx, d: Decor, t: number) {
  c.save(); c.translate(d.x, d.y); c.lineJoin = c.lineCap = 'round'; c.strokeStyle = INK
  switch (d.t) {
    case 'lamp': ell(c, 0, 0, 7, 3); c.fillStyle = 'rgba(35,32,58,.2)'; c.fill(); rr(c, -4, -6, 8, 6, 2); fs(c, '#3b3f5c', 2); c.beginPath(); c.moveTo(0, -4); c.lineTo(0, -50); c.lineWidth = 5; c.stroke(); c.lineWidth = 3; c.strokeStyle = '#4b5170'; c.stroke(); rr(c, -7, -64, 14, 16, 4); fs(c, '#fff3b0', 2.2); rr(c, -9, -67, 18, 5, 2); fs(c, '#3b3f5c', 2); break
    case 'bench': ell(c, 2, 0, 22, 5); c.fillStyle = 'rgba(35,32,58,.18)'; c.fill(); rr(c, -18, -20, 36, 7, 3); fs(c, '#b9763e', 2.2); rr(c, -20, -12, 40, 8, 3); fs(c, '#d58f50', 2.2); c.fillStyle = INK; c.fillRect(-16, -4, 3, 5); c.fillRect(13, -4, 3, 5); break
    case 'pendopo':
      ell(c, 6, 6, 72, 20); c.fillStyle = 'rgba(35,32,58,.18)'; c.fill(); rr(c, -62, -14, 124, 22, 6); fs(c, '#e9dcc3')
      for (const x of [-50, -17, 17, 50]) { rr(c, x - 4, -60, 8, 48, 2); fs(c, '#9a5a2a', 2.5) }
      poly(c, [[-78, -56], [78, -56], [44, -90], [-44, -90]]); fs(c, '#b8662b'); poly(c, [[-40, -88], [40, -88], [18, -118], [-18, -118]]); fs(c, '#8a4a1c'); rr(c, -24, -123, 48, 8, 4); fs(c, '#5e3112', 2.5); rr(c, -82, -60, 164, 9, 4); fs(c, '#6e3a16', 2.5); break
    case 'flag': {
      c.beginPath(); c.moveTo(0, 0); c.lineTo(0, -96); c.lineWidth = 5; c.stroke(); c.lineWidth = 3; c.strokeStyle = '#cbd5e1'; c.stroke(); circ(c, 0, -98, 4); fs(c, '#ffd23f', 2)
      const wv = (x: number) => Math.sin(t * 4 - x * 0.12) * 3 * (x / 40)
      const band = (y0: number, y1: number, fill: string | null) => {
        c.beginPath(); c.moveTo(2, y0); for (let x = 0; x <= 40; x += 4) c.lineTo(2 + x, y0 + wv(x)); for (let x = 40; x >= 0; x -= 4) c.lineTo(2 + x, y1 + wv(x)); c.closePath()
        if (fill) { c.fillStyle = fill; c.fill() } else { c.strokeStyle = INK; c.lineWidth = 2; c.stroke() }
      }
      band(-92, -80, '#e11d2e'); band(-80, -68, '#fff'); band(-92, -68, null); ell(c, 0, 0, 8, 3); c.fillStyle = '#64748b'; c.fill(); break
    }
    case 'board':
      rr(c, -30, -56, 4, 56, 2); fs(c, '#7a4e2d', 2); rr(c, 26, -56, 4, 56, 2); fs(c, '#7a4e2d', 2); rr(c, -36, -62, 72, 40, 5); fs(c, '#c98a4b'); rr(c, -32, -58, 64, 32, 3); fs(c, '#f3e2c4', 2)
      for (const [cl, x, y] of [['#fde68a', -26, -54], ['#bfdbfe', -6, -52], ['#fbcfe8', 12, -55], ['#bbf7d0', -18, -40], ['#fde68a', 4, -39]] as [string, number, number][]) { rr(c, x, y, 14, 11, 2); fs(c, cl, 1.5) }
      rr(c, -22, -74, 44, 12, 4); fs(c, '#ffd23f', 2); c.fillStyle = INK; c.font = '900 8px Nunito, sans-serif'; c.textAlign = 'center'; c.fillText('MISI', 0, -65); break
    case 'score':
      rr(c, -50, -40, 6, 40, 2); fs(c, '#4b5170', 2); rr(c, 44, -40, 6, 40, 2); fs(c, '#4b5170', 2); rr(c, -64, -92, 128, 56, 8); fs(c, '#2b2a44'); c.fillStyle = '#ffd23f'; c.font = '900 18px Nunito, sans-serif'; c.textAlign = 'center'; c.fillText('KUIS!', 0, -66)
      ;['#ef4444', '#3b82f6', '#f5b70b', '#16a34a'].forEach((cl, i) => { rr(c, -46 + i * 24, -56, 20, 12, 3); c.fillStyle = cl; c.fill(); if (Math.floor(t * 2) % 4 === i) { c.strokeStyle = '#fff'; c.lineWidth = 2; c.stroke() } }); break
    case 'aflag': { c.beginPath(); c.moveTo(0, 0); c.lineTo(0, -54); c.lineWidth = 4; c.stroke(); const s = Math.sin(t * 5 + d.x) * 4; poly(c, [[1, -54], [26, -47 + s], [1, -38]]); fs(c, d.cl ?? '#ef4444', 2); break }
    case 'fountain': {
      const y0 = -30
      ell(c, 0, y0, 60, 36); fs(c, '#d8d2c8'); ell(c, 0, y0, 50, 28); fs(c, '#6ec2ea', 2.5)
      for (let i = 0; i < 3; i++) { const k = (t * 0.6 + i / 3) % 1; ell(c, 0, y0, 10 + k * 38, 5 + k * 21); c.strokeStyle = `rgba(255,255,255,${0.7 * (1 - k)})`; c.lineWidth = 2; c.stroke() }
      c.strokeStyle = INK; rr(c, -8, y0 - 32, 16, 32, 4); fs(c, '#e7e1d6', 2.5); ell(c, 0, y0 - 32, 22, 8); fs(c, '#d8d2c8', 2.5); ell(c, 0, y0 - 33, 16, 5); c.fillStyle = '#93d6f3'; c.fill()
      for (let i = 0; i < 12; i++) { const a = (i / 12) * 6.28, k = (t * 1.2 + i * 0.37) % 1, x = Math.cos(a) * k * 34, y = y0 - 36 - Math.sin(k * Math.PI) * 26 + k * 16 + Math.sin(a) * k * 10; circ(c, x, y, 2.6 * (1 - k * 0.5)); c.fillStyle = `rgba(200,236,255,${1 - k * 0.6})`; c.fill() }
      break
    }
    case 'easel': c.lineWidth = 3; c.beginPath(); c.moveTo(-10, 0); c.lineTo(0, -46); c.lineTo(10, 0); c.moveTo(0, -46); c.lineTo(0, 2); c.stroke(); rr(c, -15, -44, 30, 26, 3); fs(c, '#fff', 2.5); circ(c, -5, -33, 5); c.fillStyle = '#f59e0b'; c.fill(); circ(c, 6, -28, 4); c.fillStyle = '#3b82f6'; c.fill(); break
    case 'sign':
      rr(c, -3, -52, 6, 52, 2); fs(c, '#8a5a34', 2); poly(c, [[-24, -50], [18, -50], [26, -43], [18, -36], [-24, -36]]); fs(c, '#fef3c7', 2.2); poly(c, [[24, -32], [-18, -32], [-26, -25], [-18, -18], [24, -18]]); fs(c, '#e0f2fe', 2.2)
      c.fillStyle = INK; c.font = '800 8px Nunito, sans-serif'; c.textAlign = 'center'; c.fillText('Arena', 0, -40); c.fillText('Perpus', 2, -22); break
    case 'scope': c.lineWidth = 2.5; c.beginPath(); c.moveTo(-9, 0); c.lineTo(0, -24); c.lineTo(9, 0); c.moveTo(0, -24); c.lineTo(0, 0); c.stroke(); c.save(); c.translate(0, -28); c.rotate(-0.5); rr(c, -14, -5, 30, 10, 4); fs(c, '#e2e8f0', 2.5); c.restore(); break
    case 'bikes': for (const x of [-22, 0, 22]) { c.lineWidth = 2.5; circ(c, x - 6, -6, 6); c.stroke(); circ(c, x + 6, -6, 6); c.stroke(); c.beginPath(); c.moveTo(x - 6, -6); c.lineTo(x, -14); c.lineTo(x + 6, -6); c.strokeStyle = pick2(x); c.stroke(); c.strokeStyle = INK } break
  }
  c.restore()
}

// Decor that moves needs a redraw every frame; the rest is baked once.
export const ANIMATED = new Set(['flag', 'aflag', 'fountain', 'score'])

export const GROUND_SCALE = 1.5

// paintGround draws everything static on the floor: grass, shadows, roads, aprons, garden, plaza, arena.
export function paintGround(trees: Tree[]): HTMLCanvasElement {
  const cv = document.createElement('canvas')
  cv.width = W * GROUND_SCALE; cv.height = H * GROUND_SCALE
  const g = cv.getContext('2d')!
  g.scale(GROUND_SCALE, GROUND_SCALE)
  g.lineJoin = g.lineCap = 'round'
  const rnd = mulberry32(3)
  const R = (a: number, b: number) => a + rnd() * (b - a)
  const pick = <T,>(a: T[]) => a[Math.floor(rnd() * a.length)]!
  const ground = world.ground as { color: string }
  g.fillStyle = ground.color; g.fillRect(0, 0, W, H)
  for (let i = 0; i < 520; i++) { circ(g, R(0, W), R(0, H), R(24, 110)); g.fillStyle = rnd() < 0.5 ? 'rgba(110,180,95,.13)' : 'rgba(205,240,170,.16)'; g.fill() }
  // Darker grass toward the map edge, top/bottom then left/right.
  for (const e of [g.createLinearGradient(0, 0, 0, H), g.createLinearGradient(0, 0, W, 0)]) {
    e.addColorStop(0, 'rgba(60,120,70,.34)'); e.addColorStop(0.08, 'rgba(60,120,70,0)'); e.addColorStop(0.92, 'rgba(60,120,70,0)'); e.addColorStop(1, 'rgba(60,120,70,.34)')
    g.fillStyle = e; g.fillRect(0, 0, W, H)
  }
  g.strokeStyle = 'rgba(70,140,70,.45)'; g.lineWidth = 1.4
  for (let i = 0; i < 3200; i++) { const x = R(0, W), y = R(0, H); g.beginPath(); g.moveTo(x - 3, y - 4); g.lineTo(x, y); g.lineTo(x + 3, y - 5); g.stroke() }
  for (let i = 0; i < 160; i++) {
    const x = R(60, W - 60), y = R(60, H - 60)
    if (roadDist(x, y) < 50 || inRect(x, y, ALUN, 10) || inRect(x, y, arenaR(), 10) || footprints.some((b) => inRect(x, y, b, 10))) continue
    const cl = pick(['#ffffff', '#ffe066', '#ff9ec4', '#c3b5ff'])
    for (let k = 0; k < 6; k++) { circ(g, x + R(-12, 12), y + R(-8, 8), 2.3); g.fillStyle = cl; g.fill() }
  }
  // Soft shadows under trees and buildings.
  g.fillStyle = 'rgba(35,32,58,.15)'
  for (const t of trees) { ell(g, t.x + t.r * 0.28, t.y + 1, t.r * (t.type === 'beri' ? 1.35 : t.type === 'palm' ? 0.7 : 1.05), t.r * 0.36); g.fill() }
  for (const b of footprints) { poly(g, [[b.x + b.w - 10, b.y + b.h * 0.35], [b.x + b.w + 22, b.y + b.h * 0.35 + 16], [b.x + b.w + 22, b.y + b.h + 12], [b.x + 18, b.y + b.h + 12], [b.x + 6, b.y + b.h]]); g.fill() }
  // Roads: tan edge, then a paving pattern on top.
  const pv = document.createElement('canvas'); pv.width = pv.height = 32
  { const c = pv.getContext('2d')!; c.fillStyle = '#f2e1ba'; c.fillRect(0, 0, 32, 32); c.strokeStyle = 'rgba(176,138,88,.3)'; c.lineWidth = 1.2; for (const r of [[1, 1, 15, 14], [17, 1, 14, 14], [-7, 17, 15, 14], [9, 17, 15, 14], [25, 17, 15, 14]] as [number, number, number, number][]) c.strokeRect(...r) }
  const pat = g.createPattern(pv, 'repeat')!
  const stroke = (list: Pt[][], w: number, st: string | CanvasPattern) => { g.strokeStyle = st; g.lineWidth = w; for (const s of list) { g.beginPath(); g.moveTo(...s[0]!); for (let i = 1; i < s.length; i++) g.lineTo(...s[i]!); g.stroke() } }
  stroke(ROADS, 66, '#c7a877'); stroke(SPURS, 48, '#c7a877'); stroke(ROADS, 56, pat); stroke(SPURS, 38, pat)
  g.setLineDash([2, 14]); stroke(ROADS, 3, 'rgba(255,255,255,.4)'); g.setLineDash([])
  // Building aprons with flower beds beside each door.
  for (const p of world.places) {
    const { x, y } = p.sprite_pos, { w, h } = p.size
    if (p.id === 'arena') continue
    rr(g, x - 8, y + h - 26, w + 16, 40, 14); g.fillStyle = '#ecdcbc'; g.fill(); g.strokeStyle = '#c7a877'; g.lineWidth = 3; g.stroke()
    for (const sx of [-1, 1]) {
      const bx = p.entrance.x + sx * 58 - 22
      rr(g, bx, y + h - 4, 44, 16, 6); g.fillStyle = '#8a5a3c'; g.fill(); g.strokeStyle = INK; g.lineWidth = 2; g.stroke()
      for (let k = 0; k < 7; k++) { circ(g, bx + 5 + k * 5.5, y + h + 2 + (k % 2) * 5, 2.8); g.fillStyle = pick(['#ff6b8b', '#ffd23f', '#ffffff', '#a78bfa']); g.fill() }
    }
  }
  paintGarden(g); paintPlaza(g); paintArena(g, pat)
  return cv
}

const arenaR = (): Rect => ({ x: ARENA.x, y: ARENA.y, w: ARENA.w, h: ARENA.h })

function paintGarden(g: Ctx) {
  rr(g, TAMAN.x, TAMAN.y, TAMAN.w, TAMAN.h, 46); g.fillStyle = '#92cf7b'; g.fill()
  const gaps: Pt[] = [[395, TAMAN.y], [TAMAN.x + TAMAN.w, 830], [395, TAMAN.y + TAMAN.h], [TAMAN.x, TAMAN.y], [TAMAN.x, TAMAN.y + TAMAN.h]]
  const hedge: Pt[] = []
  for (let x = TAMAN.x + 30; x <= TAMAN.x + TAMAN.w - 30; x += 19) hedge.push([x, TAMAN.y], [x, TAMAN.y + TAMAN.h])
  for (let y = TAMAN.y + 30; y <= TAMAN.y + TAMAN.h - 30; y += 19) hedge.push([TAMAN.x, y], [TAMAN.x + TAMAN.w, y])
  const hh = hedge.filter(([x, y]) => !gaps.some(([gx, gy]) => Math.hypot(x - gx, y - gy) < 46))
  for (const [x, y] of hh) { circ(g, x, y, 14.5); g.fillStyle = INK; g.fill() }
  for (const [x, y] of hh) { circ(g, x, y, 12); g.fillStyle = '#3f8a4f'; g.fill(); circ(g, x - 3, y - 4, 5); g.fillStyle = '#62ad5e'; g.fill() }
  // Stepping stones between garden spots.
  const N: Record<string, Pt> = { tN: [395, 652], tC: [410, 880], tE: [590, 830], tS: [395, 930], tW: [205, 655], tW2: [205, 925] }
  for (const [a, b] of [['tN', 'tC'], ['tC', 'tE'], ['tC', 'tS'], ['tW', 'tC'], ['tW2', 'tC']] as [string, string][]) {
    const [x0, y0] = N[a]!, [x1, y1] = N[b]!, L = Math.hypot(x1 - x0, y1 - y0)
    for (let d = 12; d < L; d += 26) {
      const x = x0 + ((x1 - x0) * d) / L, y = y0 + ((y1 - y0) * d) / L
      if (Math.hypot((x - POND.x) / POND.rx, (y - POND.y) / POND.ry) < 1.15) continue
      ell(g, x, y, 11, 7); g.fillStyle = '#e9e3d6'; g.fill(); g.strokeStyle = '#b9ae9c'; g.lineWidth = 2; g.stroke()
    }
  }
  ell(g, POND.x, POND.y, POND.rx + 10, POND.ry + 9); g.fillStyle = '#cfc6b4'; g.fill(); g.strokeStyle = INK; g.lineWidth = 3; g.stroke()
  ell(g, POND.x, POND.y, POND.rx, POND.ry); g.fillStyle = '#6ec2ea'; g.fill(); g.stroke()
  ell(g, POND.x - 14, POND.y - 8, POND.rx * 0.6, POND.ry * 0.5); g.fillStyle = '#93d6f3'; g.fill()
  for (const [dx, dy, r] of [[-40, -10, 11], [40, 15, 9], [-5, 28, 8], [60, -20, 7]] as [number, number, number][]) {
    g.beginPath(); g.arc(POND.x + dx, POND.y + dy, r, 0.35, Math.PI * 2 - 0.1); g.lineTo(POND.x + dx, POND.y + dy); g.closePath(); g.fillStyle = '#4fae5e'; g.fill(); g.strokeStyle = INK; g.lineWidth = 1.5; g.stroke()
  }
}

function paintPlaza(g: Ctx) {
  const tl = document.createElement('canvas'); tl.width = tl.height = 28
  { const c = tl.getContext('2d')!; c.fillStyle = '#eadabb'; c.fillRect(0, 0, 28, 28); c.fillStyle = '#e2cfab'; c.fillRect(0, 0, 14, 14); c.fillRect(14, 14, 14, 14); c.strokeStyle = 'rgba(160,128,80,.25)'; c.strokeRect(0, 0, 28, 28) }
  rr(g, ALUN.x, ALUN.y, ALUN.w, ALUN.h, 40); g.fillStyle = g.createPattern(tl, 'repeat')!; g.fill(); g.strokeStyle = '#c7a877'; g.lineWidth = 7; g.stroke()
  const cy = FOUNT.y + 25
  circ(g, FOUNT.x, cy, 138); g.fillStyle = '#d9c49a'; g.fill()
  circ(g, FOUNT.x, cy, 128); g.fillStyle = '#a3d88b'; g.fill(); g.strokeStyle = '#7fbf6d'; g.lineWidth = 3; g.stroke()
  for (let a = 0; a < 6.28; a += 0.13) { circ(g, FOUNT.x + Math.cos(a) * 120, cy + Math.sin(a) * 120, 3.4); g.fillStyle = ['#ff6b8b', '#ffd23f', '#ffffff'][Math.round(a * 7.7) % 3]!; g.fill() }
  // Paved spokes from the fountain to the plaza exits.
  for (const [x, y] of [[FOUNT.x, ALUN.y + 12], [ALUN.x + 12, 830], [ALUN.x + ALUN.w - 12, 830], [FOUNT.x, ALUN.y + ALUN.h - 12], [985, 935], [1215, 935]] as Pt[]) {
    const dx = x - FOUNT.x, dy = y - cy, L = Math.hypot(dx, dy)
    g.strokeStyle = '#e7d6b4'; g.lineWidth = 26; g.beginPath(); g.moveTo(FOUNT.x + (dx / L) * 70, cy + (dy / L) * 70); g.lineTo(x, y); g.stroke()
  }
}

function paintArena(g: Ctx, pat: CanvasPattern) {
  const { cx, cy } = ARENA
  ell(g, cx, cy, 228, 134); g.fillStyle = '#f5d7b0'; g.fill(); g.strokeStyle = INK; g.lineWidth = 3; g.stroke()
  for (const [cl, rx, ry] of [['#ef6f3c', 220, 127], ['#f6a05c', 206, 115], ['#ef6f3c', 192, 103], ['#f6a05c', 178, 92]] as [string, number, number][]) { ell(g, cx, cy, rx, ry); g.fillStyle = cl; g.fill(); g.strokeStyle = 'rgba(35,32,58,.35)'; g.lineWidth = 1.5; g.stroke() }
  ell(g, cx, cy, 164, 82); g.fillStyle = '#d9573a'; g.fill(); g.strokeStyle = INK; g.lineWidth = 3; g.stroke()
  g.setLineDash([10, 8]); ell(g, cx, cy, 152, 72); g.strokeStyle = 'rgba(255,255,255,.7)'; g.lineWidth = 2; g.stroke(); g.setLineDash([])
  ell(g, cx, cy, 138, 62); g.fillStyle = '#7fd47a'; g.fill(); g.strokeStyle = INK; g.lineWidth = 3; g.stroke()
  g.save(); ell(g, cx, cy, 138, 62); g.clip(); for (let x = cx - 140; x < cx + 140; x += 28) { g.fillStyle = 'rgba(255,255,255,.12)'; g.fillRect(x, cy - 70, 14, 140) } g.restore()
  const pads: [number, number, string, string][] = [[-52, -22, '#ef4444', 'tri'], [52, -22, '#3b82f6', 'dia'], [-52, 22, '#f5b70b', 'cir'], [52, 22, '#16a34a', 'sq']]
  for (const [dx, dy, cl, sh] of pads) {
    rr(g, cx + dx - 30, cy + dy - 17, 60, 34, 10); g.fillStyle = cl; g.fill(); g.strokeStyle = INK; g.lineWidth = 3; g.stroke()
    g.fillStyle = '#fff'
    const x = cx + dx, y = cy + dy
    if (sh === 'tri') { poly(g, [[x - 9, y + 7], [x + 9, y + 7], [x, y - 9]]); g.fill() } else if (sh === 'dia') { poly(g, [[x, y - 10], [x + 10, y], [x, y + 10], [x - 10, y]]); g.fill() } else if (sh === 'cir') { circ(g, x, y, 8); g.fill() } else g.fillRect(x - 8, y - 8, 16, 16)
  }
  rr(g, ARENA.x - 4, ARENA.cy + 49, 72, 36, 8); g.fillStyle = pat; g.fill()
}

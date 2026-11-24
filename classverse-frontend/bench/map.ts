// Dev-only stress page (never built: index.html is the only build input). Run with the Vite dev server:
//   http://localhost:5173/bench/map.html?n=200&moving=1
// It feeds n fake avatars into the real World + CampusScene through fake server snapshots at 10 Hz, then reports
// frame times. The map comes from /api/maps/campus (backend must run). No login, no websocket.
import Phaser from 'phaser'
import '@/styles/tokens.css'
import { CampusScene } from '@/game/CampusScene'
import { avatarKeys } from '@/game/avatars'
import { fetchMap } from '@/realtime/mapdata'
import { World } from '@/realtime/world'
import type { Direction, ServerMessage } from '@/realtime/messages'

const q = new URLSearchParams(location.search)
const n = Number(q.get('n') ?? 200), moving = q.get('moving') !== '0', seconds = Number(q.get('s') ?? 10)
const out = document.getElementById('out')!

const map = await fetchMap('campus')
const world = new World()
let seed = 1
const rnd = () => ((seed = (seed * 16807) % 2147483647) / 2147483647)
const open = () => { for (;;) { const x = rnd() * map.pixelWidth, y = rnd() * map.pixelHeight; if (!map.isBlocked(x, y)) return { x, y } } }
const people = Array.from({ length: n }, (_, i) => ({ id: `u${i}`, ...open(), a: rnd() * 6.28 }))
const now = () => performance.now()
const welcome = {
  t: 'welcome', seq: 0,
  d: {
    self_id: 'me', room_id: 'bench', map_key: 'campus', all_chat_enabled: true,
    participants: [{ id: 'me', name: 'Saya', avatar: 'default', role: 'participant', x: 1100, y: 830, dir: 'down' },
      ...people.map((p, i) => ({ id: p.id, name: `Siswa ${i}`, avatar: avatarKeys[i % avatarKeys.length]!, role: 'participant', x: Math.round(p.x), y: Math.round(p.y), dir: 'down' }))],
  },
} as ServerMessage
world.apply(welcome, now())

// 10 Hz like the server tick: every avatar steps 15 px (150 px/s) along its heading, turning at walls.
const dirOf = (a: number): Direction => (Math.abs(Math.cos(a)) > Math.abs(Math.sin(a)) ? (Math.cos(a) > 0 ? 'right' : 'left') : Math.sin(a) > 0 ? 'down' : 'up')
if (moving) setInterval(() => {
  for (const p of people) {
    const x = p.x + Math.cos(p.a) * 15, y = p.y + Math.sin(p.a) * 15
    if (map.isBlocked(x, y)) p.a += 1.7 + rnd(); else { p.x = x; p.y = y }
  }
  world.apply({ t: 'snapshot', seq: 0, d: { p: people.map((p) => ({ id: p.id, x: Math.round(p.x), y: Math.round(p.y), dir: dirOf(p.a) })) } } as ServerMessage, now())
}, 100)

// GPU-independent metrics: JS time in the scene's update() and WebGL draw calls per frame. Headless Chrome here
// renders on SwiftShader (software), so fps mostly measures the CPU rasteriser, not the app.
const updateMs: number[] = [], drawCalls: number[] = []
let calls = 0
const origUpdate = CampusScene.prototype.update
CampusScene.prototype.update = function (this: CampusScene, time: number, delta: number) {
  const t0 = performance.now()
  origUpdate.call(this, time, delta)
  updateMs.push(performance.now() - t0)
}
const game = new Phaser.Game({
  type: Phaser.AUTO, parent: 'host', transparent: true,
  scale: { mode: Phaser.Scale.RESIZE, width: '100%', height: '100%' },
  render: { antialias: true, roundPixels: false }, fps: { smoothStep: false }, banner: false, audio: { noAudio: true }, scene: [],
})
game.scene.add('campus', CampusScene, true, { map, world, sendMove: () => {} })
game.events.once(Phaser.Core.Events.READY, () => {
  const gl = (game.renderer as Phaser.Renderer.WebGL.WebGLRenderer).gl
  for (const name of ['drawArrays', 'drawElements'] as const) {
    const orig = gl[name].bind(gl) as (...a: unknown[]) => void
    ;(gl as unknown as Record<string, unknown>)[name] = (...a: unknown[]) => { calls++; orig(...a) }
  }
  game.events.on(Phaser.Core.Events.POST_RENDER, () => { drawCalls.push(calls); calls = 0 })
})

// Skip 2 s of warm-up (texture builds), then sample rAF intervals for `seconds`.
const frames: number[] = []
let last = 0, start = 0
const loop = (t: number) => {
  if (!start) start = t
  if (t - start > 2000 && last) frames.push(t - last)
  last = t
  if (t - start < 2000 + seconds * 1000) return requestAnimationFrame(loop)
  const s = [...frames].sort((a, b) => a - b), pct = (k: number) => s[Math.floor(s.length * k)]!.toFixed(1)
  const tail = (xs: number[]) => xs.slice(-Math.max(1, s.length)).sort((a, b) => a - b)
  const u = tail(updateMs), dc = tail(drawCalls)
  const mem = (performance as unknown as { memory?: { usedJSHeapSize: number } }).memory
  const gl = (game.renderer as Phaser.Renderer.WebGL.WebGLRenderer).gl
  const dbg = gl?.getExtension('WEBGL_debug_renderer_info')
  const result = {
    n, moving, frames: s.length, fps: +(1000 / (s.reduce((a, b) => a + b, 0) / s.length)).toFixed(1),
    p50: +pct(0.5), p95: +pct(0.95), p99: +pct(0.99), heapMB: mem ? +(mem.usedJSHeapSize / 1e6).toFixed(0) : null,
    updateP50: +u[Math.floor(u.length / 2)]!.toFixed(2), updateP95: +u[Math.floor(u.length * 0.95)]!.toFixed(2),
    drawCallsP50: dc[Math.floor(dc.length / 2)]!, drawCallsMax: dc.at(-1)!,
    textures: game.textures.getTextureKeys().length, gpu: dbg ? gl.getParameter(dbg.UNMASKED_RENDERER_WEBGL) : 'unknown',
  }
  out.textContent = JSON.stringify(result, null, 1)
  ;(window as unknown as { benchResult: unknown }).benchResult = result
}
requestAnimationFrame(loop)

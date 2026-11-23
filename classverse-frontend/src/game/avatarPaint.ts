// Canvas painter for map avatars (ported from the map-revamp prototype): ink outline, foot shadow, swinging legs
// and arms while walking, a slight body bob. It draws from the saved look (skin/hair/shirt) so every stored
// avatar key still resolves to the same person; nothing here is sent to or read from the server.
import { avatarLook, avatarPalette } from './avatars'
import type { Direction } from '@/realtime/messages'

const INK = '#23203a'
type Ctx = CanvasRenderingContext2D

// Frames per walk cycle: 0 = standing, 1..2 = alternating strides.
export const WALK_FRAMES = 3
export const FRAME_W = 40
export const FRAME_H = 64
// Feet sit this many px above the frame's bottom edge (room for the shadow).
export const FOOT_PAD = 6

const DIRS: Direction[] = ['down', 'left', 'right', 'up']

const rr = (c: Ctx, x: number, y: number, w: number, h: number, r: number) => { c.beginPath(); c.roundRect(x, y, w, h, r) }
const circ = (c: Ctx, x: number, y: number, r: number) => { c.beginPath(); c.arc(x, y, r, 0, Math.PI * 2) }

// stride is the walk phase for a frame: 0 stands, 1 and 2 swing opposite legs.
const stride = (frame: number) => (frame === 1 ? 1 : frame === 2 ? -1 : 0)

// paintAvatar draws one pose with its feet at (0,0) of the context.
export function paintAvatar(c: Ctx, look: { skin: string; hair: string; shirt: string }, dir: Direction, frame: number) {
  const s = stride(frame)
  const walking = frame !== 0
  const bob = walking ? -1.6 : 0
  c.save()
  c.lineJoin = c.lineCap = 'round'
  c.beginPath(); c.ellipse(0, 0, 11, 4, 0, 0, Math.PI * 2); c.fillStyle = 'rgba(35,32,58,.25)'; c.fill()
  c.lineWidth = 2; c.strokeStyle = INK
  const pants = '#334155'
  const l1 = s > 0 ? 3 : 0, l2 = s < 0 ? 3 : 0
  rr(c, -6.5, -12 - l1, 5.5, 11, 2); c.fillStyle = pants; c.fill(); c.stroke()
  rr(c, 1, -12 - l2, 5.5, 11, 2); c.fill(); c.stroke()
  c.fillStyle = '#2b2230'; rr(c, -7, -3 - l1, 6.5, 3.5, 1.5); c.fill(); rr(c, 0.5, -3 - l2, 6.5, 3.5, 1.5); c.fill()
  const sw = s * 2.5
  c.fillStyle = look.skin
  rr(c, -12, -23 + bob + sw, 4.5, 10, 2); c.fill(); c.stroke()
  rr(c, 7.5, -23 + bob - sw, 4.5, 10, 2); c.fill(); c.stroke()
  rr(c, -9, -26 + bob, 18, 16, 5); c.fillStyle = look.shirt; c.fill(); c.stroke()
  const hy = -34 + bob
  circ(c, 0, hy, 10.5); c.fillStyle = look.skin; c.fill(); c.stroke()
  c.fillStyle = look.hair
  if (dir === 'up') { circ(c, 0, hy - 0.5, 10.8); c.fill(); c.stroke() } else {
    c.beginPath(); c.arc(0, hy, 11, Math.PI * 1.02, Math.PI * 1.98); c.quadraticCurveTo(4, hy - 3, -2, hy - 4.5); c.quadraticCurveTo(-7, hy - 3, -11, hy - 0.5); c.closePath(); c.fill(); c.stroke()
  }
  if (dir !== 'up') {
    const ex = dir === 'left' ? -3 : dir === 'right' ? 3 : 0
    c.fillStyle = INK; circ(c, ex - 3.6, hy + 1.5, 1.6); c.fill(); circ(c, ex + 3.6, hy + 1.5, 1.6); c.fill()
    c.fillStyle = 'rgba(255,120,140,.45)'; circ(c, ex - 6, hy + 5, 2); c.fill(); circ(c, ex + 6, hy + 5, 2); c.fill()
  }
  c.restore()
}

export interface AvatarSheet { canvas: HTMLCanvasElement; frameW: number; frameH: number; cols: number; rows: number }

// avatarSheet renders every direction x walk frame of one saved avatar key into a single sheet:
// rows = directions (down,left,right,up), columns = frames.
export function avatarSheet(key: string): AvatarSheet {
  const look = avatarLook(key)
  const colours = { skin: avatarPalette.skin[look.skin]!, hair: avatarPalette.hair[look.hair]!, shirt: avatarPalette.shirt[look.shirt]! }
  const S = 2
  const cv = document.createElement('canvas')
  cv.width = FRAME_W * WALK_FRAMES * S; cv.height = FRAME_H * DIRS.length * S
  const c = cv.getContext('2d')!
  c.scale(S, S)
  DIRS.forEach((dir, row) => {
    for (let f = 0; f < WALK_FRAMES; f++) {
      c.save(); c.translate(f * FRAME_W + FRAME_W / 2, row * FRAME_H + FRAME_H - FOOT_PAD)
      paintAvatar(c, colours, dir, f)
      c.restore()
    }
  })
  return { canvas: cv, frameW: FRAME_W, frameH: FRAME_H, cols: WALK_FRAMES, rows: DIRS.length }
}

// frameIndex maps a direction and walk frame to the sheet's frame number (row-major).
export const frameIndex = (dir: Direction, frame: number) => DIRS.indexOf(dir) * WALK_FRAMES + frame

// STEP_PX: a new stride frame every this many pixels travelled, so legs follow speed instead of a fixed clock.
export const STEP_PX = 9

// walkFrame picks the pose from distance travelled: 0 while still, otherwise alternating 1,2.
export function walkFrame(moving: boolean, travelled: number): number {
  if (!moving) return 0
  return 1 + (Math.floor(travelled / STEP_PX) % 2)
}

import { beforeEach, expect, it, vi } from 'vitest'
import type Phaser from 'phaser'
import { CampusScene, clampCentre } from './CampusScene'
import { findPath } from '@/realtime/pathfind'

vi.mock('phaser', () => ({ default: { Scene: class {} } }))
vi.mock('@/realtime/pathfind', () => ({ findPath: vi.fn<() => { x: number; y: number }[]>(() => [{ x: 90, y: 90 }]) }))

type InputScene = {
  deps: unknown
  path: unknown[]
  tablePenalty: () => undefined
  onPointerDown: (p: Phaser.Input.Pointer) => void
  onPointerMove: (p: Phaser.Input.Pointer) => void
  onPointerUp: (p: Phaser.Input.Pointer) => void
  cameras: { main: { zoom: number; midPoint: { x: number; y: number }; centerOn: (x: number, y: number) => void } }
  scale: { width: number; height: number }
}
const pointer = (time: number, x = 90, y = 90) => ({
  downTime: time, x, y, worldX: x, worldY: y, leftButtonDown: () => true,
}) as Phaser.Input.Pointer
let scene: InputScene
const selectBuilding = vi.fn<(key: string) => void>()
beforeEach(() => {
  vi.clearAllMocks()
  scene = new CampusScene() as unknown as InputScene
  scene.deps = {
    world: { self: { x: 10, y: 10 }, mapKey: 'campus' },
    map: { buildings: [{ key: 'sains', x: 200, y: 200, w: 100, h: 100 }] },
    selectBuilding,
  }
  scene.tablePenalty = () => undefined
  scene.cameras = { main: { zoom: 1, midPoint: { x: 500, y: 400 }, centerOn: vi.fn<(x: number, y: number) => void>() } }
  scene.scale = { width: 400, height: 300 }
})
// A campus click is a press and release without dragging.
const click = (p: Phaser.Input.Pointer) => { scene.onPointerDown(p); scene.onPointerUp(p) }

it('walks only on two nearby clicks within 400 ms, not a single or third click', () => {
  click(pointer(1000))
  expect(findPath).not.toHaveBeenCalled()
  click(pointer(1200, 94))
  expect(findPath).toHaveBeenCalledTimes(1)
  click(pointer(1300, 94))
  expect(findPath).toHaveBeenCalledTimes(1)
})

it('rejects clicks too far apart in time or screen position and right clicks', () => {
  click(pointer(1000))
  click(pointer(1500))
  click(pointer(1600, 140))
  click({ ...pointer(1700, 140), leftButtonDown: () => false } as Phaser.Input.Pointer)
  expect(findPath).not.toHaveBeenCalled()
})

it('selects a building with one click and does not reuse it to start walking', () => {
  click(pointer(1000, 210, 210))
  expect(selectBuilding).toHaveBeenCalledWith('sains')
  click(pointer(1100, 198, 210))
  expect(findPath).not.toHaveBeenCalled()
})

it('drags the campus without selecting or walking, clamped to the map edge', () => {
  scene.deps = { ...(scene.deps as object), map: { buildings: [{ key: 'sains', x: 200, y: 200, w: 100, h: 100 }], pixelWidth: 2200, pixelHeight: 1500 } }
  scene.onPointerDown(pointer(1000, 210, 210))
  scene.onPointerMove({ ...pointer(1010, 150, 210), isDown: true } as unknown as Phaser.Input.Pointer)
  expect(scene.cameras.main.centerOn).toHaveBeenLastCalledWith(560, 400)
  scene.onPointerUp(pointer(1020, 150, 210))
  expect(selectBuilding).not.toHaveBeenCalled()
  click(pointer(1100, 90, 90)); click(pointer(1200, 90, 90))
  expect(findPath).toHaveBeenCalledTimes(1)
})

it('centres a map smaller than the view and clamps a larger one', () => {
  expect(clampCentre(1000, 1200, 50)).toBe(500)
  expect(clampCentre(2200, 1000, 100)).toBe(500)
  expect(clampCentre(2200, 1000, 2100)).toBe(1700)
  expect(clampCentre(2200, 1000, 900)).toBe(900)
})

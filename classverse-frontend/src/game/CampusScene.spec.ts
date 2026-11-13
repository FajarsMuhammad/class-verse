import { beforeEach, expect, it, vi } from 'vitest'
import type Phaser from 'phaser'
import { CampusScene } from './CampusScene'
import { findPath } from '@/realtime/pathfind'

vi.mock('phaser', () => ({ default: { Scene: class {} } }))
vi.mock('@/realtime/pathfind', () => ({ findPath: vi.fn<() => { x: number; y: number }[]>(() => [{ x: 90, y: 90 }]) }))

type InputScene = {
  deps: unknown
  path: unknown[]
  tablePenalty: () => undefined
  onPointerDown: (p: Phaser.Input.Pointer) => void
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
})

it('walks only on two nearby clicks within 400 ms, not a single or third click', () => {
  scene.onPointerDown(pointer(1000))
  expect(findPath).not.toHaveBeenCalled()
  scene.onPointerDown(pointer(1200, 94))
  expect(findPath).toHaveBeenCalledTimes(1)
  scene.onPointerDown(pointer(1300, 94))
  expect(findPath).toHaveBeenCalledTimes(1)
})

it('rejects clicks too far apart in time or screen position and right clicks', () => {
  scene.onPointerDown(pointer(1000))
  scene.onPointerDown(pointer(1500))
  scene.onPointerDown(pointer(1600, 140))
  scene.onPointerDown({ ...pointer(1700, 140), leftButtonDown: () => false } as Phaser.Input.Pointer)
  expect(findPath).not.toHaveBeenCalled()
})

it('selects a building with one click and does not reuse it to start walking', () => {
  scene.onPointerDown(pointer(1000, 210, 210))
  expect(selectBuilding).toHaveBeenCalledWith('sains')
  scene.onPointerDown(pointer(1100, 198, 210))
  expect(findPath).not.toHaveBeenCalled()
})

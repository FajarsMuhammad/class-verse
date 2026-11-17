import { expect, it } from 'vitest'
import { doorExit } from './doorExit'

it('does not leave at spawn, arms outside door, exits once on return', () => {
  expect(doorExit(false, 0, true)).toEqual({ armed: false, exit: false })
  expect(doorExit(false, 57, true)).toEqual({ armed: true, exit: false })
  expect(doorExit(true, 48, true)).toEqual({ armed: true, exit: false })
  expect(doorExit(true, 40, true)).toEqual({ armed: false, exit: true })
  expect(doorExit(false, 40, true)).toEqual({ armed: false, exit: false })
})
it('resets during disconnection, travel, or missing position', () => {
  expect(doorExit(true, 0, false)).toEqual({ armed: false, exit: false })
  expect(doorExit(true, Infinity, true)).toEqual({ armed: false, exit: false })
})

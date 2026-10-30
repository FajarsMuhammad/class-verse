import { describe, expect, it } from 'vitest'

import { safeRedirect } from '../pending'

describe('safeRedirect', () => {
  it.each([
    ['/gabung/ABCD-EFGH', '/gabung/ABCD-EFGH'],
    ['/', '/'],
    ['//evil.example', null],
    ['/\\evil.example', null],
    ['https://evil.example', null],
    ['javascript:alert(1)', null],
    [undefined, null],
    [['/a'], null],
  ])('%s -> %s', (input, want) => {
    expect(safeRedirect(input)).toBe(want)
  })
})

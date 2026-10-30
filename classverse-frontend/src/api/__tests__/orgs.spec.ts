import { describe, expect, it } from 'vitest'

import { normalizeInviteCode, slugify } from '../orgs'

describe('normalizeInviteCode', () => {
  it.each([
    ['abcd-efgh', 'ABCD-EFGH'],
    [' ABCD EFGH ', 'ABCD-EFGH'],
    ['abcdefgh', 'ABCD-EFGH'],
    ['ABCD-EFG0', null],
    ['ABC', null],
  ])('%s -> %s', (input, want) => {
    expect(normalizeInviteCode(input)).toBe(want)
  })
})

describe('slugify', () => {
  it.each([
    ['SMA Negeri 1 Bandung', 'sma-negeri-1-bandung'],
    ['  Sekolah Ç@bang  ', 'sekolah-c-bang'],
    ['x'.repeat(60), 'x'.repeat(48)],
  ])('%s', (input, want) => {
    expect(slugify(input)).toBe(want)
  })
})

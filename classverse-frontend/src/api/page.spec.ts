import { expect, test } from 'vitest'
import { hasMore, lastPage } from './page'

test('hasMore uses the server total, else a full page', () => {
  expect(hasMore(0, { items: Array(20), total: 20 })).toBe(false)
  expect(hasMore(0, { items: Array(20), total: 21 })).toBe(true)
  expect(hasMore(20, { items: Array(1), total: 21 })).toBe(false)
  expect(hasMore(0, { items: Array(20) })).toBe(true)
  expect(hasMore(0, { items: Array(3) })).toBe(false)
})

test('lastPage steps back from an empty page past the end', () => {
  expect(lastPage(3, { items: [] })).toBe(2)
  expect(lastPage(1, { items: [] })).toBe(1)
  expect(lastPage(2, { items: [1] })).toBe(2)
})

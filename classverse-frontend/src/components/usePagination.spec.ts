import { ref, nextTick } from 'vue'
import { expect, it } from 'vitest'
import { usePagination } from './usePagination'
it('pages all rows and resets after filtering', async () => {
  const items = ref(Array.from({ length: 45 }, (_, i) => i))
  const pager = usePagination(items)
  expect(pager.rows.value).toHaveLength(20)
  pager.page.value = 3
  expect(pager.rows.value).toEqual([40, 41, 42, 43, 44])
  items.value = [9]
  await nextTick()
  expect(pager.page.value).toBe(1)
  expect(pager.rows.value).toEqual([9])
})

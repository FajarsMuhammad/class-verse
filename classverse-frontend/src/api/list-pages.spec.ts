import { expect, it, vi } from 'vitest'
import { api } from './client'
import { moduleApi } from './modules'
import { quizApi } from './quizzes'
vi.mock('./client', () => ({ api: vi.fn<typeof api>() }))
it('requests only the chosen module page even when server returns 100 rows', async () => {
  vi.mocked(api).mockResolvedValue({ items: Array.from({ length: 100 }, () => ({})), total: 250 })
  await moduleApi.list('org', { offset: 100, limit: 20, search: 'planet', status: 'draft' })
  expect(api).toHaveBeenCalledTimes(1)
  expect(api).toHaveBeenCalledWith(expect.stringContaining('offset=100'))
  expect(api).toHaveBeenCalledWith(expect.stringContaining('limit=20'))
  vi.mocked(api).mockClear()
  await quizApi.list('org', { offset: 20, limit: 20 })
  expect(api).toHaveBeenCalledTimes(1)
  expect(api).toHaveBeenCalledWith(expect.stringContaining('offset=20'))
})

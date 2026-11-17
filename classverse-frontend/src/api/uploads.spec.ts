import { beforeEach, expect, it, vi } from 'vitest'
import { uploadFile } from './uploads'
import { api } from './client'

vi.mock('./client', async (original) => ({ ...await original<typeof import('./client')>(), api: vi.fn<typeof import('./client').api>() }))
const call = vi.mocked(api)
const state = { id: 'upload', size: 5 * 1024 * 1024 + 4, chunk_size: 5 * 1024 * 1024, parts: [] as number[], state: 'uploading', expires_at: '2099-01-01' }

beforeEach(() => {
  localStorage.clear()
  call.mockReset()
  vi.stubGlobal('crypto', { randomUUID: () => 'upload', subtle: { digest: vi.fn<SubtleCrypto['digest']>().mockResolvedValue(new Uint8Array(32).buffer) } })
})

it('resumes server-confirmed chunks after a failed request without another reservation', async () => {
  const file = new File([new Uint8Array(state.size)], 'book.pdf')
  Object.defineProperty(file, 'arrayBuffer', { value: async () => new ArrayBuffer(1) })
  call.mockResolvedValueOnce(state).mockResolvedValueOnce(undefined).mockRejectedValueOnce(new TypeError('offline'))
  await expect(uploadFile('org', 'user', file, 'pdf', vi.fn())).rejects.toThrow('offline')
  call.mockReset()
  call.mockResolvedValueOnce({ ...state, parts: [1] }).mockResolvedValueOnce(undefined).mockResolvedValueOnce({ ...state, state: 'assembled' })
  await expect(uploadFile('org', 'user', file, 'pdf', vi.fn())).resolves.toMatchObject({ state: 'assembled' })
  expect(call.mock.calls.map(([path]) => path)).toEqual([
    '/api/orgs/org/uploads/upload', '/api/orgs/org/uploads/upload/parts/2', '/api/orgs/org/uploads/upload/complete',
  ])
  expect((call.mock.calls[1]![1]!.body as Blob).size).toBe(4)
})

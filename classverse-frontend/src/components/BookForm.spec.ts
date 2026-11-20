import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, expect, it, vi } from 'vitest'
import BookForm from './BookForm.vue'
import { ApiError } from '@/api/client'
import { fileApi } from '@/api/files'
import { libraryApi } from '@/api/library'
import type { Book } from '@/api/types'

afterEach(() => vi.restoreAllMocks())
const pdf = { id: 'pdf1', name: 'tata.pdf', kind: 'pdf' as const, content_type: 'application/pdf' as const, size: 1, uploaded_by: 'u', created_at: '', position: 0 }
const book = { id: 'b2', title: 'Tata Surya', author: '', shelf: 'sains', level: 'SD', format: 'ebook', copies: 0, file_id: 'pdf1', cover: 'ipa',
  published: true, sample: false, position: 0, updated_at: '', available: 0, my_loan: null } as Book

it('lists only files of the chosen kind and updates the readiness checklist', async () => {
  const files = vi.spyOn(fileApi, 'list').mockResolvedValue({ used_bytes: 0, quota_bytes: 1, total: 1, items: [pdf] })
  const w = mount(BookForm, { props: { orgId: 'o', userId: 'u' } })
  await flushPromises()
  expect(files).toHaveBeenLastCalledWith('o', 0, 'pdf')
  expect(w.get('.checks').text()).toContain('File belum dipilih')
  await w.get('input[value="pdf1"]').setValue(true)
  await w.get('input[id$="-t"]').setValue('Tata Surya')
  expect(w.get('.checks').text()).toContain('File dipilih')
  expect(w.get('.cover').text()).toContain('Tata Surya')
  await w.get('input[value="audio"]').setValue(true)
  await flushPromises()
  expect(files).toHaveBeenLastCalledWith('o', 0, 'audio')
  await w.get('input[value="buku"]').setValue(true)
  await flushPromises()
  expect(w.find('input[type=number]').exists()).toBe(true)
})

it('publishes an e-book without author or copies and shows server field errors', async () => {
  vi.spyOn(fileApi, 'list').mockResolvedValue({ used_bytes: 0, quota_bytes: 1, total: 1, items: [pdf] })
  const create = vi.spyOn(libraryApi, 'create')
    .mockRejectedValueOnce(new ApiError(400, { code: 'validation_failed', message: 'x', fields: { title: 'Judul 1–200 karakter' } }))
    .mockResolvedValueOnce(book)
  const w = mount(BookForm, { props: { orgId: 'o', userId: 'u' } })
  await flushPromises()
  await w.get('input[value="pdf1"]').setValue(true)
  await w.get('form').trigger('submit')
  await flushPromises()
  expect(w.get('.field-error').text()).toBe('Judul 1–200 karakter')
  expect(w.get('input[id$="-t"]').attributes('aria-invalid')).toBe('true')
  await w.get('input[id$="-t"]').setValue('Tata Surya')
  await w.get('form').trigger('submit')
  await flushPromises()
  expect(create).toHaveBeenLastCalledWith('o', expect.objectContaining({ title: 'Tata Surya', author: '', format: 'ebook', file_id: 'pdf1', copies: 0, published: true }))
  expect(w.emitted('saved')?.[0]).toEqual([book])
})

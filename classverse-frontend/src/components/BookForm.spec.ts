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
  expect(w.find('[aria-label="Tambah eksemplar"]').exists()).toBe(true)
})

it('blocks saving until title and file are set, then publishes without author', async () => {
  vi.spyOn(fileApi, 'list').mockResolvedValue({ used_bytes: 0, quota_bytes: 1, total: 1, items: [pdf] })
  const create = vi.spyOn(libraryApi, 'create').mockResolvedValue(book)
  const w = mount(BookForm, { props: { orgId: 'o', userId: 'u' } })
  await flushPromises()
  await w.get('form').trigger('submit')
  await flushPromises()
  expect(create).not.toHaveBeenCalled()
  expect(w.get('.banner.bad').text()).toBe('Lengkapi dulu: judul dan file.')
  expect(w.get('input[id$="-t"]').attributes('aria-invalid')).toBe('true')
  expect(w.get('.field-error').text()).toBe('Judul belum diisi.')
  await w.get('input[value="pdf1"]').setValue(true)
  await w.get('input[id$="-t"]').setValue('Tata Surya')
  await w.get('form').trigger('submit')
  await flushPromises()
  expect(create).toHaveBeenCalledWith('o', expect.objectContaining({ title: 'Tata Surya', author: '', format: 'ebook', file_id: 'pdf1', copies: 0, published: true }))
  expect(w.emitted('saved')?.[0]).toEqual([book])
  expect(w.emitted('done')).toBeUndefined()
  expect(w.get('.banner.ok').text()).toContain('Buku terbit')
  expect(w.get('.state').text()).toBe('Terbit')
  await w.findAll('.banner.ok button').find(b => b.text() === 'Tambah buku lain')!.trigger('click')
  expect(w.find('.banner.ok').exists()).toBe(false)
  expect((w.get('input[id$="-t"]').element as HTMLInputElement).value).toBe('')
})

it('shows server field errors and steps physical copies between 1 and 99', async () => {
  vi.spyOn(fileApi, 'list').mockResolvedValue({ used_bytes: 0, quota_bytes: 1, total: 0, items: [] })
  const create = vi.spyOn(libraryApi, 'create').mockRejectedValue(new ApiError(400, { code: 'validation_failed', message: 'x', fields: { title: 'Judul 1–200 karakter' } }))
  const w = mount(BookForm, { props: { orgId: 'o', userId: 'u' } })
  await flushPromises()
  expect(w.text()).toContain('Belum ada file PDF di Berkas')
  await w.get('input[value="buku"]').setValue(true)
  await flushPromises()
  const dec = w.get('[aria-label="Kurangi eksemplar"]')
  expect(dec.attributes('disabled')).toBeDefined()
  await w.get('[aria-label="Tambah eksemplar"]').trigger('click')
  expect(w.get('output').text()).toBe('2')
  expect(w.get('.ok').text()).toBe('2 dari 2 eksemplar tersedia')
  await w.get('input[id$="-t"]').setValue('Atlas')
  await w.get('form').trigger('submit')
  await flushPromises()
  expect(create).toHaveBeenCalledWith('o', expect.objectContaining({ format: 'buku', copies: 2, file_id: null }))
  expect(w.get('.field-error').text()).toBe('Judul 1–200 karakter')
})

import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, expect, it, vi } from 'vitest'
import LibraryManager from './LibraryManager.vue'
import { ApiError } from '@/api/client'
import { fileApi } from '@/api/files'
import { libraryApi } from '@/api/library'
import type { Book } from '@/api/types'

const draft: Book = {
  id: 'b1', title: 'Atlas', author: 'Rina', shelf: 'referensi', level: 'SMP', format: 'buku', copies: 2,
  file_id: null, cover: 'sejarah', published: false, sample: false, position: 0, updated_at: '2026-01-01T00:00:00Z',
  available: 2, my_loan: null,
}

afterEach(() => vi.restoreAllMocks())

function setup() {
  const list = vi.spyOn(libraryApi, 'list').mockResolvedValue({ items: [draft], next_cursor: null })
  vi.spyOn(fileApi, 'list').mockResolvedValue({
    used_bytes: 0, quota_bytes: 1,
    items: [
      { id: 'pdf1', name: 'tata.pdf', kind: 'pdf', content_type: 'application/pdf', size: 1, uploaded_by: 'u', created_at: '', position: 0 },
      { id: 'mp3', name: 'lagu.mp3', kind: 'audio', content_type: 'audio/mpeg', size: 1, uploaded_by: 'u', created_at: '', position: 0 },
    ],
  })
  return { wrapper: mount(LibraryManager, { props: { orgId: 'o' } }), list }
}

it('lists drafts for staff and offers only matching files for e-books', async () => {
  const { wrapper, list } = setup()
  await flushPromises()
  expect(list).toHaveBeenCalledWith('o', { drafts: true, cursor: null })
  expect(wrapper.text()).toContain('Draf')

  await wrapper.get('.head button').trigger('click')
  await flushPromises()
  await wrapper.get('select[id$="-f"]').setValue('ebook')
  await flushPromises()
  const options = wrapper.get('select[id$="-file"]').findAll('option').map((o) => o.text())
  expect(options).toEqual(['Pilih file', 'tata.pdf'])
})

it('creates a published e-book without copies and shows field errors from the server', async () => {
  const { wrapper } = setup()
  await flushPromises()
  const create = vi.spyOn(libraryApi, 'create')
    .mockRejectedValueOnce(new ApiError(400, { code: 'validation_failed', message: 'x', fields: { title: 'Judul 1–200 karakter' } }))
    .mockResolvedValueOnce({ ...draft, id: 'b2', title: 'Tata Surya', format: 'ebook', published: true })
  await wrapper.get('.head button').trigger('click')
  await flushPromises()
  await wrapper.get('select[id$="-f"]').setValue('ebook')
  await flushPromises()
  await wrapper.get('select[id$="-file"]').setValue('pdf1')
  await wrapper.get('input[type=checkbox]').setValue(true)
  await wrapper.get('form').trigger('submit')
  await flushPromises()
  expect(wrapper.get('.field-error').text()).toBe('Judul 1–200 karakter')
  expect(wrapper.get('input[id$="-t"]').attributes('aria-invalid')).toBe('true')

  await wrapper.get('input[id$="-t"]').setValue('Tata Surya')
  await wrapper.get('input[id$="-a"]').setValue('Tim')
  await wrapper.get('form').trigger('submit')
  await flushPromises()
  expect(create).toHaveBeenLastCalledWith('o', expect.objectContaining({ title: 'Tata Surya', format: 'ebook', file_id: 'pdf1', copies: 0, published: true }))
  expect(wrapper.find('form').exists()).toBe(false)
  expect(wrapper.get('[role=status]').text()).toBe('Tata Surya terbit.')
})

it('edits and archives after confirmation', async () => {
  const { wrapper, list } = setup()
  await flushPromises()
  const update = vi.spyOn(libraryApi, 'update').mockResolvedValue({ ...draft, published: true })
  await wrapper.findAll('article button')[0]!.trigger('click')
  await flushPromises()
  expect((wrapper.get('input[id$="-t"]').element as HTMLInputElement).value).toBe('Atlas')
  await wrapper.get('form').trigger('submit')
  await flushPromises()
  expect(update).toHaveBeenCalledWith('o', 'b1', expect.objectContaining({ file_id: null, copies: 2 }))

  vi.spyOn(window, 'confirm').mockReturnValue(true)
  const archive = vi.spyOn(libraryApi, 'archive').mockResolvedValue()
  await wrapper.findAll('article button')[1]!.trigger('click')
  await flushPromises()
  expect(archive).toHaveBeenCalledWith('o', 'b1')
  expect(list).toHaveBeenCalledTimes(3)
})

it('previews an e-book in the reader', async () => {
  vi.spyOn(libraryApi, 'list').mockResolvedValue({ items: [{ ...draft, format: 'ebook', file_id: 'pdf1', copies: 0 }], next_cursor: null })
  const wrapper = mount(LibraryManager, { props: { orgId: 'o' }, global: { stubs: { FileReader: { props: ['file'], template: '<div class="reader">{{ file.id }}</div>' } } } })
  await flushPromises()
  await wrapper.get('article button.btn-primary').trigger('click')
  expect(wrapper.get('.reader').text()).toBe('pdf1')
})

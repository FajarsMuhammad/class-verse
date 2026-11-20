import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, expect, it, vi } from 'vitest'
import LibraryManager from './LibraryManager.vue'
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
  const files = vi.spyOn(fileApi, 'list').mockResolvedValue({
    used_bytes: 0, quota_bytes: 1, total: 2,
    items: [
      { id: 'pdf1', name: 'tata.pdf', kind: 'pdf', content_type: 'application/pdf', size: 1, uploaded_by: 'u', created_at: '', position: 0 },
      { id: 'mp3', name: 'lagu.mp3', kind: 'audio', content_type: 'audio/mpeg', size: 1, uploaded_by: 'u', created_at: '', position: 0 },
    ],
  })
  return { wrapper: mount(LibraryManager, { props: { orgId: 'o', userId: 'u' } }), list, files }
}

it('lists drafts for staff and opens the Tambah buku page in place of the catalog', async () => {
  const { wrapper, list } = setup()
  await flushPromises()
  expect(list).toHaveBeenCalledWith('o', expect.objectContaining({ drafts: true, cursor: null, counts: true }))
  expect(wrapper.text()).toContain('Draf')
  await wrapper.get('.head button').trigger('click')
  await flushPromises()
  expect(wrapper.find('.book-form').exists()).toBe(true)
  expect(wrapper.find('.toolbar').exists()).toBe(false)
  await wrapper.findAll('button').find(b => b.text() === 'Batal')!.trigger('click')
  expect(wrapper.find('.toolbar').exists()).toBe(true)
})

it('edits and archives after confirmation', async () => {
  const { wrapper, list } = setup()
  await flushPromises()
  const update = vi.spyOn(libraryApi, 'update').mockResolvedValue({ ...draft, published: true })
  const button = (text: string) => wrapper.findAll('article button').find(b => b.text().startsWith(text))!
  await button('Ubah').trigger('click')
  await flushPromises()
  expect((wrapper.get('input[id$="-t"]').element as HTMLInputElement).value).toBe('Atlas')
  await wrapper.findAll('button').find(b => b.text() === 'Simpan sebagai draf')!.trigger('click')
  await flushPromises()
  expect(update).toHaveBeenCalledWith('o', 'b1', expect.objectContaining({ file_id: null, copies: 2, published: false }))
  expect(wrapper.find('.book-form').exists()).toBe(false)

  vi.spyOn(window, 'confirm').mockReturnValue(true)
  const archive = vi.spyOn(libraryApi, 'archive').mockResolvedValue()
  await button('Arsipkan').trigger('click')
  await flushPromises()
  expect(archive).toHaveBeenCalledWith('o', 'b1')
  expect(list).toHaveBeenCalledTimes(3)
})

it('previews an e-book in the reader', async () => {
  vi.spyOn(libraryApi, 'list').mockResolvedValue({ items: [{ ...draft, format: 'ebook', file_id: 'pdf1', copies: 0 }], next_cursor: null })
  const wrapper = mount(LibraryManager, { props: { orgId: 'o', userId: 'u' }, global: { stubs: { FileReader: { props: ['file'], template: '<div class="reader">{{ file.id }}</div>' } } } })
  await flushPromises()
  await wrapper.get('article button.btn-primary').trigger('click')
  expect(wrapper.get('.reader').text()).toBe('pdf1')
})

it('publishes a draft from its card and sends catalog filters to the server', async () => {
  const { wrapper, list } = setup()
  await flushPromises()
  const update = vi.spyOn(libraryApi, 'update').mockResolvedValue({ ...draft, published: true })
  await wrapper.findAll('article button').find(b => b.text().startsWith('Terbitkan'))!.trigger('click')
  await flushPromises()
  expect(update).toHaveBeenCalledWith('o', draft.id, expect.objectContaining({ published: true }))
  await wrapper.get('.toolbar select').setValue('ebook')
  await flushPromises()
  expect(list).toHaveBeenLastCalledWith('o', expect.objectContaining({ format: 'ebook', counts: true }))
})

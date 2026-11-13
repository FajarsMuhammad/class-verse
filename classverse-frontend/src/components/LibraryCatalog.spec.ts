import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import LibraryCatalog from './LibraryCatalog.vue'
import { libraryApi } from '@/api/library'
import type { Book, BookList } from '@/api/types'

const book = (title: string, extra: Partial<Book> = {}): Book => ({
  id: title, title, author: 'Tim', shelf: 'sains', level: 'SD', format: 'ebook', copies: 0,
  file_id: `file-${title}`, cover: 'ipa', published: true, sample: false, position: 0,
  updated_at: '2026-01-01T00:00:00Z', available: 0, my_loan: null, ...extra,
})
const page = (items: Book[], next: string | null = null): BookList => ({ items, next_cursor: next })

beforeEach(() => {
  vi.useFakeTimers()
  vi.spyOn(libraryApi, 'loans').mockResolvedValue({ items: [], max_active: 5 })
})
afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

function mountCatalog(shelf = '') {
  return mount(LibraryCatalog, {
    props: { orgId: 'o', shelf: shelf as never, 'onUpdate:shelf': (v: string) => wrapper.setProps({ shelf: v as never }) },
    global: { stubs: { FileReader: { props: ['file'], template: '<div class="reader">{{ file.name }}:{{ file.kind }}:{{ file.position }}</div>' } } },
  })
}
let wrapper: ReturnType<typeof mountCatalog>

it('filters by the shelf chosen on the map and toggles it from the tabs', async () => {
  const list = vi.spyOn(libraryApi, 'list').mockResolvedValue(page([book('Tata Surya')]))
  wrapper = mountCatalog('sains')
  await flushPromises()
  expect(list).toHaveBeenLastCalledWith('o', { shelf: 'sains', q: '', cursor: null })
  expect(wrapper.text()).toContain('Rak Sains')
  expect(wrapper.get('button[aria-pressed="true"]').text()).toBe('Sains')

  await wrapper.get('button[aria-pressed="true"]').trigger('click')
  await flushPromises()
  expect(wrapper.props('shelf')).toBe('')
  expect(list).toHaveBeenLastCalledWith('o', { shelf: '', q: '', cursor: null })
  expect(wrapper.text()).toContain('Semua rak')
})

it('debounces search, ignores stale responses, and pages with the cursor', async () => {
  let resolveSlow!: (v: BookList) => void
  const list = vi.spyOn(libraryApi, 'list')
    .mockResolvedValueOnce(page([book('A')], 'c1'))
    .mockReturnValueOnce(new Promise((r) => (resolveSlow = r)))
    .mockResolvedValueOnce(page([book('Bumi')]))
  wrapper = mountCatalog()
  await flushPromises()

  await wrapper.get('input[type=search]').setValue('bu')
  vi.advanceTimersByTime(300)
  await wrapper.get('input[type=search]').setValue('bumi')
  vi.advanceTimersByTime(300)
  await flushPromises()
  resolveSlow(page([book('Stale')]))
  await flushPromises()
  expect(wrapper.text()).toContain('Bumi')
  expect(wrapper.text()).not.toContain('Stale')
  expect(list).toHaveBeenCalledTimes(3)

  list.mockReset()
  list.mockResolvedValueOnce(page([book('A')], 'c1')).mockResolvedValueOnce(page([book('B')]))
  await wrapper.get('input[type=search]').setValue('')
  vi.advanceTimersByTime(300)
  await flushPromises()
  await wrapper.get('button.btn-secondary').trigger('click')
  await flushPromises()
  expect(list).toHaveBeenLastCalledWith('o', { shelf: '', q: '', cursor: 'c1' })
  expect(wrapper.findAll('article').map((a) => a.attributes('aria-label'))).toEqual(['A', 'B'])
  expect(wrapper.find('button.btn-secondary').exists()).toBe(false)
})

it('opens the in-app reader at the saved position; physical books offer Pinjam instead', async () => {
  vi.spyOn(libraryApi, 'list').mockResolvedValue(page([
    book('Lagu', { format: 'audio', position: 30 }),
    book('Atlas', { format: 'buku', file_id: null, copies: 2, available: 2 }),
  ]))
  wrapper = mountCatalog()
  await flushPromises()
  const [audio, physical] = wrapper.findAll('article')
  expect(physical!.text()).toContain('2 dari 2 tersedia')
  expect(physical!.get('button').text()).toContain('Pinjam')
  expect(audio!.get('button').text()).toContain('Lanjutkan')
  await audio!.get('button').trigger('click')
  expect(wrapper.get('.reader').text()).toBe('Lagu:audio:30')
})

it('shows an empty state and a retry on errors', async () => {
  const list = vi.spyOn(libraryApi, 'list').mockRejectedValueOnce(new TypeError('offline')).mockResolvedValue(page([]))
  wrapper = mountCatalog()
  await flushPromises()
  expect(wrapper.get('[role=alert]').text()).toContain('Coba lagi')
  await wrapper.get('[role=alert] button').trigger('click')
  await flushPromises()
  expect(list).toHaveBeenCalledTimes(2)
  expect(wrapper.text()).toContain('Belum ada buku di katalog.')
})

it('borrows a physical book, lists it in Pinjamanku with a WIB due date, and returns it', async () => {
  const due = '2026-11-26T17:30:00Z' // 27 Nov 00:30 WIB: the label must not show 26 Nov
  const physical = book('Laskar Pelangi', { format: 'buku', file_id: null, copies: 2, available: 1 })
  const lent = { ...physical, available: 0, my_loan: { id: 'loan1', due_at: due } }
  const list = vi.spyOn(libraryApi, 'list').mockResolvedValueOnce(page([physical])).mockResolvedValue(page([lent]))
  const loan = { id: 'loan1', book_id: physical.id, title: physical.title, author: 'Tim', cover: 'ipa', borrowed_at: due, due_at: due, returned_at: null }
  const loans = vi.mocked(libraryApi.loans).mockResolvedValueOnce({ items: [], max_active: 5 }).mockResolvedValue({ items: [loan], max_active: 5 })
  const borrow = vi.spyOn(libraryApi, 'borrow').mockResolvedValue({ id: 'loan1', due_at: due })
  wrapper = mountCatalog()
  await flushPromises()
  expect(wrapper.text()).toContain('1 dari 2 tersedia')

  await wrapper.get('article button.btn-primary').trigger('click')
  await flushPromises()
  expect(borrow).toHaveBeenCalledWith('o', 'Laskar Pelangi')
  expect(list).toHaveBeenCalledTimes(2)
  expect(wrapper.text()).toContain('Pinjamanku (1/5)')
  expect(wrapper.text()).toContain('27 Nov 2026')
  expect(wrapper.text()).toContain('Laskar Pelangi dipinjam.')

  const giveBack = vi.spyOn(libraryApi, 'giveBack').mockResolvedValue()
  loans.mockResolvedValue({ items: [], max_active: 5 })
  list.mockResolvedValue(page([physical]))
  await wrapper.get('.loans button').trigger('click')
  await flushPromises()
  expect(giveBack).toHaveBeenCalledWith('o', 'loan1')
  expect(wrapper.find('.loans').exists()).toBe(false)
  expect(wrapper.text()).toContain('1 dari 2 tersedia')
})

it('shows why a borrow was refused and hides Pinjam when no copy is free', async () => {
  const free = book('Bebas', { format: 'buku', file_id: null, copies: 1, available: 1 })
  const out = book('Habis', { format: 'buku', file_id: null, copies: 1, available: 0 })
  vi.spyOn(libraryApi, 'list').mockResolvedValue(page([free, out]))
  const { ApiError } = await import('@/api/client')
  vi.spyOn(libraryApi, 'borrow').mockRejectedValue(new ApiError(409, { code: 'loan_limit', message: 'Kamu sudah meminjam 5 buku. Kembalikan satu dulu.' }))
  wrapper = mountCatalog()
  await flushPromises()
  const cards = wrapper.findAll('article')
  expect(cards[1]!.text()).toContain('Sedang dipinjam')
  expect(cards[1]!.find('button').exists()).toBe(false)
  await cards[0]!.get('button').trigger('click')
  await flushPromises()
  expect(wrapper.get('[role=alert]').text()).toContain('Kamu sudah meminjam 5 buku')
})

import { flushPromises, mount } from '@vue/test-utils'
import { expect, it, vi } from 'vitest'
import PagedSelect from './PagedSelect.vue'
it('loads only requested page and preserves selected option', async () => {
  const load = vi.fn<(offset: number) => Promise<{ items: { value: string; label: string }[]; more: boolean }>>()
    .mockResolvedValueOnce({ items: [{ value: 'a', label: 'Pertama' }], more: true })
    .mockResolvedValueOnce({ items: [{ value: 'b', label: 'Kedua' }], more: false })
  const w = mount(PagedSelect, { props: { label: 'Berkas', modelValue: 'a', load } })
  await flushPromises()
  expect(load).toHaveBeenCalledTimes(1)
  await w.findAll('button')[1]!.trigger('click')
  await flushPromises()
  expect(load).toHaveBeenLastCalledWith(20)
  expect(w.get('select').element).toHaveProperty('value', 'a')
  expect(w.text()).toContain('Pertama')
})

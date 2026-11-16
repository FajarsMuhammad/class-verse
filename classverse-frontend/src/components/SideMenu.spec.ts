import { mount } from '@vue/test-utils'
import { expect, it } from 'vitest'
import { h, nextTick } from 'vue'
import SideMenu, { type SideItem } from './SideMenu.vue'

const items: SideItem[] = [
  { key: 'learn', label: 'Belajar', icon: 'M0 0' },
  { key: 'chat', label: 'Chat', icon: 'M0 0', keep: true, dot: true },
  { key: 'tool', label: 'Kelas', icon: 'M0 0', wide: true },
]
const make = (active: string | null) => mount(SideMenu, {
  props: { items, label: 'Menu ruang', active, 'onUpdate:active': (v: string | null) => w.setProps({ active: v }) },
  slots: { learn: () => h('p', 'isi belajar'), chat: () => h('input', { class: 'draft' }), tool: () => h('p', { class: 'tool' }, 'alat') },
  attachTo: document.body,
})
let w: ReturnType<typeof make>

it('rail stays visible; panel collapses and reopens with focus on its title then button', async () => {
  w = make('learn')
  expect(w.text()).toContain('isi belajar')
  await w.get('[aria-label="Ciutkan menu samping"]').trigger('click'); await nextTick(); await nextTick()
  expect(w.props('active')).toBe(null)
  expect(w.get('.panel').isVisible()).toBe(false)
  expect(w.findAll('.rail-button')).toHaveLength(3)
  expect(document.activeElement).toBe(w.get('[data-item="learn"]').element)
  await w.get('[data-item="tool"]').trigger('click'); await nextTick(); await nextTick()
  expect(w.get('.side-menu').classes()).toContain('wide')
  expect(document.activeElement?.tagName).toBe('H2')
  expect(w.find('.tool').exists()).toBe(true)
  w.unmount()
})
it('kept items stay mounted while hidden; others mount only when open; Escape collapses', async () => {
  w = make('chat')
  ;(w.get('.draft').element as HTMLInputElement).value = 'halo'
  await w.get('[data-item="learn"]').trigger('click')
  expect((w.get('.draft').element as HTMLInputElement).value).toBe('halo')
  expect(w.find('.tool').exists()).toBe(false)
  expect(w.get('[data-item="chat"]').attributes('aria-label')).toBe('Chat (ada pesan baru)')
  await w.get('.side-menu').trigger('keydown', { key: 'Escape' })
  expect(w.props('active')).toBe(null)
  w.unmount()
})

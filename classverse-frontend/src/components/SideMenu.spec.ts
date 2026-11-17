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

it('supports workspace identity and account controls without adding header navigation', () => {
  const menu = mount(SideMenu, {
    props: { items, label: 'Menu ruang' },
    slots: { identity: '<span>Kelasverse</span>', account: '<button aria-label="Akun">Akun</button>' },
  })
  expect(menu.get('.rail-identity').text()).toBe('Kelasverse')
  expect(menu.get('.rail-account button').attributes('aria-label')).toBe('Akun')
  menu.unmount()
})

it('full-page workspace exposes Peta and returns without removing kept content', async () => {
  w = make('chat')
  await w.setProps({ fullPage: true })
  expect(w.get('.side-menu').classes()).toContain('full-page')
  expect(w.get('[data-item="map"]').text()).toBe('Peta')
  expect(w.get('[aria-label="Kembali ke peta"]').text()).toContain('Kembali ke peta')
  await w.get('[data-item="map"]').trigger('click')
  expect(w.props('active')).toBe(null)
  expect(w.find('.draft').exists()).toBe(true)
  expect(w.get('[data-item="map"]').attributes('aria-pressed')).toBe('true')
  w.unmount()
})

it('companion panels keep the map beside them within a full-page workspace', async () => {
  w = make('chat')
  await w.setProps({ fullPage: true, items: items.map(item => ({ ...item, companion: item.key !== 'tool' })) })
  expect(w.get('.side-menu').classes()).not.toContain('full-page')
  expect(w.find('[data-item="map"]').exists()).toBe(true)
  expect(w.find('[aria-label="Ciutkan menu samping"]').exists()).toBe(true)
  await w.get('[data-item="tool"]').trigger('click')
  expect(w.get('.side-menu').classes()).toContain('full-page')
  expect(w.find('[aria-label="Kembali ke peta"]').exists()).toBe(true)
  await w.get('[data-item="learn"]').trigger('click')
  expect(w.get('.side-menu').classes()).not.toContain('full-page')
  w.unmount()
})

it('Peta owns its companion panel without a duplicate rail item', async () => {
  w = make('tool')
  await w.setProps({ fullPage: true, items: [{ key: 'map', label: 'Peta', icon: 'M0 0', companion: true }, ...items] })
  expect(w.findAll('[data-item="map"]')).toHaveLength(1)
  await w.get('[aria-label="Kembali ke peta"]').trigger('click')
  expect(w.props('active')).toBe('map')
  expect(w.get('.side-menu').classes()).not.toContain('full-page')
  await w.get('[aria-label="Ciutkan menu samping"]').trigger('click')
  expect(w.props('active')).toBe(null)
  await w.get('[data-item="map"]').trigger('click')
  expect(w.props('active')).toBe('map')
  w.unmount()
})

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

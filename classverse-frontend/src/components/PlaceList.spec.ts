import { mount } from '@vue/test-utils'
import { expect, it } from 'vitest'
import PlaceList from './PlaceList.vue'
import type { Building } from '@/realtime/mapdata'

const base = { roof: '', quiet: false, x: 0, y: 0, w: 10, h: 10, doorX: 0, doorY: 0 }
const places: Building[] = [
  { ...base, key: 'sains', name: 'Menara Sains', description: 'IPA', staffOnly: false },
  { ...base, key: 'ruang-guru', name: 'Ruang Guru', description: 'Pengajar', staffOnly: true },
]

it('stays closed until asked, so the map keeps its space', async () => {
  const w = mount(PlaceList, { props: { places, counts: [] } })
  expect(w.find('.sheet').exists()).toBe(false)
  expect(w.get('.trigger').attributes('aria-expanded')).toBe('false')
  await w.get('.trigger').trigger('click')
  expect(w.get('.sheet').isVisible()).toBe(true)
  await w.get('.places').trigger('keydown.esc')
  expect(w.find('.sheet').exists()).toBe(false)
})

it('lists every place with live counts, marks the selection, emits the key and closes', async () => {
  const w = mount(PlaceList, { props: { places, selected: 'sains', counts: [{ building_key: 'sains', people: 3, live_rooms: 1 }] } })
  await w.get('.trigger').trigger('click')
  const rows = w.findAll('.sheet button')
  expect(rows).toHaveLength(2)
  expect(rows[0]!.text()).toContain('3 orang · 1 live')
  expect(rows[0]!.attributes('aria-pressed')).toBe('true')
  expect(rows[1]!.text()).toContain('Khusus pengajar')
  expect(rows[1]!.text()).not.toContain('orang')
  await rows[1]!.trigger('click')
  expect(w.emitted('select')).toEqual([['ruang-guru']])
  expect(w.find('.sheet').exists()).toBe(false)
})

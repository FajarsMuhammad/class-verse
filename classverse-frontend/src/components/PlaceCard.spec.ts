import { mount } from '@vue/test-utils'
import { expect, it } from 'vitest'
import PlaceCard from './PlaceCard.vue'
import type { Building } from '@/realtime/mapdata'

const building: Building = { key: 'ruang-guru', name: 'Ruang Guru', description: 'Pengajar', roof: '',
  staffOnly: true, quiet: false, x: 0, y: 0, w: 100, h: 100, doorX: 50, doorY: 100 }

it('blocks staff-only actions for participants without inventing occupancy', () => {
  const w = mount(PlaceCard, { props: { building, atDoor: false, teacher: false, disabled: false } })
  expect(w.text()).toContain('Khusus pengajar dan admin')
  expect(w.get('[data-testid="place-action"]').attributes('disabled')).toBeDefined()
  expect(w.get('[data-testid="place-action"]').text()).toBe('Hanya untuk pengajar')
  expect(w.text()).not.toContain('0 orang')
})

it('walks to the door then offers room selection when nearby', async () => {
  const w = mount(PlaceCard, { props: { building, atDoor: false, teacher: true, disabled: false,
    occupancy: { building_key: 'ruang-guru', people: 3, live_rooms: 1 } } })
  expect(w.text()).toContain('3 orang')
  await w.get('[data-testid="place-action"]').trigger('click')
  expect(w.emitted('walk')).toHaveLength(1)
  await w.setProps({ atDoor: true })
  await w.get('[data-testid="place-action"]').trigger('click')
  expect(w.emitted('rooms')).toHaveLength(1)
  await w.get('[aria-label="Tutup detail tempat"]').trigger('click')
  expect(w.emitted('close')).toHaveLength(1)
})

it('offers modules for learning buildings but not for staff-only ones', async () => {
  const w = mount(PlaceCard, { props: { building: { ...building, key: 'sains', staffOnly: false }, atDoor: false, teacher: false, disabled: true } })
  // Reading modules does not need the room connection, so it stays enabled while travel is disabled.
  await w.get('[data-testid="place-modules"]').trigger('click')
  expect(w.emitted('modules')).toHaveLength(1)
  await w.setProps({ building })
  expect(w.find('[data-testid="place-modules"]').exists()).toBe(false)
})

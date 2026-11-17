import { mount } from '@vue/test-utils'
import { expect, it } from 'vitest'
import LearningPanel from './LearningPanel.vue'

const base = {
  name: 'Budi', avatar: 'default', teacher: false, roomName: 'Biologi XI-2', zoneName: '',
  seats: [{ key: '0', x: 10, y: 10 }], self: null, others: [], seatsDisabled: false,
}

it('shows campus guidance without seats, and seats inside a room', async () => {
  const w = mount(LearningPanel, { props: { ...base, campus: true } })
  expect(w.text()).toContain('Jelajahi kampus')
  expect(w.find('[aria-label="Tempat duduk"]').exists()).toBe(false)
  await w.setProps({ campus: false })
  expect(w.text()).toContain('Biologi XI-2')
  expect(w.text()).toContain('Area bersama')
  await w.get('[aria-label="Kursi 1: kosong"]').trigger('click')
  expect(w.emitted('walk')?.[0]).toEqual([base.seats[0]])
})

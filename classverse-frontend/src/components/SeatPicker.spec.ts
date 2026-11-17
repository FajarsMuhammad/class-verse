import { mount } from '@vue/test-utils'
import { expect, it } from 'vitest'
import SeatPicker from './SeatPicker.vue'

it('walks to free seats and keeps occupied and current seats unavailable', async () => {
  const seats = [{ key: '0', x: 100, y: 100 }, { key: '1', x: 200, y: 100 }, { key: '2', x: 300, y: 100 }]
  const w = mount(SeatPicker, { props: { seats, self: { x: 100, y: 100 }, others: [{ x: 200, y: 100 }], disabled: false } })
  expect(w.get('[aria-label="Kursi 1: kamu di sini"]').attributes('disabled')).toBeDefined()
  expect(w.get('[aria-label="Kursi 2: terisi"]').attributes('disabled')).toBeDefined()
  await w.get('[aria-label="Kursi 3: kosong"]').trigger('click')
  expect(w.emitted('walk')?.[0]).toEqual([seats[2]])
  await w.setProps({ disabled: true })
  expect(w.get('[aria-label="Kursi 3: kosong"]').attributes('disabled')).toBeDefined()
})

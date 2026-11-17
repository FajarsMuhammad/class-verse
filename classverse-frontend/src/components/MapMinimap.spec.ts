import { mount } from '@vue/test-utils'
import { expect, it } from 'vitest'
import { MapData } from '@/realtime/mapdata'
import MapMinimap from './MapMinimap.vue'

it('shows only the actual player position and clears it when unavailable', async () => {
  const map = new MapData({ width: 2, height: 2, tilewidth: 32, tileheight: 32,
    tilesets: [], layers: [{ name: 'collision', type: 'tilelayer', data: [0, 0, 0, 0] }] })
  const wrapper = mount(MapMinimap, { props: { map, position: { x: 16, y: 32 } } })
  expect(wrapper.attributes('viewBox')).toBe('0 0 64 64')
  expect(wrapper.get('circle').attributes('cx')).toBe('16')
  expect(wrapper.get('circle').attributes('cy')).toBe('32')
  await wrapper.setProps({ position: null })
  expect(wrapper.find('circle').exists()).toBe(false)
})

import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import MapZoom from './MapZoom.vue'

describe('map zoom controls', () => {
  it('shows default zoom and emits bounded changes', async () => {
    const wrapper = mount(MapZoom, { props: { modelValue: 0.5 } })
    expect(wrapper.text()).toContain('50%')
    await wrapper.get('[aria-label="Perbesar peta"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([0.6])
    await wrapper.setProps({ modelValue: 1 })
    expect(wrapper.get('[aria-label="Perbesar peta"]').attributes('disabled')).toBeDefined()
    await wrapper.setProps({ modelValue: 0.3 })
    expect(wrapper.get('[aria-label="Perkecil peta"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[aria-label="Reset zoom peta"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([0.5])
  })
})

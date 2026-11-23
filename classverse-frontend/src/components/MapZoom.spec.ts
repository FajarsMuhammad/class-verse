import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import MapZoom from './MapZoom.vue'
import { CONTENT, ZOOM_DEFAULT, ZOOM_MAX, ZOOM_MIN, cameraZoom, clampZoom } from '@/game/mapZoom'
import world from '@/assets/world/world.json'

describe('map zoom controls', () => {
  it('starts on the whole campus and emits bounded changes', async () => {
    expect(ZOOM_DEFAULT).toBe(1)
    const wrapper = mount(MapZoom, { props: { modelValue: ZOOM_DEFAULT } })
    expect(wrapper.text()).toContain('100%')
    await wrapper.get('[aria-label="Perbesar peta"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([1.1])
    await wrapper.setProps({ modelValue: ZOOM_MAX })
    expect(wrapper.get('[aria-label="Perbesar peta"]').attributes('disabled')).toBeDefined()
    await wrapper.setProps({ modelValue: ZOOM_MIN })
    expect(wrapper.get('[aria-label="Perkecil peta"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[aria-label="Reset zoom peta"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([ZOOM_DEFAULT])
  })

  it('clamps and drops float noise', () => {
    expect(clampZoom(1 + 0.1)).toBe(1.1)
    expect(clampZoom(5)).toBe(ZOOM_MAX)
    expect(clampZoom(0)).toBe(ZOOM_MIN)
  })

  it('fits every building into the frame at the default zoom', () => {
    for (const p of world.places) {
      expect(p.sprite_pos.x).toBeGreaterThanOrEqual(CONTENT.x)
      expect(p.label_anchor.y).toBeGreaterThan(CONTENT.y)
      expect(p.sprite_pos.x + p.size.w).toBeLessThanOrEqual(CONTENT.x + CONTENT.w)
      expect(p.sprite_pos.y + p.size.h).toBeLessThanOrEqual(CONTENT.y + CONTENT.h)
    }
    // Desktop frames; on phones the 0.3 camera floor wins and the campus pans.
    for (const [w, h] of [[1100, 640], [1512, 860]] as const) {
      const z = cameraZoom(ZOOM_DEFAULT, w, h)
      expect(CONTENT.w * z).toBeLessThanOrEqual(w + 0.5)
      expect(CONTENT.h * z).toBeLessThanOrEqual(h + 0.5)
    }
  })
})

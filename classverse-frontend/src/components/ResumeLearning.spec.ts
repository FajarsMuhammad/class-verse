import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, expect, it, vi } from 'vitest'
import { moduleApi } from '@/api/modules'
import { ApiError } from '@/api/client'
import ResumeLearning from './ResumeLearning.vue'

beforeEach(() => vi.restoreAllMocks())
const resume = {
  module_id: 'm', building_key: 'sains', level: 'SMP' as const, title: 'Tata Surya',
  done: 1, total: 3, next_title: 'Planet',
}
const global = { stubs: { BuildingModules: { name: 'BuildingModules', props: ['initialLevel', 'moduleId', 'buildingKey'], template: '<button class="close" @click="$emit(\'close\')">Tutup</button>' } } }

it('uses server progress and opens the matching building, level and module', async () => {
  const get = vi.spyOn(moduleApi, 'resume').mockResolvedValueOnce({ resume }).mockResolvedValue({ resume: null })
  const w = mount(ResumeLearning, { props: { orgId: 'o' }, global })
  await flushPromises()
  expect(w.text()).toContain('1 dari 3 selesai')
  await w.get('.btn-primary').trigger('click')
  expect(w.getComponent({ name: 'BuildingModules' }).props()).toMatchObject({ initialLevel: 'SMP', moduleId: 'm', buildingKey: 'sains' })
  await w.get('.close').trigger('click')
  await flushPromises()
  expect(get).toHaveBeenCalledTimes(2)
  expect(w.text()).toContain('Belum ada modul untuk dilanjutkan')
})

it('shows errors rather than fabricated progress, retries and refreshes after another dialog closes', async () => {
  const get = vi.spyOn(moduleApi, 'resume').mockRejectedValueOnce(new ApiError(500, { code: 'internal_error', message: 'x' }))
    .mockResolvedValue({ resume })
  const w = mount(ResumeLearning, { props: { orgId: 'o', refreshKey: 0 }, global })
  await flushPromises()
  expect(w.get('[role=alert]').text()).toContain('Terjadi kesalahan')
  expect(w.find('.btn-primary').exists()).toBe(false)
  await w.get('.link').trigger('click')
  await flushPromises()
  expect(w.text()).toContain('Berikutnya: Planet')
  await w.setProps({ refreshKey: 1 })
  await flushPromises()
  expect(get).toHaveBeenCalledTimes(3)
})

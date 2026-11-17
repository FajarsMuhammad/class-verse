import { flushPromises, mount } from '@vue/test-utils'
import { expect, it } from 'vitest'
import { ref } from 'vue'
import TeacherTools from './TeacherTools.vue'
import type { RoomHandle } from '@/realtime/useRoom'

const room = { current: ref({ room_id: 'r', map_key: 'library' }), kicked: ref(false) } as unknown as RoomHandle
const stubs = {
  RoomTools: { props: ['tool', 'room'], emits: ['pinned'], template: '<div class="body">{{ tool }}<button @click="$emit(\'pinned\')">Pin</button></div>' },
  TeacherPanel: { template: '<div class="session">Kontrol sesi</div>' },
}
it('groups modules and teacher tools under one navigation, mounting only the selected tool', async () => {
  const w = mount(TeacherTools, { props: { orgId: 'o', userId: 'u', isAdmin: false, room, modelValue: 'modules' }, global: { stubs } })
  expect(w.get('[aria-current="page"]').text()).toBe('Modul belajar')
  expect(w.get('.body').text()).toContain('library-modules')
  expect(w.findAll('.body')).toHaveLength(1)
  await w.get('[data-tool="quizzes"]').trigger('click')
  expect(w.emitted('update:modelValue')).toEqual([['quizzes']])
  await w.setProps({ modelValue: 'quizzes' })
  expect(w.get('.body').text()).toContain('quizzes')
  expect(w.get('h3').text()).toBe('Kuis')
  await w.get('.body button').trigger('click')
  expect(w.emitted('pinned')).toHaveLength(1)
  await w.setProps({ modelValue: 'session' })
  expect(w.find('.body').exists()).toBe(false)
  expect(w.find('.session').exists()).toBe(true)
})
it('hides session controls on campus', async () => {
  const campus = { current: ref({ room_id: 'c', map_key: 'campus' }), kicked: ref(false) } as unknown as RoomHandle
  const w = mount(TeacherTools, { props: { orgId: 'o', userId: 'u', isAdmin: false, room: campus }, global: { stubs } })
  await flushPromises()
  expect(w.find('[data-tool="session"]').exists()).toBe(false)
  expect(w.get('[aria-current="page"]').text()).toBe('Modul belajar')
})

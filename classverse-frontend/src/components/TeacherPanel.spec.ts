import { mount } from '@vue/test-utils'
import { expect, it, vi } from 'vitest'
import { ref } from 'vue'
import TeacherPanel from './TeacherPanel.vue'
import type { RoomHandle } from '@/realtime/useRoom'

function handle(muted: string[], people = [{ id: 'a', name: 'Ani', role: 'participant' }, { id: 'b', name: 'Budi', role: 'participant' }]) {
  const muteAll = vi.fn<RoomHandle['muteAll']>()
  const room = {
    status: ref('open'), traveling: ref(false), current: ref({ room_id: 'r', room_name: 'Kelas 1A' }),
    chat: { allChatEnabled: false, locked: false }, chatVersion: ref(0), gather: vi.fn<RoomHandle['gather']>(), setLocked: vi.fn<RoomHandle['setLocked']>(),
    people: ref(people), mutedIds: ref(muted), moderate: vi.fn<RoomHandle['moderate']>(), muteAll, kick: vi.fn<RoomHandle['kick']>(), banned: ref([]), allow: vi.fn<RoomHandle['allow']>(),
  } as unknown as RoomHandle
  return { room, muteAll }
}
const mountPanel = (room: RoomHandle) => mount(TeacherPanel, { props: { room, page: true }, global: { stubs: { GuestPanel: true } } })
const btn = (w: ReturnType<typeof mountPanel>, label: string) => w.findAll('button').find((b) => b.text() === label)

it('mutes everyone, then offers to unmute once all participants are muted', async () => {
  const { room, muteAll } = handle([])
  const w = mountPanel(room)
  await btn(w, 'Bisukan semua')!.trigger('click')
  expect(muteAll).toHaveBeenLastCalledWith(true)
  const all = handle(['a', 'b'])
  const w2 = mountPanel(all.room)
  await btn(w2, 'Aktifkan semua')!.trigger('click')
  expect(all.muteAll).toHaveBeenLastCalledWith(false)
  expect(w2.text()).toContain('Yang masuk setelahnya tidak ikut dibisukan')
})

it('disables the button when nobody but staff is in the room', () => {
  const { room } = handle([], [{ id: 't', name: 'Guru', role: 'teacher' }])
  expect(btn(mountPanel(room), 'Bisukan semua')!.attributes('disabled')).toBeDefined()
})

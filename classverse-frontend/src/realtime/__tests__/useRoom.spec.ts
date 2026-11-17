import { mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { expect, it, vi } from 'vitest'

import type { ServerMessage } from '../messages'
import { useRoom, type RoomHandle } from '../useRoom'

let push: (msg: ServerMessage) => void = () => {}
let seq = 0
vi.mock('../client', () => ({
  roomSocketURL: () => 'ws://test',
  RealtimeClient: class {
    constructor(o: { onStatus: (s: string) => void; onMessage: (m: ServerMessage) => void }) {
      push = (m) => o.onMessage(m)
      o.onStatus('open')
    }
    start() {}
    stop() {}
    send() { return true }
    sendSeq() { return ++seq }
  },
}))
vi.mock('../mapdata', () => ({ fetchMap: () => Promise.resolve({ buildings: [] }) }))

it('keeps rejected emotes out of the map and chat error banners', async () => {
  let room!: RoomHandle
  mount(defineComponent({ setup() { room = useRoom(); return () => h('div') } }))
  await room.start('r', 'campus')
  push({ t: 'welcome', seq: 1, d: { self_id: 'me', room_id: 'r', map_key: 'campus', participants: [], all_chat_enabled: true } })
  expect(room.sendEmote('wave')).toBe(true)
  push({ t: 'error', seq: 2, d: { code: 'muted', message: 'Kamu dibisukan oleh pengajar', seq } })
  expect(room.sendEmote('hand')).toBe(true)
  push({ t: 'error', seq: 3, d: { code: 'rate_limited', message: 'Tunggu sebentar', seq } })
  expect(room.error.value).toBe('')
  expect(room.chatError.value).toBe('')
  // Other errors still surface.
  push({ t: 'error', seq: 4, d: { code: 'forbidden', message: 'Ditolak' } })
  expect(room.error.value).toBe('Ditolak')
})

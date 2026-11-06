import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref, shallowRef } from 'vue'

import MobileRoom from './MobileRoom.vue'
import { ChatState } from '@/realtime/chat'
import { MapData, type TiledMap } from '@/realtime/mapdata'
import type { ServerMessage, WelcomeMsg } from '@/realtime/messages'
import type { RoomHandle } from '@/realtime/useRoom'
import { World } from '@/realtime/world'

vi.mock('@/api/rooms', () => ({
  roomApi: { list: vi.fn<() => Promise<unknown>>(() => Promise.resolve({ items: [], next_cursor: null })) },
}))

const classroom = new MapData({
  width: 10,
  height: 10,
  tilewidth: 32,
  tileheight: 32,
  tilesets: [],
  layers: [
    { name: 'collision', type: 'tilelayer', data: Array(100).fill(0) },
    {
      name: 'zones',
      type: 'objectgroup',
      objects: [
        {
          id: 1,
          name: 'Meja 3',
          x: 0,
          y: 0,
          width: 64,
          height: 64,
          properties: [{ name: 'key', value: 'meja-3' }],
        },
      ],
    },
  ],
} as TiledMap)

const p = (id: string, x: number, y: number) => ({
  id,
  name: id,
  avatar: 'default',
  role: 'participant',
  x,
  y,
  dir: 'down' as const,
})

function handle(welcome: WelcomeMsg, map: MapData): RoomHandle {
  const world = new World()
  const chat = new ChatState()
  const msg = { t: 'welcome', seq: 1, d: welcome } as ServerMessage
  world.apply(msg, 0)
  chat.apply(msg)
  return {
    status: ref('open'),
    map: shallowRef(map),
    world,
    online: ref(world.count),
    error: ref(''),
    current: shallowRef(welcome),
    buildings: ref([]),
    traveling: ref(false),
    travel: vi.fn<RoomHandle['travel']>(),
    chat,
    chatVersion: ref(chat.version),
    chatError: ref(''),
    sendChat: vi.fn<RoomHandle['sendChat']>(() => true),
    gather: vi.fn<RoomHandle['gather']>(),
    kick: vi.fn<RoomHandle['kick']>(),
    moderate: vi.fn<RoomHandle['moderate']>(),
    setLocked: vi.fn<RoomHandle['setLocked']>(),
    mutedIds: ref([]),
    allow: vi.fn<RoomHandle['allow']>(),
    banned: ref([]),
    kicked: ref(false),
    people: ref([]),
    rejoin: vi.fn<RoomHandle['rejoin']>(),
    sendMove: vi.fn<RoomHandle['sendMove']>(),
    seat: vi.fn<RoomHandle['seat']>(),
    start: vi.fn<RoomHandle['start']>(),
  } as RoomHandle
}

const inRoom: WelcomeMsg = {
  self_id: 'me',
  room_id: 'room-1',
  map_key: 'classroom',
  room_name: 'Biologi XI-2',
  participants: [p('me', 200, 200), p('Sari', 16, 16), p('Dimas', 300, 300)],
  all_chat_enabled: true,
}

beforeEach(() => setActivePinia(createPinia()))

describe('MobileRoom', () => {
  it('lists places with counts and seats on tap without moving locally', async () => {
    const room = handle(inRoom, classroom)
    const w = mount(MobileRoom, {
      props: { room, roomId: 'campus-1', schoolName: 'SMA 1', statusText: '' },
    })
    const buttons = w.findAll('.place')
    expect(buttons.map((b) => b.text())).toEqual(['Area umum · 2', 'Meja 3 · 1'])
    await buttons[1]!.trigger('click')
    expect(room.seat).toHaveBeenCalledWith('meja-3')
    expect(room.sendMove).not.toHaveBeenCalled()

    // The server confirms with zone; the chip and tablemates follow.
    room.chat.apply({ t: 'zone', seq: 2, d: { key: 'meja-3', name: 'Meja 3' } })
    room.chatVersion.value = room.chat.version
    await w.vm.$nextTick()
    expect(w.find('.place[aria-pressed="true"]').text()).toBe('Meja 3 · 2')
    expect(w.text()).toContain('Kamu di Meja 3')
    expect(w.text()).toContain('Sari')
    expect(w.find('canvas').exists()).toBe(false)
  })

  it('back button travels to the campus', async () => {
    const room = handle(inRoom, classroom)
    const w = mount(MobileRoom, {
      props: { room, roomId: 'campus-1', schoolName: 'SMA 1', statusText: '' },
    })
    await w.get('button[aria-label="Kembali ke peta sekolah"]').trigger('click')
    expect(room.travel).toHaveBeenCalledWith('campus-1')
  })

  it('shows the kicked notice instead of chat', async () => {
    const room = handle(inRoom, classroom)
    room.kicked.value = true
    const w = mount(MobileRoom, {
      props: { room, roomId: 'campus-1', schoolName: 'SMA 1', statusText: '' },
    })
    expect(w.get('[role="alert"]').text()).toContain('dikeluarkan')
    expect(w.find('.chat').exists()).toBe(false)
  })
})

import { describe, expect, it } from 'vitest'

import { ChatState, MAX_PER_TAB } from '../chat'
import type { ChatOutMsg, ServerMessage } from '../messages'

const welcome = (
  room: string,
  t: 'welcome' | 'traveled' = 'welcome',
  allChat = true,
): ServerMessage => ({
  t,
  seq: 1,
  d: {
    self_id: 'me',
    room_id: room,
    map_key: 'classroom',
    participants: [],
    all_chat_enabled: allChat,
  },
})
const chat = (
  from: string,
  scope: ChatOutMsg['scope'],
  text: string,
  id?: number,
): ServerMessage => ({
  t: 'chat',
  seq: 2,
  d: {
    id,
    from: { id: from, name: from, role: 'participant' },
    text,
    scope,
    at: '2026-01-01T00:00:00Z',
  },
})

describe('ChatState', () => {
  it('tracks personal mute and room lock from welcome and room_state', () => {
    const s = new ChatState()
    s.apply(welcome('r1'))
    expect(s.muted).toBe(false)
    s.apply({ t: 'room_state', seq: 2, d: { all_chat_enabled: true, locked: true, muted: true } })
    expect(s.muted).toBe(true)
    expect(s.locked).toBe(true)
    s.apply(welcome('r2', 'traveled'))
    expect(s.muted).toBe(false)
    expect(s.locked).toBe(false)
  })

  it('copies announcements into all tabs with bounded history and clears them on travel', () => {
    const s = new ChatState()
    s.apply(welcome('r1', 'welcome', false))
    for (let i = 0; i <= MAX_PER_TAB; i++)
      s.apply(chat('teacher', 'broadcast', `notice ${i}`, i + 1))
    for (const lines of Object.values(s.lines)) {
      expect(lines).toHaveLength(MAX_PER_TAB)
      expect(lines[0]).toMatchObject({ text: 'notice 1', broadcast: true })
    }
    s.apply(welcome('r2', 'traveled'))
    for (const lines of Object.values(s.lines)) expect(lines).toHaveLength(0)
  })

  it('sorts messages into tabs and marks own lines', () => {
    const s = new ChatState()
    s.apply(welcome('r1'))
    s.apply(chat('me', 'near', 'halo', 1))
    s.apply(chat('budi', 'zone', 'meja', 2))
    s.apply(chat('budi', 'room', 'semua'))
    expect(s.lines.near).toEqual([
      expect.objectContaining({ text: 'halo', mine: true, key: 'id-1' }),
    ])
    expect(s.lines.zone[0]).toMatchObject({ text: 'meja', mine: false })
    expect(s.lines.room).toHaveLength(1)
  })

  it('keeps at most MAX_PER_TAB lines', () => {
    const s = new ChatState()
    s.apply(welcome('r1'))
    for (let i = 0; i < MAX_PER_TAB + 5; i++) s.apply(chat('b', 'near', `m${i}`, i + 1))
    expect(s.lines.near).toHaveLength(MAX_PER_TAB)
    expect(s.lines.near[0]!.text).toBe('m5')
  })

  it('tracks zone and default tab', () => {
    const s = new ChatState()
    s.apply(welcome('r1'))
    expect(s.defaultTab()).toBe('near')
    s.apply({ t: 'zone', seq: 3, d: { key: 'meja-1', name: 'Meja 1' } })
    expect(s.defaultTab()).toBe('zone')
    expect(s.zone.name).toBe('Meja 1')
    s.apply({ t: 'zone', seq: 4, d: { key: '', name: '' } })
    expect(s.defaultTab()).toBe('near')
  })

  it('follows room_state and welcome for the all-room tab', () => {
    const s = new ChatState()
    s.apply(welcome('r1', 'welcome', false))
    expect(s.allChatEnabled).toBe(false)
    s.apply({ t: 'room_state', seq: 5, d: { all_chat_enabled: true } })
    expect(s.allChatEnabled).toBe(true)
  })

  it('clears history on travel but keeps it on reconnect to the same room', () => {
    const s = new ChatState()
    s.apply(welcome('r1'))
    s.apply(chat('b', 'room', 'satu', 1))
    s.apply(welcome('r1')) // reconnect
    expect(s.lines.room).toHaveLength(1)
    s.apply(welcome('r2', 'traveled'))
    expect(s.lines.room).toHaveLength(0)
  })
})

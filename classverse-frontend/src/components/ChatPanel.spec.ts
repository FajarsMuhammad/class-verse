import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import ChatPanel from './ChatPanel.vue'
import { ChatState } from '@/realtime/chat'

function state(allChat = true) {
  const s = new ChatState()
  s.apply({
    t: 'welcome',
    seq: 1,
    d: {
      self_id: 'me',
      room_id: 'r',
      map_key: 'classroom',
      participants: [],
      all_chat_enabled: allChat,
    },
  })
  return s
}

const mountPanel = (chat: ChatState, campus = false) =>
  mount(ChatPanel, { props: { chat, version: chat.version, error: '', disabled: false, campus } })

describe('ChatPanel', () => {
  it('disables every tab while muted, except on the campus', async () => {
    const chat = state()
    chat.apply({ t: 'room_state', seq: 2, d: { all_chat_enabled: true, muted: true } })
    const w = mountPanel(chat)
    expect(w.get('textarea').attributes('disabled')).toBeDefined()
    expect(w.get('textarea').attributes('placeholder')).toContain('dibisukan')
    expect(w.text()).toContain('membisukan')
  })

  it('offers broadcast only to teachers and allows it when room chat is disabled', async () => {
    const w = mountPanel(state(false))
    expect(w.findAll('[role=tab]').some((t) => t.text() === 'Pengumuman')).toBe(false)
    await w.setProps({ teacher: true })
    await w
      .findAll('[role=tab]')
      .find((t) => t.text() === 'Pengumuman')!
      .trigger('click')
    await w.get('textarea').setValue('Mulai kelas')
    await w.get('form').trigger('submit')
    expect(w.emitted('send')).toEqual([['Mulai kelas', 'broadcast']])
  })

  it('sends on Enter to the selected tab and keeps Shift+Enter as newline', async () => {
    const w = mountPanel(state())
    const input = w.get('textarea')
    await input.setValue('halo')
    await input.trigger('keydown', { key: 'Enter', shiftKey: true })
    expect(w.emitted('send')).toBeUndefined()
    await input.trigger('keydown', { key: 'Enter' })
    expect(w.emitted('send')).toEqual([['halo', 'near']])
    expect((input.element as HTMLTextAreaElement).value).toBe('')
  })

  it('shows the zone tab when inside a zone and switches to it', async () => {
    const chat = state()
    const w = mountPanel(chat)
    expect(w.findAll('[role=tab]').map((t) => t.text())).toEqual(['Dekat', 'Semua'])
    chat.apply({ t: 'zone', seq: 2, d: { key: 'meja-3', name: 'Meja 3' } })
    await w.setProps({ version: chat.version })
    expect(w.findAll('[role=tab]').map((t) => t.text())).toEqual(['Meja 3', 'Dekat', 'Semua'])
    expect(w.get('[aria-selected=true]').text()).toBe('Meja 3')
    expect(w.get('textarea').attributes('placeholder')).toContain('Meja 3')
  })

  it('disables the Semua input when the teacher turned it off, except on the campus', async () => {
    const w = mountPanel(state(false))
    await w
      .findAll('[role=tab]')
      .find((t) => t.text() === 'Semua')!
      .trigger('click')
    expect(w.get('textarea').attributes('disabled')).toBeDefined()
    expect(w.get('textarea').attributes('placeholder')).toContain('dimatikan')

    const campus = mountPanel(state(false), true)
    await campus
      .findAll('[role=tab]')
      .find((t) => t.text() === 'Semua')!
      .trigger('click')
    expect(campus.get('textarea').attributes('disabled')).toBeUndefined()
  })

  it('renders own and other messages in a live log', async () => {
    const chat = state()
    const w = mountPanel(chat)
    for (const [id, from] of [
      [1, 'me'],
      [2, 'budi'],
    ] as const) {
      chat.apply({
        t: 'chat',
        seq: 3,
        d: {
          id,
          from: { id: from, name: from, role: 'participant' },
          text: `pesan ${id}`,
          scope: 'near',
          at: '2026-01-01T08:00:00Z',
        },
      })
    }
    await w.setProps({ version: chat.version })
    expect(w.get('[role=log]').attributes('aria-live')).toBe('polite')
    const lines = w.findAll('.line')
    expect(lines).toHaveLength(2)
    expect(lines[0]!.classes()).toContain('mine')
    expect(lines[0]!.text()).toContain('Kamu')
    expect(lines[1]!.text()).toContain('budi')
  })

  it('blocks sending past 500 characters', async () => {
    const w = mountPanel(state())
    await w.get('textarea').setValue('a'.repeat(501))
    expect(w.get('button.send').attributes('disabled')).toBeDefined()
    expect(w.get('.count').text()).toContain('-1')
  })
})

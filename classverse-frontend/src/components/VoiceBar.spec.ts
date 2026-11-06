import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'

import VoiceBar from './VoiceBar.vue'
import { ChatState } from '@/realtime/chat'
import type { VoiceHandle, VoiceStatus } from '@/realtime/useVoice'

function voice(status: VoiceStatus = 'on'): VoiceHandle {
  return {
    status: ref(status),
    error: ref(''),
    micOn: ref(false),
    speaking: ref(new Set<string>()),
    needsAudioStart: ref(false),
    toggleMic: vi.fn<VoiceHandle['toggleMic']>(),
    startAudio: vi.fn<VoiceHandle['startAudio']>(),
  }
}

function chat(patch: Partial<ChatState> = {}) {
  return Object.assign(new ChatState(), patch)
}

const mountBar = (v: VoiceHandle, c: ChatState, teacher = false) =>
  mount(VoiceBar, {
    props: { voice: v, chat: c, version: 0, selfId: 'me', teacher, disabled: false },
  })

describe('VoiceBar', () => {
  it('starts with the mic off and says which channel you hear', async () => {
    const v = voice()
    const w = mountBar(v, chat({ zone: { key: 'meja-3', name: 'Meja 3' } }))
    const mic = w.get('button.mic')
    expect(mic.attributes('aria-pressed')).toBe('false')
    expect(mic.attributes('aria-label')).toBe('Nyalakan mikrofon')
    expect(w.text()).toContain('kamu mendengar Meja 3')
    await mic.trigger('click')
    expect(v.toggleMic).toHaveBeenCalled()
    expect(w.text()).not.toContain('Bicara ke semua ruang')
  })

  it('disables the mic while muted by the teacher', () => {
    const w = mountBar(voice(), chat({ muted: true }))
    expect(w.get('button.mic').attributes('disabled')).toBeDefined()
    expect(w.text()).toContain('dimatikan pengajar')
  })

  it('lets a teacher toggle voice broadcast', async () => {
    const w = mountBar(voice(), chat({ voiceBroadcast: ['me'] }), true)
    const toggle = w.get('[role="switch"]')
    expect(toggle.attributes('aria-checked')).toBe('true')
    await toggle.trigger('click')
    expect(w.emitted('broadcast')).toEqual([[false]])
  })

  it('shows unconfirmed eviction to staff instead of claiming success', () => {
    const w = mountBar(voice(), chat({ voicePending: ['user'] }), true)
    expect(w.text()).toContain('Pemutusan suara 1 peserta belum terkonfirmasi')
    expect(w.text()).toContain('Server mencoba ulang')
    expect(mountBar(voice(), chat({ voicePending: ['user'] })).text()).not.toContain(
      'belum terkonfirmasi',
    )
  })

  it('stays hidden when voice is not configured', () => {
    expect(mountBar(voice('unavailable'), chat()).find('.voice').exists()).toBe(false)
  })
})

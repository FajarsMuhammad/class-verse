import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, ref } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ChatState } from '../chat'
import { useVoice } from '../useVoice'
import type { WelcomeMsg } from '../messages'

const mocks = vi.hoisted(() => ({
  token: vi.fn<() => Promise<{ url: string; token: string; broadcast_token: string; identity: string }>>(),
  connects: vi.fn<() => Promise<void>>(),
  instances: [] as { disconnect: ReturnType<typeof vi.fn> }[],
}))
vi.mock('@/api/rooms', () => ({ roomApi: { voiceToken: mocks.token } }))
vi.mock('livekit-client', () => ({
  Room: class {
    canPlaybackAudio = true
    localParticipant = { setMicrophoneEnabled: vi.fn<(on: boolean) => Promise<void>>() }
    disconnect = vi.fn<() => Promise<void>>()
    constructor() {
      mocks.instances.push(this)
    }
    on() {
      return this
    }
    connect = mocks.connects
    startAudio = vi.fn<() => Promise<void>>()
  },
  RoomEvent: {},
  Track: { Kind: { Audio: 'audio' } },
}))

describe('useVoice admissions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.instances.length = 0
    mocks.token.mockResolvedValue({
      url: 'ws://app/voice',
      token: 'zone',
      broadcast_token: 'broadcast',
      identity: 'user:one',
    })
  })
  it('stays off in the quiet library instead of waiting for an identity', async () => {
    const chat = new ChatState()
    const current = ref({ room_id: 'room', map_key: 'library' } as WelcomeMsg)
    let voice!: ReturnType<typeof useVoice>
    const wrapper = mount(defineComponent({ setup() {
      voice = useVoice({ current, chat, chatVersion: ref(0) })
      return () => null
    } }))
    await flushPromises()
    expect(voice.status.value).toBe('off')
    expect(voice.error.value).toBe('')
    expect(mocks.token).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('does not reconnect when unrelated chat state changes', async () => {
    const chat = new ChatState()
    chat.voiceIdentity = 'user:one'
    const version = ref(0)
    const current = ref({ room_id: 'room', map_key: 'classroom' } as WelcomeMsg)
    const wrapper = mount(
      defineComponent({
        setup() {
          useVoice({ current, chat, chatVersion: version })
          return () => null
        },
      }),
    )
    await flushPromises()
    expect(mocks.token).toHaveBeenCalledTimes(1)
    expect(mocks.connects).toHaveBeenCalledTimes(2)
    version.value++
    await flushPromises()
    expect(mocks.token).toHaveBeenCalledTimes(1)
    chat.voiceIdentity = ''
    version.value++
    await flushPromises()
    expect(mocks.instances.every((room) => room.disconnect.mock.calls.length === 1)).toBe(true)
    expect(mocks.token).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })
})

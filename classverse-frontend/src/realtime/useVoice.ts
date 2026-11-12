import { onBeforeUnmount, ref, watch, type Ref } from 'vue'
import type { Room as LkRoom } from 'livekit-client'

import { errorMessage } from '@/api/client'
import { roomApi } from '@/api/rooms'
import type { ChatState } from './chat'
import type { WelcomeMsg } from './messages'

export type VoiceStatus = 'off' | 'connecting' | 'on' | 'unavailable' | 'error'

// LiveKit room grants enforce isolation. Never infer voice permissions from
// client positions: actor-issued identity changes after every revocation.
export function useVoice(opts: {
  current: Ref<WelcomeMsg | null>
  chat: ChatState
  chatVersion: Ref<number>
}) {
  const status = ref<VoiceStatus>('off')
  const error = ref('')
  const micOn = ref(false)
  const speaking = ref<Set<string>>(new Set())
  const needsAudioStart = ref(false)
  let rooms: LkRoom[] = []
  let generation = 0
  let wantedMic = false
  let retry: ReturnType<typeof setTimeout> | undefined
  let stopped = false
  const audio = new Map<string, HTMLMediaElement>()
  const active = new Map<LkRoom, string[]>()

  async function leave() {
    generation++
    clearTimeout(retry)
    const old = rooms
    rooms = []
    micOn.value = false
    needsAudioStart.value = false
    for (const el of audio.values()) el.remove()
    audio.clear()
    active.clear()
    speaking.value = new Set()
    await Promise.all(old.map((room) => room.disconnect()))
  }

  async function join(roomId: string, identity: string) {
    const gen = ++generation
    status.value = 'connecting'
    error.value = ''
    const pending: LkRoom[] = []
    try {
      const [{ Room, RoomEvent, Track }, token] = await Promise.all([
        import('livekit-client'),
        roomApi.voiceToken(roomId),
      ])
      if (gen !== generation) return
      if (token.identity !== identity) {
        scheduleRetry()
        return
      }
      for (const grant of [token.token, token.broadcast_token]) {
        const room = new Room({ adaptiveStream: false })
        pending.push(room)
        room
          .on(RoomEvent.TrackSubscribed, (track, pub, participant) => {
            if (gen !== generation || track.kind !== Track.Kind.Audio) return
            audio.get(pub.trackSid)?.remove()
            const el = track.attach()
            el.dataset.voice = participant.identity.split(':')[0]
            document.body.appendChild(el)
            audio.set(pub.trackSid, el)
          })
          .on(RoomEvent.TrackUnsubscribed, (track, pub) => {
            for (const el of track.detach()) el.remove()
            audio.get(pub.trackSid)?.remove()
            audio.delete(pub.trackSid)
          })
          .on(RoomEvent.ActiveSpeakersChanged, (speakers) => {
            if (gen !== generation) return
            active.set(
              room,
              speakers.map((p) => p.identity.split(':')[0]!),
            )
            speaking.value = new Set([...active.values()].flat())
          })
          .on(RoomEvent.AudioPlaybackStatusChanged, () => {
            needsAudioStart.value = pending.some((r) => !r.canPlaybackAudio)
          })
          .on(RoomEvent.Disconnected, () => {
            if (gen === generation && rooms.includes(room)) {
              void leave()
              status.value = 'error'
              error.value = 'Suara terputus. Mencoba menyambung kembali…'
              scheduleRetry()
            }
          })
        await room.connect(token.url, grant, { autoSubscribe: true })
        if (gen !== generation) {
          await Promise.all(pending.map((r) => r.disconnect()))
          return
        }
      }
      rooms = pending
      status.value = 'on'
      needsAudioStart.value = rooms.some((r) => !r.canPlaybackAudio)
      if (wantedMic && !opts.chat.muted) await setMic(true)
    } catch (e) {
      await Promise.all(pending.map((r) => r.disconnect()))
      if (gen !== generation) return
      const code = (e as { code?: string }).code
      status.value = code === 'voice_unavailable' ? 'unavailable' : 'error'
      error.value = code === 'voice_unavailable' ? '' : `Suara belum tersambung: ${errorMessage(e)}`
      if (code !== 'voice_unavailable' && code !== 'not_found') scheduleRetry()
    }
  }

  function scheduleRetry() {
    clearTimeout(retry)
    retry = setTimeout(() => {
      if (!stopped) void reconnect()
    }, 1500)
  }

  async function reconnect() {
    const cur = opts.current.value
    const identity = opts.chat.voiceIdentity
    const closing = leave()
    const gen = generation
    await closing
    if (gen !== generation) return
    // Campus and the library (quiet zone, chat only) never get a voice identity.
    if (stopped || !cur || cur.map_key === 'campus' || cur.map_key === 'library') {
      status.value = 'off'
      return
    }
    if (!identity) {
      status.value = 'connecting'
      error.value = 'Menunggu konfirmasi pemutusan suara sebelumnya…'
      return
    }
    await join(cur.room_id, identity)
  }

  async function setMic(on: boolean) {
    const gen = generation
    try {
      const broadcast = !!opts.chat.selfId && opts.chat.voiceBroadcast.includes(opts.chat.selfId)
      // One publisher only; broadcast listeners cannot publish in this room.
      const room = rooms[broadcast ? 1 : 0]
      if (!room) return
      await room.localParticipant.setMicrophoneEnabled(on)
      if (gen === generation) micOn.value = on
    } catch (e) {
      if (gen !== generation) return
      wantedMic = false
      micOn.value = false
      error.value =
        (e as Error).name === 'NotAllowedError'
          ? 'Izin mikrofon ditolak. Izinkan mikrofon di pengaturan browser.'
          : 'Mikrofon tidak dapat dinyalakan.'
    }
  }

  async function toggleMic() {
    wantedMic = !micOn.value
    error.value = ''
    await setMic(wantedMic)
  }
  async function startAudio() {
    try {
      await Promise.all(rooms.map((r) => r.startAudio()))
      needsAudioStart.value = rooms.some((r) => !r.canPlaybackAudio)
    } catch {
      error.value = 'Suara belum dapat diputar. Coba aktifkan suara lagi.'
    }
  }

  watch(
    () => {
      void opts.chatVersion.value
      return `${opts.current.value?.room_id ?? ''}|${opts.chat.voiceIdentity}`
    },
    (next, previous) => {
      if (next.split('|')[0] !== previous?.split('|')[0]) wantedMic = false
      if (opts.chat.muted) wantedMic = false
      void reconnect()
    },
    { immediate: true },
  )

  onBeforeUnmount(() => {
    stopped = true
    void leave()
  })
  return { status, error, micOn, speaking, needsAudioStart, toggleMic, startAudio }
}

export type VoiceHandle = ReturnType<typeof useVoice>

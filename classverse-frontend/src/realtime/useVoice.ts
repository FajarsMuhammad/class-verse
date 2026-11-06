import { onBeforeUnmount, ref, watch, type Ref } from 'vue'
import type { Room as LkRoom } from 'livekit-client'

import { errorMessage } from '@/api/client'
import { roomApi } from '@/api/rooms'
import type { ChatState } from './chat'
import type { MapData } from './mapdata'
import type { WelcomeMsg } from './messages'
import { audible } from './voice'
import type { World } from './world'

export type VoiceStatus = 'off' | 'connecting' | 'on' | 'unavailable' | 'error'

// Subscriptions follow positions from WS snapshots; resync this often.
const SYNC_MS = 250

// useVoice joins the LiveKit room of the current classroom/garden (never the
// campus) and keeps subscriptions to the user's voice channel (V9-03).
// livekit-client is loaded only when a room with voice is entered.
export function useVoice(opts: {
  current: Ref<WelcomeMsg | null>
  map: Ref<MapData | null>
  world: World
  chat: ChatState
  chatVersion: Ref<number>
}) {
  const status = ref<VoiceStatus>('off')
  const error = ref('')
  const micOn = ref(false)
  const speaking = ref<Set<string>>(new Set())
  const needsAudioStart = ref(false)
  let lk: LkRoom | null = null
  let timer: ReturnType<typeof setInterval> | undefined
  let generation = 0
  // Audio elements per remote track sid. The SDK may detach before emitting
  // TrackUnsubscribed, so elements are removed from this map, not detach().
  const audio = new Map<string, HTMLMediaElement>()

  function dropAudio() {
    for (const el of audio.values()) el.remove()
    audio.clear()
    speaking.value = new Set()
  }

  async function leave() {
    generation++
    clearInterval(timer)
    timer = undefined
    micOn.value = false
    needsAudioStart.value = false
    dropAudio()
    const old = lk
    lk = null
    await old?.disconnect()
  }

  async function join(roomId: string) {
    const gen = ++generation
    status.value = 'connecting'
    error.value = ''
    try {
      const [{ Room, RoomEvent, Track }, token] = await Promise.all([
        import('livekit-client'),
        roomApi.voiceToken(roomId),
      ])
      if (gen !== generation) return
      const room = new Room({ adaptiveStream: false })
      room
        .on(RoomEvent.TrackSubscribed, (track, pub, participant) => {
          if (track.kind !== Track.Kind.Audio) return
          audio.get(pub.trackSid)?.remove()
          const el = track.attach()
          el.dataset.voice = participant.identity
          document.body.appendChild(el)
          audio.set(pub.trackSid, el)
        })
        .on(RoomEvent.TrackUnsubscribed, (track, pub) => {
          for (const el of track.detach()) el.remove()
          audio.get(pub.trackSid)?.remove()
          audio.delete(pub.trackSid)
        })
        .on(RoomEvent.ActiveSpeakersChanged, (speakers) => {
          speaking.value = new Set(speakers.map((s) => s.identity))
        })
        .on(RoomEvent.AudioPlaybackStatusChanged, () => {
          needsAudioStart.value = !room.canPlaybackAudio
        })
        .on(RoomEvent.Disconnected, () => {
          // Kick or leaving the room on the server side: stop playing everything.
          if (lk === room) {
            lk = null
            clearInterval(timer)
            timer = undefined
            micOn.value = false
            dropAudio()
            status.value = 'off'
          }
        })
      await room.connect(token.url, token.token, { autoSubscribe: false })
      if (gen !== generation) {
        await room.disconnect()
        return
      }
      lk = room
      status.value = 'on'
      needsAudioStart.value = !room.canPlaybackAudio
      sync()
      timer = setInterval(sync, SYNC_MS)
    } catch (e) {
      if (gen !== generation) return
      const code = (e as { code?: string }).code
      status.value = code === 'voice_unavailable' ? 'unavailable' : 'error'
      error.value = code === 'voice_unavailable' ? '' : `Suara tidak tersambung: ${errorMessage(e)}`
    }
  }

  // sync subscribes to exactly the audible speakers' microphone tracks.
  function sync() {
    const map = opts.map.value
    if (!lk || !map) return
    const want = audible(map, opts.world, opts.chat.zone.key, opts.chat.voiceBroadcast)
    for (const p of lk.remoteParticipants.values()) {
      const on = want.has(p.identity)
      for (const pub of p.audioTrackPublications.values()) {
        if (pub.isDesired !== on) pub.setSubscribed(on)
      }
    }
  }

  async function toggleMic() {
    if (!lk) return
    const next = !micOn.value
    error.value = ''
    try {
      await lk.localParticipant.setMicrophoneEnabled(next)
      micOn.value = next
    } catch (e) {
      micOn.value = false
      error.value =
        (e as Error).name === 'NotAllowedError'
          ? 'Izin mikrofon ditolak. Izinkan mikrofon di pengaturan browser.'
          : 'Mikrofon tidak dapat dinyalakan.'
    }
  }

  async function startAudio() {
    await lk?.startAudio()
    needsAudioStart.value = !(lk?.canPlaybackAudio ?? true)
  }

  // A teacher mute removes publish permission server-side; reflect it here.
  watch(opts.chatVersion, () => {
    if (opts.chat.muted && micOn.value) {
      micOn.value = false
      void lk?.localParticipant.setMicrophoneEnabled(false).catch(() => {})
    }
  })

  // One LiveKit room per classroom/garden; mic is always off after travel.
  watch(
    () => opts.current.value,
    (cur, prev) => {
      const id = cur && cur.map_key !== 'campus' ? cur.room_id : null
      const prevId = prev && prev.map_key !== 'campus' ? prev.room_id : null
      if (id === prevId && lk) return
      void leave().then(() => {
        if (id) void join(id)
        else status.value = 'off'
      })
    },
  )

  onBeforeUnmount(() => void leave())

  return { status, error, micOn, speaking, needsAudioStart, toggleMic, startAudio }
}

export type VoiceHandle = ReturnType<typeof useVoice>

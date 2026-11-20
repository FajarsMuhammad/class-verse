import { onBeforeUnmount, ref, shallowRef, type Ref } from 'vue'

import { getAccessToken, refreshAccessToken } from '@/api/client'
import { RealtimeClient, roomSocketURL, type ConnectionStatus } from './client'
import { fetchMap, type MapData } from './mapdata'
import { ChatState } from './chat'
import type {
  BannedUser,
  LearningPin,
  BuildingCount,
  ChatScope,
  ConnectionMode,
  MoveMsg,
  EmoteKey,
  WelcomeMsg,
} from './messages'
import { World } from './world'

export interface RoomHandle {
  pin?: Ref<LearningPin | null>
  pinLearning?: (moduleID: string, stepID?: string) => void
  status: Ref<ConnectionStatus>
  map: Ref<MapData | null>
  world: World
  online: Ref<number>
  error: Ref<string>
  current: Ref<WelcomeMsg | null>
  buildings: Ref<BuildingCount[]>
  traveling: Ref<boolean>
  travel: (roomId: string) => void
  chat: ChatState
  // chatVersion changes whenever chat state does (ChatState itself is not reactive).
  chatVersion: Ref<number>
  chatError: Ref<string>
  sendChat: (text: string, scope: ChatScope) => boolean
  gather: () => void
  // voiceBroadcast starts or stops the teacher's voice to the whole room.
  voiceBroadcast: (on: boolean) => void
  kick: (target: string) => void
  moderate: (name: 'mute' | 'unmute', target: string) => void
  // muteAll mutes (or unmutes) everyone in the room now; each change is audited separately. Later joiners are not muted.
  muteAll: (on: boolean) => void
  setLocked: (locked: boolean) => void
  // muted lists participants muted in this session (staff only).
  mutedIds: Ref<string[]>
  allow: (target: string) => void
  // banned lists participants kicked from the open session (staff only).
  banned: Ref<BannedUser[]>
  // kicked is set when this user was removed; rejoin() returns to the campus.
  kicked: Ref<boolean>
  // people lists other avatars in the room (for teacher moderation).
  people: Ref<{ id: string; name: string; role: string }[]>
  rejoin: () => void
  sendMove: (m: MoveMsg) => void
  sendEmote: (key: EmoteKey) => boolean
  // seat picks a zone in mode HP ('' = general area).
  seat: (zone: string) => void
  start: (roomId: string, mapKey: string, mode?: ConnectionMode) => Promise<void>
}

// useRoom connects to one room over WebSocket and keeps a World in sync.
// The connection closes when the component unmounts.
export function useRoom(getToken: () => Promise<string | null> = freshToken): RoomHandle {
  const pin = ref<LearningPin | null>(null)
  const status = ref<ConnectionStatus>('connecting')
  const map = shallowRef<MapData | null>(null)
  const online = ref(0)
  const error = ref('')
  const world = new World()
  const current = shallowRef<WelcomeMsg | null>(null)
  const buildings = ref<BuildingCount[]>([])
  const traveling = ref(false)
  const banned = ref<BannedUser[]>([])
  const kicked = ref(false)
  const mutedIds = ref<string[]>([])
  const people = ref<{ id: string; name: string; role: string }[]>([])
  let mapVersion = 0
  const chat = new ChatState()
  const chatVersion = ref(0)
  const chatError = ref('')
  const chatErrors = new Set(['rate_limited', 'not_in_zone', 'all_chat_disabled'])
  let lastChatSeq = -1
  // Seqs of sent emotes; their rejections are transient and never block the map.
  const emoteSeqs = new Set<number>()
  let client: RealtimeClient | null = null
  let lastVersion = -1
  const abort = new AbortController()
  const offline = () => client?.networkLost()
  window.addEventListener('offline', offline)

  let mode: ConnectionMode = 'map'
  async function start(roomId: string, mapKey: string, connMode: ConnectionMode = 'map') {
    mode = connMode
    try {
      map.value = await fetchMap(mapKey, abort.signal)
    } catch {
      if (!abort.signal.aborted) error.value = 'Peta sekolah gagal dimuat. Muat ulang halaman.'
      return
    }
    if (abort.signal.aborted) return
    client = new RealtimeClient({
      url: roomSocketURL(roomId),
      mode,
      getToken,
      onStatus: (s) => {
        status.value = s
        if (s !== 'open') traveling.value = false
      },
      onMessage: (msg) => {
        world.apply(msg, performance.now())
        chat.apply(msg)
        if (chat.version !== chatVersion.value) chatVersion.value = chat.version
        if (msg.t === 'event' && msg.d!.name !== 'gather') {
          if (msg.d!.banned) banned.value = msg.d!.banned
          mutedIds.value = msg.d!.muted ?? []
        }
        if (msg.t === 'error' && msg.d!.code === 'kicked') kicked.value = true
        if (msg.t === 'welcome') emoteSeqs.clear() // seq restarts after every auth
        if (msg.t === 'welcome' || msg.t === 'traveled') {
          kicked.value = false
          banned.value = []
          mutedIds.value = []
          pin.value = null
          current.value = msg.d!
          traveling.value = false
          error.value = ''
          buildings.value = []
          const version = ++mapVersion
          map.value = null
          void fetchMap(msg.d!.map_key, abort.signal)
            .then((loaded) => {
              if (!abort.signal.aborted && version === mapVersion) map.value = loaded
            })
            .catch(() => {
              if (!abort.signal.aborted && version === mapVersion)
                error.value = 'Peta gagal dimuat. Muat ulang halaman.'
            })
        } else if (msg.t === 'room_state') pin.value = msg.d!.pin ?? null
        else if (msg.t === 'buildings') buildings.value = msg.d!.items
        else if (msg.t === 'error' && emoteSeqs.delete(msg.d!.seq ?? -1)) {
          // The bar already cools down and is disabled while muted.
        } else if (
          msg.t === 'error' &&
          (chatErrors.has(msg.d!.code) ||
            (msg.d!.code === 'bad_message' && msg.d!.seq === lastChatSeq))
        ) {
          chatError.value = msg.d!.message
        } else if (msg.t === 'error' && msg.d!.code !== 'move_rejected') {
          traveling.value = false
          error.value = msg.d!.message
        }
        if (world.version !== lastVersion) {
          lastVersion = world.version
          online.value = world.count
          people.value = [...world.avatars.values()]
            .map((a) => ({ id: a.id, name: a.name, role: a.role }))
            .sort((a, b) => a.name.localeCompare(b.name, 'id'))
        }
      },
    })
    client.start()
  }

  onBeforeUnmount(() => {
    abort.abort()
    window.removeEventListener('offline', offline)
    client?.stop()
  })

  return {
    pin,
    pinLearning: (moduleID, stepID) => {
      if (status.value !== 'open' || traveling.value) return
      if (moduleID) client?.send('cmd', { name: 'learning_pin', target: moduleID, step: stepID })
      else client?.send('cmd', { name: 'learning_unpin' })
    },
    status,
    map,
    world,
    online,
    error,
    current,
    buildings,
    traveling,
    travel: (roomId) => {
      if (status.value !== 'open' || traveling.value || !map.value) return
      error.value = ''
      traveling.value = true
      // Moves are throttled, so the server may still hold an older position.
      // Flush the predicted one first; the room handles it before the travel.
      const self = world.self
      if (self && mode === 'map')
        client?.send('move', { x: Math.round(self.x), y: Math.round(self.y), dir: self.dir })
      if (!client?.send('travel', { room_id: roomId })) traveling.value = false
    },
    chat,
    chatVersion,
    chatError,
    sendChat: (text, scope) => {
      if (status.value !== 'open' || !client) return false
      chatError.value = ''
      const seq = client.sendSeq('chat', { text, scope })
      if (seq === null) return false
      lastChatSeq = seq
      return true
    },
    banned,
    kicked,
    people,
    rejoin: () => {
      if (!kicked.value) return
      error.value = ''
      client?.start()
    },
    mutedIds,
    moderate: (name, target) => {
      if (status.value === 'open') client?.send('cmd', { name, target })
    },
    muteAll: (on) => {
      if (status.value === 'open') client?.send('cmd', { name: on ? 'mute_all' : 'unmute_all' })
    },
    setLocked: (locked) => {
      if (status.value === 'open') client?.send('cmd', { name: locked ? 'lock' : 'unlock' })
    },
    kick: (target) => {
      if (status.value === 'open') client?.send('cmd', { name: 'kick', target })
    },
    allow: (target) => {
      if (status.value === 'open') client?.send('cmd', { name: 'allow', target })
    },
    voiceBroadcast: (on) => {
      if (status.value === 'open')
        client?.send('cmd', { name: on ? 'voice_broadcast' : 'voice_broadcast_off' })
    },
    gather: () => {
      if (status.value === 'open' && !traveling.value) client?.send('cmd', { name: 'gather' })
    },
    seat: (zone) => {
      if (status.value === 'open' && !traveling.value) client?.send('seat', { zone })
    },
    sendEmote: (key) => {
      if (status.value !== 'open' || traveling.value || kicked.value) return false
      const seq = client?.sendSeq('emote', { kind: key }) ?? null
      if (seq === null) return false
      emoteSeqs.add(seq)
      if (emoteSeqs.size > 20) emoteSeqs.delete(emoteSeqs.values().next().value!)
      return true
    },
    sendMove: (m) => {
      if (!traveling.value && map.value) void client?.send('move', m)
    },
    start,
  }
}

// freshToken returns the in-memory access token unless it expires within a
// minute, in which case it refreshes first (the WebSocket checks the token
// only once, at auth).
async function freshToken(): Promise<string | null> {
  const t = getAccessToken()
  if (t && !expiresSoon(t)) return t
  return refreshAccessToken()
}

export function expiresSoon(jwt: string, nowMs = Date.now()): boolean {
  try {
    const payload = jwt.split('.')[1] ?? ''
    const json = atob(payload.replace(/-/g, '+').replace(/_/g, '/'))
    const exp = (JSON.parse(json) as { exp?: number }).exp
    return typeof exp !== 'number' || exp * 1000 - nowMs < 60_000
  } catch {
    return true
  }
}

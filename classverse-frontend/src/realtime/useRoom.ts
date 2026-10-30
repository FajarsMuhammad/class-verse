import { onBeforeUnmount, ref, shallowRef, type Ref } from 'vue'

import { getAccessToken, refreshAccessToken } from '@/api/client'
import { RealtimeClient, roomSocketURL, type ConnectionStatus } from './client'
import { fetchMap, type MapData } from './mapdata'
import type { MoveMsg } from './messages'
import { World } from './world'

export interface RoomHandle {
  status: Ref<ConnectionStatus>
  map: Ref<MapData | null>
  world: World
  online: Ref<number>
  error: Ref<string>
  sendMove: (m: MoveMsg) => void
  start: (roomId: string, mapKey: string) => Promise<void>
}

// useRoom connects to one room over WebSocket and keeps a World in sync.
// The connection closes when the component unmounts.
export function useRoom(): RoomHandle {
  const status = ref<ConnectionStatus>('connecting')
  const map = shallowRef<MapData | null>(null)
  const online = ref(0)
  const error = ref('')
  const world = new World()
  let client: RealtimeClient | null = null
  let lastVersion = -1
  const abort = new AbortController()
  const offline = () => client?.networkLost()
  window.addEventListener('offline', offline)

  async function start(roomId: string, mapKey: string) {
    try {
      map.value = await fetchMap(mapKey, abort.signal)
    } catch {
      if (!abort.signal.aborted) error.value = 'Peta sekolah gagal dimuat. Muat ulang halaman.'
      return
    }
    if (abort.signal.aborted) return
    client = new RealtimeClient({
      url: roomSocketURL(roomId),
      getToken: freshToken,
      onStatus: (s) => (status.value = s),
      onMessage: (msg) => {
        world.apply(msg, performance.now())
        if (world.version !== lastVersion) {
          lastVersion = world.version
          online.value = world.count
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
    status,
    map,
    world,
    online,
    error,
    sendMove: (m) => void client?.send('move', m),
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

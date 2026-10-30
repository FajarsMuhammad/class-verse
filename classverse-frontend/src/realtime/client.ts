import { CloseCode, type ClientMessages, type ServerMessage } from './messages'

export type ConnectionStatus =
  | 'connecting'
  | 'open'
  | 'reconnecting'
  | 'replaced' // another tab took over; no automatic reconnect
  | 'denied' // not allowed in this room; no automatic reconnect
  | 'closed'

// Reconnect backoff from docs/design.md: 1, 2, 4, 8 s, then 8 s again.
export const BACKOFF_MS = [1000, 2000, 4000, 8000] as const

export interface RealtimeOptions {
  url: string
  // getToken returns a fresh access token (refreshing if needed), or null
  // when the user is logged out.
  getToken: () => Promise<string | null>
  onMessage: (msg: ServerMessage) => void
  onStatus: (status: ConnectionStatus) => void
  // Injected for tests.
  createSocket?: (url: string) => WebSocket
  setTimer?: (fn: () => void, ms: number) => unknown
  clearTimer?: (handle: unknown) => void
}

// RealtimeClient keeps one WebSocket open to a room: it sends auth first,
// numbers outgoing messages, and reconnects with backoff.
export class RealtimeClient {
  private socket: WebSocket | null = null
  private seq = 0
  private attempt = 0
  private timer: unknown = null
  private stopped = false
  private authed = false
  private readonly opts: Required<RealtimeOptions>

  constructor(opts: RealtimeOptions) {
    this.opts = {
      createSocket: (url) => new WebSocket(url),
      setTimer: (fn, ms) => setTimeout(fn, ms),
      clearTimer: (h) => clearTimeout(h as ReturnType<typeof setTimeout>),
      ...opts,
    }
  }

  start() {
    if (this.socket || this.timer !== null) return
    this.stopped = false
    this.open()
  }

  stop() {
    this.stopped = true
    if (this.timer !== null) this.opts.clearTimer(this.timer)
    this.timer = null
    this.authed = false
    this.socket?.close(CloseCode.normal)
    this.socket = null
    this.opts.onStatus('closed')
  }

  // Browsers may leave a dead TCP socket open after losing network access.
  // Network events invalidate it, preserving the normal reconnect backoff.
  networkLost() {
    if (this.stopped || !this.socket) return
    const ws = this.socket
    this.socket = null
    this.authed = false
    ws.close(CloseCode.normal)
    if (this.timer === null) this.scheduleReconnect()
  }

  // send returns false when not connected and authenticated; callers drop
  // the message (positions are re-sent on the next step anyway).
  send<K extends keyof ClientMessages>(t: K, d: ClientMessages[K]): boolean {
    if (!this.socket || !this.authed || this.socket.readyState !== WebSocket.OPEN) return false
    this.socket.send(JSON.stringify({ t, seq: ++this.seq, d }))
    return true
  }

  private open() {
    this.opts.onStatus(this.attempt === 0 ? 'connecting' : 'reconnecting')
    this.authed = false
    const ws = this.opts.createSocket(this.opts.url)
    this.socket = ws
    ws.onopen = async () => {
      let token: string | null
      try {
        token = await this.opts.getToken()
      } catch {
        if (this.socket === ws) ws.close(CloseCode.normal)
        return
      }
      if (this.socket !== ws || this.stopped || ws.readyState !== WebSocket.OPEN) return
      if (!token) {
        this.stopped = true
        ws.close(CloseCode.normal)
        this.opts.onStatus('denied')
        return
      }
      this.seq = 0
      ws.send(JSON.stringify({ t: 'auth', seq: ++this.seq, d: { token } }))
    }
    ws.onmessage = (ev: MessageEvent) => {
      if (this.socket !== ws || this.stopped) return
      let msg: ServerMessage
      try {
        msg = JSON.parse(String(ev.data)) as ServerMessage
      } catch {
        return
      }
      if (msg.t === 'welcome') {
        this.authed = true
        this.attempt = 0
        this.opts.onStatus('open')
      }
      this.opts.onMessage(msg)
    }
    ws.onclose = (ev: CloseEvent) => {
      if (this.socket !== ws) return
      this.socket = null
      this.authed = false
      if (this.stopped) return
      if (ev.code === CloseCode.replaced) {
        this.stopped = true
        this.opts.onStatus('replaced')
        return
      }
      if (ev.code === CloseCode.notFound || ev.code === CloseCode.forbidden) {
        this.stopped = true
        this.opts.onStatus('denied')
        return
      }
      this.scheduleReconnect()
    }
  }

  private scheduleReconnect() {
    const delay = BACKOFF_MS[Math.min(this.attempt, BACKOFF_MS.length - 1)]!
    this.attempt++
    this.opts.onStatus('reconnecting')
    this.timer = this.opts.setTimer(() => {
      this.timer = null
      if (!this.stopped) this.open()
    }, delay)
  }
}

// roomSocketURL builds the same-origin WebSocket URL for a room.
export function roomSocketURL(roomId: string, loc: Location = window.location): string {
  const proto = loc.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${loc.host}/ws/rooms/${encodeURIComponent(roomId)}`
}

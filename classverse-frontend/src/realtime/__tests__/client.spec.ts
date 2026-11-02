import { describe, expect, it, vi } from 'vitest'

import { RealtimeClient, roomSocketURL, type ConnectionStatus } from '../client'
import type { ServerMessage } from '../messages'

class FakeSocket {
  static OPEN = 1
  readyState = 0
  sent: string[] = []
  onopen: (() => void) | null = null
  onmessage: ((ev: { data: string }) => void) | null = null
  onclose: ((ev: { code: number }) => void) | null = null
  closedWith: number | null = null
  send(s: string) {
    this.sent.push(s)
  }
  close(code: number) {
    this.closedWith = code
  }
  open() {
    this.readyState = 1
    this.onopen?.()
  }
  receive(msg: object) {
    this.onmessage?.({ data: JSON.stringify(msg) })
  }
  drop(code: number) {
    this.readyState = 3
    this.onclose?.({ code })
  }
}

function setup(token: string | null = 'tok') {
  vi.stubGlobal('WebSocket', { OPEN: 1 })
  const sockets: FakeSocket[] = []
  const timers: { fn: () => void; ms: number }[] = []
  const statuses: ConnectionStatus[] = []
  const messages: ServerMessage[] = []
  const client = new RealtimeClient({
    url: 'ws://x/ws/rooms/r',
    getToken: () => Promise.resolve(token),
    onMessage: (m) => messages.push(m),
    onStatus: (s) => statuses.push(s),
    createSocket: () => {
      const s = new FakeSocket()
      sockets.push(s)
      return s as unknown as WebSocket
    },
    setTimer: (fn, ms) => timers.push({ fn, ms }),
    clearTimer: () => {},
  })
  return { client, sockets, timers, statuses, messages }
}

const welcome = {
  t: 'welcome',
  seq: 1,
  d: { self_id: 'u', room_id: 'r', map_key: 'campus', participants: [] },
}

describe('RealtimeClient', () => {
  it('sends auth first and only sends other messages after welcome', async () => {
    const { client, sockets, statuses } = setup()
    client.start()
    expect(client.send('ping', { ts: 1 })).toBe(false)
    sockets[0]!.open()
    await Promise.resolve()
    expect(JSON.parse(sockets[0]!.sent[0]!)).toEqual({ t: 'auth', seq: 1, d: { token: 'tok' } })
    sockets[0]!.receive(welcome)
    expect(statuses[statuses.length - 1]).toBe('open')
    expect(client.send('move', { x: 1, y: 2, dir: 'up' })).toBe(true)
    expect(JSON.parse(sockets[0]!.sent[1]!)).toMatchObject({ t: 'move', seq: 2 })
  })

  it('reconnects with backoff 1, 2, 4, 8, 8 s and resets after welcome', async () => {
    const { client, sockets, timers } = setup()
    client.start()
    for (let i = 0; i < 5; i++) {
      sockets[sockets.length - 1]!.drop(1006)
      timers[timers.length - 1]!.fn()
    }
    expect(timers.map((t) => t.ms)).toEqual([1000, 2000, 4000, 8000, 8000])
    sockets[sockets.length - 1]!.open()
    await Promise.resolve()
    sockets[sockets.length - 1]!.receive(welcome)
    sockets[sockets.length - 1]!.drop(1006)
    expect(timers[timers.length - 1]!.ms).toBe(1000)
  })

  it('does not reconnect when replaced or denied', () => {
    for (const [code, status] of [
      [4409, 'replaced'],
      [4404, 'denied'],
    ] as const) {
      const { client, sockets, timers, statuses } = setup()
      client.start()
      sockets[0]!.drop(code)
      expect(timers).toHaveLength(0)
      expect(statuses[statuses.length - 1]).toBe(status)
    }
  })

  it('reconnects after an expired token (4401) so a fresh token is fetched', () => {
    const { client, sockets, timers } = setup()
    client.start()
    sockets[0]!.drop(4401)
    expect(timers).toHaveLength(1)
  })

  it('stops when logged out', async () => {
    const { client, sockets, statuses } = setup(null)
    client.start()
    sockets[0]!.open()
    await Promise.resolve()
    expect(statuses[statuses.length - 1]).toBe('denied')
    expect(sockets[0]!.sent).toHaveLength(0)
  })

  it('invalidates stale socket on network loss and schedules reconnect once', () => {
    const { client, sockets, timers, statuses } = setup()
    client.start()
    client.networkLost()
    client.networkLost()
    expect(timers).toHaveLength(1)
    expect(statuses[statuses.length - 1]).toBe('reconnecting')
    sockets[0]!.drop(1006)
    expect(timers).toHaveLength(1)
    timers[0]!.fn()
    expect(sockets).toHaveLength(2)
  })

  it('ignores late welcome from a replaced transport', () => {
    const { client, sockets, messages } = setup()
    client.start()
    client.networkLost()
    sockets[0]!.receive(welcome)
    expect(messages).toHaveLength(0)
  })

  it('builds same-origin URLs', () => {
    expect(roomSocketURL('abc', { protocol: 'https:', host: 'k.id' } as Location)).toBe(
      'wss://k.id/ws/rooms/abc',
    )
    expect(roomSocketURL('abc', { protocol: 'http:', host: 'localhost:5173' } as Location)).toBe(
      'ws://localhost:5173/ws/rooms/abc',
    )
  })
})

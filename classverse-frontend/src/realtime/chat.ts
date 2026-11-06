import type { ChatOutMsg, ChatScope, ServerMessage, ZoneMsg } from './messages'

// Messages kept per tab; older ones are dropped (docs/design.md, slice 6).
export const MAX_PER_TAB = 200

export interface ChatLine {
  key: string
  fromId: string
  fromName: string
  role: string
  text: string
  at: string
  mine: boolean
  broadcast: boolean
  // messageId is set when the server stored the message (classroom/garden).
  messageId?: number
}

// ChatState is plain TypeScript (unit-tested); Vue wraps it in a reactive ref.
export class ChatState {
  selfId: string | null = null
  roomId: string | null = null
  zone: ZoneMsg = { key: '', name: '' }
  allChatEnabled = true
  muted = false
  locked = false
  // Staff currently speaking to the whole room (voice broadcast).
  voiceBroadcast: string[] = []
  readonly lines: Record<ChatScope, ChatLine[]> = { near: [], zone: [], room: [], broadcast: [] }
  // Bumped on any change so Vue can re-render cheaply.
  version = 0
  private n = 0

  apply(msg: ServerMessage) {
    switch (msg.t) {
      case 'welcome':
      case 'traveled': {
        // A new room (or reconnect) starts with an empty history: the server
        // never replays chat. Same room after reconnect keeps what we saw.
        const sameRoom = msg.t === 'welcome' && this.roomId === msg.d!.room_id
        this.selfId = msg.d!.self_id
        this.allChatEnabled = msg.d!.all_chat_enabled
        this.muted = msg.d!.muted ?? false
        this.locked = msg.d!.locked ?? false
        this.voiceBroadcast = msg.d!.voice_broadcast ?? []
        if (!sameRoom) {
          this.lines.near = []
          this.lines.zone = []
          this.lines.room = []
          this.lines.broadcast = []
        }
        this.zone = { key: '', name: '' }
        this.roomId = msg.d!.room_id
        break
      }
      case 'zone':
        this.zone = msg.d!
        break
      case 'room_state':
        this.allChatEnabled = msg.d!.all_chat_enabled
        this.muted = msg.d!.muted ?? false
        this.locked = msg.d!.locked ?? false
        this.voiceBroadcast = msg.d!.voice_broadcast ?? []
        break
      case 'chat':
        this.add(msg.d!)
        break
      default:
        return
    }
    this.version++
  }

  private add(m: ChatOutMsg) {
    const list = this.lines[m.scope]
    if (!list) return
    const line: ChatLine = {
      key: m.id ? `id-${m.id}` : `n-${++this.n}`,
      fromId: m.from.id,
      fromName: m.from.name,
      role: m.from.role,
      text: m.text,
      at: m.at,
      mine: m.from.id === this.selfId,
      broadcast: m.scope === 'broadcast',
      messageId: m.id,
    }
    const targets = m.scope === 'broadcast' ? Object.values(this.lines) : [list]
    for (const target of targets) {
      target.push(line)
      if (target.length > MAX_PER_TAB) target.splice(0, target.length - MAX_PER_TAB)
    }
  }

  // defaultTab is the zone tab while inside a zone, otherwise near.
  defaultTab(): ChatScope {
    return this.zone.key ? 'zone' : 'near'
  }
}

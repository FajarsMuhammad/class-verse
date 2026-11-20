// Mirror of classverse-backend/internal/realtime/message.go. Change both files
// in the same slice; field names are the JSON names used on the wire.

export interface Envelope<T extends string = string, D = unknown> {
  t: T
  seq: number
  d?: D
}

export type EmoteKey = 'wave' | 'hand' | 'quiz' | 'cheer'
export type Direction = 'up' | 'down' | 'left' | 'right'

// Client → server
// mode 'list' is mode HP: no movement, travel without the door, seat (slice 8).
export type ConnectionMode = 'map' | 'list'
export interface AuthMsg {
  token: string
  mode?: ConnectionMode
}
// SeatMsg places a mode HP avatar in a zone; '' is the general area.
export interface SeatMsg {
  zone: string
}
export interface MoveMsg {
  x: number
  y: number
  dir: Direction
}
export interface PingMsg {
  ts: number
}

export interface ClientMessages {
  emote: { kind: EmoteKey }
  cmd: CommandMsg
  auth: AuthMsg
  move: MoveMsg
  ping: PingMsg
  travel: { room_id: string }
  chat: ChatInMsg
  seat: SeatMsg
}

export type CommandMsg =
  | { name: 'gather' | 'lock' | 'unlock' | 'mute_all' | 'unmute_all' | 'voice_broadcast' | 'voice_broadcast_off' }
  | { name: 'kick' | 'allow' | 'mute' | 'unmute'; target: string }
  | { name: 'learning_pin'; target: string; step?: string }
  | { name: 'learning_unpin' }
export interface BannedUser {
  id: string
  name: string
}
export type EventMsg =
  | { name: 'gather'; by: string; participants: Participant[] }
  | {
      name: 'kick' | 'allow' | 'mute' | 'unmute' | 'lock' | 'unlock'
      by: string
      target: string
      banned?: BannedUser[]
      muted: string[]
    }

export type ChatScope = 'near' | 'zone' | 'room' | 'broadcast'
export interface ChatInMsg {
  text: string
  scope: ChatScope
}

// Server → client
export interface Participant {
  id: string
  name: string
  avatar: string
  role: string
  x: number
  y: number
  dir: Direction
}
export interface WelcomeMsg {
  self_id: string
  room_id: string
  map_key: string
  room_name?: string
  building_key?: string
  participants: Participant[]
  all_chat_enabled: boolean
  locked?: boolean
  muted?: boolean
  // staff whose voice reaches the whole room (slice 9)
  voice_broadcast?: string[]
  voice_identity?: string
  voice_channel?: string
  voice_pending?: string[]
}
export interface LeftMsg {
  id: string
}
export interface Position {
  id: string
  x: number
  y: number
  dir: Direction
}
export interface SnapshotMsg {
  p: Position[]
  out?: string[]
}
export interface PongMsg {
  ts: number
}
export interface ErrorMsg {
  code: ErrorCode
  message: string
  seq?: number
  x?: number
  y?: number
  continue_path?: boolean
}

export type ErrorCode =
  | 'unauthenticated'
  | 'forbidden'
  | 'not_found'
  | 'bad_message'
  | 'move_rejected'
  | 'room_full'
  | 'replaced'
  | 'unavailable'
  | 'not_at_door'
  | 'scheduled'
  | 'travel_in_progress'
  | 'rate_limited'
  | 'not_in_zone'
  | 'kicked'
  | 'muted'
  | 'room_locked'
  | 'target_not_found'
  | 'all_chat_disabled'

export interface BuildingCount {
  building_key: string
  people: number
  live_rooms: number
}

export interface ChatOutMsg {
  id?: number
  from: { id: string; name: string; role: string }
  text: string
  scope: ChatScope
  zone?: string
  at: string
}
export interface ZoneMsg {
  key: string
  name: string
}
export interface LearningPin {
  module_id: string
  step_id: string
  title: string
  building_key: string
  level: string
}
export interface RoomStateMsg {
  pin?: LearningPin | null
  all_chat_enabled: boolean
  locked?: boolean
  muted?: boolean
  voice_broadcast?: string[]
  voice_identity?: string
  voice_channel?: string
  voice_pending?: string[]
}

export interface ServerMessages {
  emote: { id: string; kind: EmoteKey }
  event: EventMsg
  welcome: WelcomeMsg
  traveled: WelcomeMsg
  buildings: { items: BuildingCount[] }
  chat: ChatOutMsg
  zone: ZoneMsg
  room_state: RoomStateMsg
  snapshot: SnapshotMsg
  joined: Participant
  profile_updated: { id: string; name: string; avatar: string }
  left: LeftMsg
  pong: PongMsg
  error: ErrorMsg
}

export type ServerMessage = {
  [K in keyof ServerMessages]: Envelope<K, ServerMessages[K]>
}[keyof ServerMessages]

// Close codes (docs/design.md, slice 3).
export const CloseCode = {
  normal: 1000,
  goingAway: 1001,
  unauthenticated: 4401,
  forbidden: 4403,
  notFound: 4404,
  replaced: 4409,
} as const

// Movement rules shared with the server (internal/realtime/movement.go).
export const Movement = {
  maxSpeed: 160, // px per second, default when a map has no walk_speed
  maxMovesPerSec: 15,
  tickMs: 100,
} as const

// Mirror of classverse-backend/internal/realtime/message.go. Change both files
// in the same slice; field names are the JSON names used on the wire.

export interface Envelope<T extends string = string, D = unknown> {
  t: T
  seq: number
  d?: D
}

export type Direction = 'up' | 'down' | 'left' | 'right'

// Client → server
export interface AuthMsg {
  token: string
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
  auth: AuthMsg
  move: MoveMsg
  ping: PingMsg
  travel: { room_id: string }
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

export interface BuildingCount {
  building_key: string
  people: number
  live_rooms: number
}

export interface ServerMessages {
  welcome: WelcomeMsg
  traveled: WelcomeMsg
  buildings: { items: BuildingCount[] }
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

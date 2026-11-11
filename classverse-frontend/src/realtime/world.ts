import type { Direction, Participant, Position, ServerMessage } from './messages'

// Remote avatars are drawn this far in the past so there are always two
// snapshots to interpolate between (one server tick).
export const RENDER_DELAY_MS = 100
export const EMOTE_MS = 3000
export const emoteLabels = { wave: 'Halo!', hand: 'Aku mau tanya!', quiz: 'Ayo ke Arena!', cheer: 'Hore!' } as const

interface Sample {
  at: number
  x: number
  y: number
}

export interface Avatar {
  id: string
  name: string
  avatar: string
  role: string
  dir: Direction
  hidden: boolean // outside the area of interest
  samples: Sample[]
  // Rendered position (updated by sample()).
  x: number
  y: number
}

export interface SelfState {
  x: number
  y: number
  dir: Direction
}

// World holds what the client knows about the room. It is plain TypeScript
// so the rules are unit-tested; Phaser only reads from it.
export class World {
  selfId: string | null = null
  mapKey: string | null = null
  readonly avatars = new Map<string, Avatar>()
  self: SelfState | null = null
  selfName = 'Kamu'
  selfAvatar = 'default'
  // Bumped whenever avatars are added or removed (for UI lists and counts).
  version = 0
  movementRevision = 0
  readonly emotes = new Map<string, { text: string; until: number }>()

  apply(msg: ServerMessage, now: number) {
    switch (msg.t) {
      case 'welcome':
      case 'traveled': {
        const d = msg.d!
        this.selfId = d.self_id
        this.mapKey = d.map_key
        this.avatars.clear()
        this.emotes.clear()
        this.self = null
        for (const p of d.participants) {
          if (p.id === d.self_id) {
            this.self = { x: p.x, y: p.y, dir: p.dir }
            this.selfName = p.name
            this.selfAvatar = p.avatar
          } else this.upsert(p, now)
        }
        this.version++
        break
      }
      case 'event':
        if (msg.d!.name === 'gather') {
          for (const p of msg.d!.participants) {
            if (p.id === this.selfId) this.self = { x: p.x, y: p.y, dir: p.dir }
            else this.upsert(p, now)
          }
          this.movementRevision++
          this.version++
        }
        break
      case 'joined':
        if (msg.d!.id !== this.selfId) {
          this.upsert(msg.d!, now)
          this.version++
        }
        break
      case 'profile_updated': {
        const p = msg.d!
        if (p.id === this.selfId) {
          this.selfName = p.name
          this.selfAvatar = p.avatar
        }
        const a = this.avatars.get(p.id)
        if (a) {
          a.name = p.name
          a.avatar = p.avatar
        }
        this.version++
        break
      }
      case 'emote': {
        const { id, kind: key } = msg.d!
        if (Object.prototype.hasOwnProperty.call(emoteLabels, key) && (id === this.selfId || this.avatars.has(id)))
          this.emotes.set(id, { text: emoteLabels[key], until: now + EMOTE_MS })
        break
      }
      case 'left':
        this.emotes.delete(msg.d!.id)
        if (this.avatars.delete(msg.d!.id)) this.version++
        break
      case 'snapshot':
        for (const p of msg.d!.p) this.position(p, now)
        for (const id of msg.d!.out ?? []) {
          const a = this.avatars.get(id)
          if (a) a.hidden = true
        }
        break
      case 'error':
        if (
          msg.d!.code === 'move_rejected' &&
          this.self &&
          msg.d!.x !== undefined &&
          msg.d!.y !== undefined
        ) {
          this.self.x = msg.d!.x
          this.self.y = msg.d!.y
          // Successful zone corrections keep the route; real rejections stop
          // repeated attempts to enter a zone blocked by pending eviction.
          if (!msg.d!.continue_path) this.movementRevision++
        }
        break
    }
  }

  private upsert(p: Participant, now: number) {
    const existing = this.avatars.get(p.id)
    const a: Avatar = existing ?? {
      id: p.id,
      name: p.name,
      avatar: p.avatar,
      role: p.role,
      dir: p.dir,
      hidden: false,
      samples: [],
      x: p.x,
      y: p.y,
    }
    a.name = p.name
    a.avatar = p.avatar
    a.role = p.role
    a.dir = p.dir
    a.samples = [{ at: now, x: p.x, y: p.y }]
    a.x = p.x
    a.y = p.y
    this.avatars.set(p.id, a)
  }

  private position(p: Position, now: number) {
    const a = this.avatars.get(p.id)
    if (!a) return
    if (a.hidden) {
      // Coming back into view: jump instead of sliding across the map.
      a.hidden = false
      a.samples = [{ at: now, x: p.x, y: p.y }]
      a.x = p.x
      a.y = p.y
    } else {
      a.samples.push({ at: now, x: p.x, y: p.y })
      if (a.samples.length > 8) a.samples.shift()
    }
    a.dir = p.dir
  }

  // sample updates every avatar's rendered position for time `now`.
  sample(now: number) {
    for (const [id, emote] of this.emotes) if (emote.until <= now) this.emotes.delete(id)
    const t = now - RENDER_DELAY_MS
    for (const a of this.avatars.values()) {
      const s = a.samples
      if (s.length === 0) continue
      while (s.length >= 2 && s[1]!.at <= t) s.shift()
      const a0 = s[0]!
      const a1 = s[1]
      if (!a1 || t <= a0.at) {
        a.x = a0.x
        a.y = a0.y
        continue
      }
      const k = Math.min(1, (t - a0.at) / (a1.at - a0.at))
      a.x = a0.x + (a1.x - a0.x) * k
      a.y = a0.y + (a1.y - a0.y) * k
    }
  }

  get count() {
    return this.avatars.size + (this.selfId ? 1 : 0)
  }
}

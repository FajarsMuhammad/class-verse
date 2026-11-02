import { Movement } from './messages'

// Client view of a Tiled map, mirroring internal/realtime/mapdata.go so that
// client-side prediction agrees with the server.

export interface TiledProperty {
  name: string
  type?: string
  value: unknown
}

export interface TiledObject {
  id: number
  name?: string
  type?: string
  x: number
  y: number
  width: number
  height: number
  point?: boolean
  properties?: TiledProperty[]
}

export interface TiledLayer {
  name: string
  type: 'tilelayer' | 'objectgroup'
  width?: number
  height?: number
  data?: number[]
  objects?: TiledObject[]
  visible?: boolean
}

export interface TiledMap {
  properties?: TiledProperty[]
  width: number
  height: number
  tilewidth: number
  tileheight: number
  layers: TiledLayer[]
  tilesets: { firstgid: number; name: string; tilecount: number }[]
}

export interface Building {
  key: string
  name: string
  roof: string
  description: string
  staffOnly: boolean
  quiet: boolean
  x: number
  y: number
  w: number
  h: number
  doorX: number
  doorY: number
}

export interface Zone {
  key: string
  name: string
  x: number
  y: number
  w: number
  h: number
}

export class MapData {
  readonly width: number
  readonly height: number
  readonly tileSize: number
  readonly buildings: Building[]
  readonly zones: Zone[]
  // Max walking speed in px/s from the map property walk_speed (same rule as
  // the server, internal/realtime/mapdata.go).
  readonly walkSpeed: number
  private readonly blocked: Uint8Array

  constructor(readonly raw: TiledMap) {
    this.width = raw.width
    this.height = raw.height
    this.tileSize = raw.tilewidth
    const coll = raw.layers.find((l) => l.name === 'collision')
    if (!coll?.data || coll.data.length !== raw.width * raw.height) {
      throw new Error('map has no valid collision layer')
    }
    this.blocked = Uint8Array.from(coll.data, (g) => (g !== 0 ? 1 : 0))
    this.buildings = parseBuildings(raw)
    this.zones = (raw.layers.find((l) => l.name === 'zones')?.objects ?? []).map((o) => ({
      key: prop(o, 'key', ''),
      name: o.name ?? '',
      x: o.x,
      y: o.y,
      w: o.width,
      h: o.height,
    }))
    const speed = raw.properties?.find((p) => p.name === 'walk_speed')?.value
    this.walkSpeed = typeof speed === 'number' ? speed : Movement.maxSpeed
  }

  get pixelWidth() {
    return this.width * this.tileSize
  }

  get pixelHeight() {
    return this.height * this.tileSize
  }

  // blocked reports whether a pixel point is outside the map or on a
  // collision tile (same rule as the server).
  isBlocked(x: number, y: number): boolean {
    if (x < 0 || y < 0 || x >= this.pixelWidth || y >= this.pixelHeight) return true
    return (
      this.blocked[Math.floor(y / this.tileSize) * this.width + Math.floor(x / this.tileSize)] === 1
    )
  }

  isTileBlocked(tx: number, ty: number): boolean {
    if (tx < 0 || ty < 0 || tx >= this.width || ty >= this.height) return true
    return this.blocked[ty * this.width + tx] === 1
  }

  // pathClear samples the segment every 8 px like the server.
  pathClear(x0: number, y0: number, x1: number, y1: number): boolean {
    const dx = x1 - x0
    const dy = y1 - y0
    const steps = Math.ceil(Math.hypot(dx, dy) / 8)
    for (let i = 0; i <= steps; i++) {
      const t = steps > 0 ? i / steps : 1
      if (this.isBlocked(x0 + Math.round(dx * t), y0 + Math.round(dy * t))) return false
    }
    return true
  }
}

function prop<T>(o: TiledObject, name: string, fallback: T): T {
  const p = o.properties?.find((q) => q.name === name)
  return (p?.value as T | undefined) ?? fallback
}

function parseBuildings(raw: TiledMap): Building[] {
  const layer = raw.layers.find((l) => l.name === 'buildings')
  if (!layer?.objects) return []
  const doors = new Map<string, TiledObject>()
  for (const o of layer.objects) {
    if (o.type === 'door') doors.set(prop(o, 'building', ''), o)
  }
  return layer.objects
    .filter((o) => o.type === 'building')
    .map((o) => {
      const key = prop(o, 'key', '')
      const door = doors.get(key)
      return {
        key,
        name: prop(o, 'name', o.name ?? key),
        roof: prop(o, 'roof', ''),
        description: prop(o, 'description', ''),
        staffOnly: prop(o, 'staff_only', false),
        quiet: prop(o, 'quiet', false),
        x: o.x,
        y: o.y,
        w: o.width,
        h: o.height,
        doorX: Math.round(door?.x ?? o.x + o.width / 2),
        doorY: Math.round(door?.y ?? o.y + o.height),
      }
    })
}

export async function fetchMap(key: string, signal?: AbortSignal): Promise<MapData> {
  const res = await fetch(`/api/maps/${encodeURIComponent(key)}`, { signal })
  if (!res.ok) throw new Error(`map ${key}: HTTP ${res.status}`)
  return new MapData((await res.json()) as TiledMap)
}

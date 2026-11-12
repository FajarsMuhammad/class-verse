import rooms from '@/assets/world/rooms.json'

// Server map key -> rooms.json layout; mirrors LAYOUTS in scripts/generate_world.py.
export const roomLayouts = {
  classroom: 'kelas', garden: 'diskusi', lab: 'lab', baca: 'baca', library: 'perpustakaan',
} as const

interface Box { x: number; y: number; w: number; h: number }
interface Labelled extends Box { label: string }
interface RoomData {
  floor: string
  floor_size?: { w: number; h: number }
  items: (Box & { kind: string; label?: string })[]
  shelves?: Labelled[]
  areas?: Labelled[]
}

const DOOR_W = 100

// roomLayout returns world-space geometry (walls included) for an interior map.
export function roomLayout(mapKey: string) {
  const name = roomLayouts[mapKey as keyof typeof roomLayouts]
  if (!name) return null
  const room = rooms.rooms[name] as RoomData
  const wall = rooms.wall.thickness
  const floor = room.floor_size ?? rooms.floor_size
  const shift = <T extends Box>(b: T): T => ({ ...b, x: b.x + wall, y: b.y + wall })
  const width = floor.w + wall * 2
  const height = floor.h + wall * 2
  return {
    wall, floor, width, height,
    floorKind: room.floor,
    door: { x: width / 2 - DOOR_W / 2, y: floor.h + wall, w: DOOR_W, h: wall },
    items: room.items.map(shift),
    shelves: (room.shelves ?? []).map(shift),
    areas: (room.areas ?? []).map(shift),
    // Same rule as generate_world.py: only the library has category shelves.
    quiet: room.shelves !== undefined,
  }
}

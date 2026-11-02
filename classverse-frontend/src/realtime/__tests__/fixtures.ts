import { MapData, type TiledMap } from '../mapdata'

// smallMap builds a map from rows: '#' blocked, '.' free. Tile size 32.
export function smallMap(rows: string[], walkSpeed?: number): MapData {
  const height = rows.length
  const width = rows[0]!.length
  const data = rows.flatMap((r) => [...r].map((c) => (c === '#' ? 18 : 0)))
  const raw: TiledMap = {
    properties: walkSpeed ? [{ name: 'walk_speed', type: 'int', value: walkSpeed }] : undefined,
    width,
    height,
    tilewidth: 32,
    tileheight: 32,
    tilesets: [{ firstgid: 1, name: 'kelasverse', tilecount: 18 }],
    layers: [
      { name: 'ground', type: 'tilelayer', data: data.map(() => 1) },
      { name: 'collision', type: 'tilelayer', data },
      { name: 'spawn', type: 'objectgroup', objects: [] },
      {
        name: 'buildings',
        type: 'objectgroup',
        objects: [
          {
            id: 1,
            type: 'building',
            x: 32,
            y: 0,
            width: 64,
            height: 32,
            properties: [
              { name: 'key', value: 'kelas-a' },
              { name: 'name', value: 'Gedung Kelas A' },
              { name: 'staff_only', value: false },
            ],
          },
          {
            id: 2,
            type: 'door',
            x: 64,
            y: 48,
            width: 0,
            height: 0,
            properties: [{ name: 'building', value: 'kelas-a' }],
          },
        ],
      },
    ],
  }
  return new MapData(raw)
}

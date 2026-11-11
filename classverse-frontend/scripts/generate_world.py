"""Build server Tiled geometry from production frontend world data (stdlib only)."""
import json
import math
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
DATA = ROOT / 'classverse-frontend/src/assets/world'
OUT = ROOT / 'classverse-backend/maps'
TILE = 32


def props(**values):
    return [{'name': key, 'type': 'bool' if isinstance(value, bool) else
             'int' if isinstance(value, int) else 'string', 'value': value}
            for key, value in values.items()]


def obj(label, x, y, w=0, h=0, kind='', **values):
    return dict(name=label, type=kind, x=x, y=y, width=w, height=h,
                properties=props(**values))


def build(width, height, spawn, blocks, buildings=(), zones=(), exit_point=None, seats=(), speed=160):
    cols, rows = math.ceil(width / TILE), math.ceil(height / TILE)
    collision = [int(any(x <= tx*TILE+TILE/2 < x+w and y <= ty*TILE+TILE/2 < y+h
                         for x, y, w, h in blocks))
                 for ty in range(rows) for tx in range(cols)]
    # Seats and doorway standing points remain walkable, including narrow furniture gaps.
    free = [spawn, *seats, *[(b['x'], b['y']) for b in buildings if b['type'] == 'door']]
    if exit_point:
        free.append(exit_point)
    for x, y in free:
        collision[int(y // TILE)*cols + int(x // TILE)] = 0
    layers = [dict(name='ground', type='tilelayer', width=cols, height=rows, data=[1]*(cols*rows)),
              dict(name='collision', type='tilelayer', width=cols, height=rows, data=collision),
              dict(name='spawn', type='objectgroup', objects=[obj('spawn', spawn[0]-32, spawn[1]-32, 64, 64)]),
              dict(name='buildings', type='objectgroup', objects=list(buildings)),
              dict(name='zones', type='objectgroup', objects=list(zones))]
    if exit_point:
        layers.append(dict(name='exit', type='objectgroup', objects=[obj('exit', *exit_point)]))
    layers.append(dict(name='seats', type='objectgroup', objects=[obj(str(i), *s) for i, s in enumerate(seats)]))
    for i, layer in enumerate(layers, 1):
        layer['id'] = i
        for j, item in enumerate(layer.get('objects', []), 1):
            item['id'] = i*1000+j
    return dict(type='map', version='1.10', orientation='orthogonal', renderorder='right-down',
                width=cols, height=rows, tilewidth=TILE, tileheight=TILE, layers=layers,
                properties=props(walk_speed=speed, near_radius=85, world_width=width, world_height=height),
                tilesets=[dict(firstgid=1, name='kelasverse', tilecount=18)])


def campus(world):
    buildings, blocks = [], []
    for p in world['places']:
        x, y = p['sprite_pos'].values()
        w, h = p['size'].values()
        # Arena is an open playing field; other landmarks have solid rear walls.
        if p['id'] != 'arena':
            blocks.append((x, y, w, max(0, p['entrance']['y']-y-16)))
        buildings += [obj(p['name'], x, y, w, h, 'building', key=p['id'], name=p['name'],
                          description=p['subject'], roof='classroom', staff_only=p.get('staff_only', False), interact_radius=p['interact_radius']),
                      obj('door', **p['entrance'], kind='door', building=p['id'])]
    for z in world['zones']:
        r = z['rect']
        buildings += [obj(z['name'], r['x'], r['y'], r['w'], r['h'], 'building',
                          key=z['id'], name=z['name'], description=z['subject'], roof=''),
                      obj('door', r['x']+r['w']/2, r['y']+r['h']-24, kind='door', building=z['id'])]
    return build(world['size']['w'], world['size']['h'], tuple(world['spawn'].values()),
                 blocks, buildings, speed=320)


def interior(data, room):
    wall = data['wall']['thickness']
    width, height = data['floor_size']['w']+2*wall, data['floor_size']['h']+2*wall
    blocks = [(0, 0, width, wall), (0, 0, wall, height), (width-wall, 0, wall, height),
              (0, height-wall, width/2-50, wall), (width/2+50, height-wall, width/2-50, wall)]
    for item in room['items']:
        if item['kind'] in ('desk', 'table', 'shelf', 'sink'):
            blocks.append((item['x']+wall, item['y']+wall, item['w'], item['h']))
    seats = [(s['x']+wall, s['y']+wall) for s in room['seats']]
    zones = [obj(z['label'], z['x']+wall-z['d']/2, z['y']+wall-z['d']/2,
                 z['d'], z['d'], key=f'table-{i+1}', shape='circle') for i, z in enumerate(room['zones'])]
    return build(width, height, (width/2, height-60), blocks, zones=zones,
                 exit_point=(width/2, height-wall/2), seats=seats)


def generate():
    world = json.loads((DATA / 'world.json').read_text())
    rooms = json.loads((DATA / 'rooms.json').read_text())
    maps = {'campus': campus(world)}
    for key, layout in [('classroom', 'kelas'), ('garden', 'diskusi'), ('lab', 'lab'), ('baca', 'baca')]:
        maps[key] = interior(rooms, rooms['rooms'][layout])
    for key, data in maps.items():
        (OUT / f'{key}.tmj').write_text(json.dumps(data, separators=(',', ':'))+'\n')
    return maps


if __name__ == '__main__':
    generate()

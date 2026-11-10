#!/usr/bin/env python3
"""Generate campus.tmj (128x80 tiles). Layout data is the source of truth; run with --check in tests/CI."""
import argparse
import json
from pathlib import Path

W, H, TS = 128, 80, 32
GRASS, DARK, PATH, PLAZA, WALL, WINDOW, DOOR = 1, 2, 3, 4, 5, 6, 7
ROOF = {"classroom": 8, "library": 9, "hall": 10, "canteen": 11, "staff": 12, "lab": 19, "studio": 20, "mosque": 21, "clinic": 22}
WATER, WOOD, LINE, FLAG, TREE, BLOCK = 13, 14, 15, 16, 17, 18
LAMP, FLOWER, SAND = 23, 24, 25

# (key, name, roof, description, tile rect (x, y, w, h), staff_only, quiet).
# Removed buildings are absent; existing room records are never deleted here.
BUILDINGS = [
    ("kelas-a", "Gedung Kelas A", "classroom", "Ruang kelas", (6, 4, 26, 16), False, False),
    ("kelas-b", "Gedung Kelas B", "classroom", "Ruang kelas", (6, 60, 26, 16), False, False),
    ("kelas-c", "Gedung Kelas C", "classroom", "Ruang kelas tingkat atas", (96, 60, 26, 16), False, False),
    ("ruang-guru", "Ruang Guru", "staff", "Khusus pengajar dan admin", (38, 6, 14, 14), True, False),
    ("perpustakaan", "Perpustakaan", "library", "Belajar mandiri, chat saja", (76, 4, 24, 16), False, True),
    ("kantin", "Kantin", "canteen", "Ngobrol santai saat istirahat", (58, 62, 14, 14), False, False),
    ("taman", "Taman diskusi", "", "Kerja kelompok di luar kelas", (6, 28, 22, 24), False, False),
    ("lapangan", "Lapangan", "", "Olahraga dan main bersama", (96, 40, 28, 18), False, False),
]
PLAZA_RECT = (54, 32, 20, 16)
SPAWN = (57, 35, 14, 10)
ROADS = [
    (1, 24, W - 2, 2), (1, 56, W - 2, 2),      # north and south avenues
    (62, 1, 4, H - 2),                         # north-south spine
    (1, 39, 52, 2), (75, 39, 52, 2),           # east-west promenade through the plaza
    (16, 26, 2, 2),
    (18, 20, 2, 4), (18, 52, 2, 4),            # Kelas A / B approaches
    (44, 20, 2, 4),                          # Ruang Guru approach
    (87, 20, 2, 4),                          # Perpustakaan approach
    (28, 26, 2, 13),                         # Taman side lane
    (92, 26, 2, 30),                           # eastern lane along the field
    (18, 77, 93, 2), (54, 56, 2, 23),        # south doors and bypass around the canteen
]
TREES = [(3, 2), (34, 2), (54, 2), (72, 2), (102, 2), (124, 2), (3, 22), (124, 22), (3, 58), (124, 58),
         (3, 77), (34, 77), (54, 77), (74, 77), (92, 77), (124, 77), (31, 36), (31, 46), (33, 28),
         (56, 28), (72, 28), (56, 50), (72, 50), (95, 30), (116, 30), (99, 36), (120, 36), (90, 44),
         (90, 50), (50, 48), (76, 60), (90, 62), (100, 24), (20, 26), (24, 54), (84, 24)]
LAMPS = [(53, 31), (74, 31), (53, 48), (74, 48), (60, 24), (68, 24), (60, 57), (68, 57), (30, 40), (97, 40)]
FLOWERS = [(55, 33, 3, 1), (70, 33, 3, 1), (55, 46, 3, 1), (70, 46, 3, 1), (12, 22, 4, 1), (84, 22, 4, 1), (12, 58, 4, 1), (100, 58, 4, 1)]
BENCHES = [(58, 30, 3, 1), (67, 30, 3, 1), (58, 49, 3, 1), (67, 49, 3, 1), (32, 24, 3, 1), (94, 24, 3, 1)]



def build():
    ground = [GRASS] * (W * H)
    collision = [0] * (W * H)

    def fill(layer, rect, value):
        x0, y0, w, h = rect
        for y in range(y0, y0 + h):
            for x in range(x0, x0 + w):
                if 0 <= x < W and 0 <= y < H:
                    layer[y * W + x] = value

    def blocked(rect, value):
        fill(ground, rect, value)
        fill(collision, rect, BLOCK)

    def point(tx, ty, value):
        ground[ty * W + tx] = value
        collision[ty * W + tx] = BLOCK

    # Perimeter hedge keeps avatars inside the school.
    for rect in ((0, 0, W, 1), (0, H - 1, W, 1), (0, 0, 1, H), (W - 1, 0, 1, H)):
        blocked(rect, DARK)
    for road in ROADS:
        fill(ground, road, PATH)
    fill(ground, PLAZA_RECT, PLAZA)
    point(PLAZA_RECT[0] + 10, PLAZA_RECT[1] + 7, FLAG)

    objects, doors = [], []
    for key, name, roof, desc, (bx, by, bw, bh), staff, quiet in BUILDINGS:
        south = by + bh - 1
        if roof:
            blocked((bx, by, bw, bh), WALL)
            fill(ground, (bx, by, bw, 4), ROOF[roof])
            rows = (by + 6, by + 10) if bh >= 16 else (by + 6,)
            for wy in rows:
                for wx in range(bx + 2, bx + bw - 2, 4):
                    fill(ground, (wx, wy, 2, 1), WINDOW)
            door_tx = bx + bw // 2
            fill(ground, (door_tx - 1, south - 1, 2, 2), DOOR)
            door = (door_tx * TS, (south + 2) * TS)
        else:
            fill(ground, (bx, by, bw, bh), DARK)
            if key == "lapangan":
                for rect in ((bx + 1, by + 1, bw - 2, 1), (bx + 1, by + bh - 2, bw - 2, 1), (bx + 1, by + 1, 1, bh - 2),
                             (bx + bw - 2, by + 1, 1, bh - 2), (bx + 1, by + bh // 2, bw - 2, 1)):
                    fill(ground, rect, LINE)
                for rect in ((bx, by, bw, 1), (bx, by + bh - 1, bw, 1), (bx, by, 1, bh), (bx + bw - 1, by, 1, bh)):
                    fill(collision, rect, BLOCK)
                collision[by * W + bx + bw // 2] = 0
                collision[by * W + bx + bw // 2 - 1] = 0
                door = ((bx + bw // 2) * TS, (by - 1) * TS + TS // 2)
            else:
                blocked((bx + 7, by + 9, 8, 6), WATER)
                for bench in ((bx + 3, by + 4, 3, 1), (bx + bw - 6, by + bh - 5, 3, 1)):
                    blocked(bench, WOOD)
                for tx, ty in ((bx + bw - 4, by + 4), (bx + 4, by + bh - 4)):
                    point(tx, ty, TREE)
                door = ((bx + bw // 2) * TS, (by - 1) * TS + TS // 2)
        objects.append({
            "id": len(objects) + 1, "name": name, "type": "building", "x": bx * TS, "y": by * TS,
            "width": bw * TS, "height": bh * TS, "rotation": 0, "visible": True,
            "properties": [
                {"name": "key", "type": "string", "value": key}, {"name": "name", "type": "string", "value": name},
                {"name": "roof", "type": "string", "value": roof}, {"name": "description", "type": "string", "value": desc},
                {"name": "staff_only", "type": "bool", "value": staff}, {"name": "quiet", "type": "bool", "value": quiet},
            ],
        })
        doors.append((key, door))
    for tx, ty in TREES:
        if collision[ty * W + tx] == 0 and ground[ty * W + tx] == GRASS:
            point(tx, ty, TREE)
    for tx, ty in LAMPS:
        if collision[ty * W + tx] == 0:
            point(tx, ty, LAMP)
    for rect in FLOWERS:
        blocked(rect, FLOWER)
    for bench in BENCHES:
        blocked(bench, WOOD)
    # Door stands and the two tiles beneath are paved and walkable.
    for key, (dx, dy) in doors:
        tx, ty = dx // TS, dy // TS
        for ox in (-1, 0):
            for oy in (0, 1, 2):
                if 0 < tx + ox < W - 1 and 0 < ty + oy < H - 1:
                    ground[(ty + oy) * W + tx + ox] = PATH
                    collision[(ty + oy) * W + tx + ox] = 0
        objects.append({
            "id": len(objects) + 1, "name": "door", "type": "door", "x": dx, "y": dy, "width": 0, "height": 0,
            "point": True, "rotation": 0, "visible": True,
            "properties": [{"name": "building", "type": "string", "value": key}],
        })
    sx, sy, sw, sh = SPAWN
    return {
        "type": "map", "version": "1.10", "tiledversion": "1.11.0", "orientation": "orthogonal", "renderorder": "right-down",
        "width": W, "height": H, "tilewidth": TS, "tileheight": TS, "infinite": False, "nextlayerid": 5,
        "nextobjectid": 101,
        "properties": [{"name": "map_key", "type": "string", "value": "campus"}, {"name": "walk_speed", "type": "int", "value": 320}],
        "tilesets": [{"firstgid": 1, "name": "kelasverse", "tilewidth": TS, "tileheight": TS, "tilecount": 25, "columns": 25,
                      "image": "kelasverse-tiles.png", "imagewidth": 800, "imageheight": 32, "margin": 0, "spacing": 0}],
        "layers": [
            {"id": 1, "name": "ground", "type": "tilelayer", "width": W, "height": H, "x": 0, "y": 0, "opacity": 1, "visible": True, "data": ground},
            {"id": 2, "name": "collision", "type": "tilelayer", "width": W, "height": H, "x": 0, "y": 0, "opacity": 1, "visible": False, "data": collision},
            {"id": 3, "name": "spawn", "type": "objectgroup", "x": 0, "y": 0, "opacity": 1, "visible": True, "draworder": "topdown",
             "objects": [{"id": 100, "name": "spawn", "type": "spawn", "x": sx * TS, "y": sy * TS, "width": sw * TS, "height": sh * TS, "rotation": 0, "visible": True}]},
            {"id": 4, "name": "buildings", "type": "objectgroup", "x": 0, "y": 0, "opacity": 1, "visible": True, "draworder": "topdown", "objects": objects},
        ],
    }


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    out = Path(__file__).with_name("campus.tmj")
    campus = build()
    # Check paved connectivity, not just grass reachability: every entrance joins the promenade.
    ground = campus['layers'][0]['data']
    collision = campus['layers'][1]['data']
    start = (SPAWN[0], SPAWN[1])
    seen, queue = {start}, [start]
    for x, y in queue:
        for nx, ny in ((x-1, y), (x+1, y), (x, y-1), (x, y+1)):
            if (0 <= nx < W and 0 <= ny < H and (nx, ny) not in seen
                    and ground[ny*W+nx] in (PATH, PLAZA) and not collision[ny*W+nx]):
                seen.add((nx, ny))
                queue.append((nx, ny))
    for obj in campus['layers'][3]['objects']:
        if obj['type'] == 'door':
            assert (obj['x']//TS, obj['y']//TS) in seen, f"Unpaved door: {obj['properties']}"
    text = json.dumps(campus, separators=(",", ":")) + "\n"
    if args.check:
        assert out.read_text() == text, "campus.tmj is stale; run generate_campus.py"
        print("campus.tmj up to date")
    else:
        out.write_text(text)
        print(f"wrote {out} ({W}x{H}, {len(BUILDINGS)} buildings)")

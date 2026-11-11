import unittest
from collections import deque
import json
from generate_world import campus, interior, DATA, OUT, TILE


class WorldTests(unittest.TestCase):
    def assert_reachable(self, m):
        cols, rows = m['width'], m['height']
        layers = {l['name']: l for l in m['layers']}
        blocked = layers['collision']['data']
        spawn = layers['spawn']['objects'][0]
        starts = [(x, y) for y in range(rows) for x in range(cols)
                  if spawn['x'] <= x*TILE+16 < spawn['x']+spawn['width']
                  and spawn['y'] <= y*TILE+16 < spawn['y']+spawn['height'] and not blocked[y*cols+x]]
        self.assertTrue(starts)
        seen = set(starts)
        queue = deque(starts)
        while queue:
            x, y = queue.popleft()
            for dx, dy in [(1, 0), (-1, 0), (0, 1), (0, -1)]:
                p = x+dx, y+dy
                if 0 <= p[0] < cols and 0 <= p[1] < rows and p not in seen and not blocked[p[1]*cols+p[0]]:
                    seen.add(p)
                    queue.append(p)
        points = layers['seats']['objects'] + [b for b in layers['buildings']['objects'] if b['type'] == 'door']
        points += layers.get('exit', {}).get('objects', [])
        for p in points:
            self.assertIn((int(p['x']//TILE), int(p['y']//TILE)), seen, p)

    def test_every_entrance_and_seat_is_reachable(self):
        w = json.loads((DATA/'world.json').read_text())
        c = campus(w)
        self.assert_reachable(c)
        self.assertEqual(len([b for b in c['layers'][3]['objects'] if b['type'] == 'building']), 10)
        r = json.loads((DATA/'rooms.json').read_text())
        for room in r['rooms'].values():
            self.assert_reachable(interior(r, room))


    def test_committed_server_maps_match_frontend_data(self):
        # Editing world.json/rooms.json without regenerating leaves the
        # server enforcing old collision, doors and zones.
        w = json.loads((DATA/'world.json').read_text())
        r = json.loads((DATA/'rooms.json').read_text())
        want = {'campus': campus(w)}
        for key, layout in [('classroom', 'kelas'), ('garden', 'diskusi'), ('lab', 'lab'), ('baca', 'baca')]:
            want[key] = interior(r, r['rooms'][layout])
        for key, data in want.items():
            self.assertEqual(json.loads((OUT/f'{key}.tmj').read_text()), data,
                             f'{key}.tmj is stale: run python3 scripts/generate_world.py')

if __name__ == '__main__':
    unittest.main()

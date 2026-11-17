# Pending world sprites

Copied from the reference pack for features that are not built yet. They sit
outside `sprites/`, so `worldArt.ts` does not bundle or load them.

| Sprite | Planned use | Needs before moving into `sprites/props/` |
| --- | --- | --- |
| `papan-misi.svg` | Daily missions board at the Alun-alun (Kiko's chat line points there) | Persisted missions service and a position in `world.json` |
| `panggung.svg` | Stage in the Alun-alun for announcements/events | Event feature and a position in `world.json` |
| `fountain.svg`, `flowers.svg` | Campus decoration | Positions in `world.json` and collision in `generate_world.py` |

Move a file into `sprites/props/`, add it to `world.json`, then regenerate the
server maps with `scripts/generate_world.py` so collision matches.

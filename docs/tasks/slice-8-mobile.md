# Slice 8 — Mode HP (M1)

Status: **decisions proposed; awaiting approval**. No code yet.
Branch: `slice-8-mobile`, based on `fix-travel-webgl-leak` (`cd4583f`, slice 7 + WebGL fix).

References: docs/spec.md (M1 row: "mode HP — pilih zona lewat daftar + chat, tanpa movement"; NFR Perangkat),
docs/visual.md (alur HP, `05-hp-peta-sekolah.png`, `06-hp-di-ruang.png`), docs/design.md (slice 5 door rule,
slice 6 zones/near, slice 7 controls).

## Existing requirements

- Phone users (Chrome Android, Safari iOS) use a simple view without movement.
- Campus: "Di dekatmu" (people + latest near message), building list with people/live counts, open a building's rooms,
  near-chat input.
- Room: back button, room name + building + people, teacher announcement, "Pilih tempat" (Area umum + zones with
  counts), "Kamu di <zona>" with tablemates, zone chat. Mic is M2 (not in this slice).

## Conflict with approved design

Slice 5 requires every role to stand at the door (48 px) to enter and at the exit to leave; `move` cannot teleport.
A phone without movement cannot satisfy either rule or enter a zone. A server-side exception is required.

## Decisions proposed for approval

| ID | Proposed behavior |
| M8-01 | Mode HP turns on automatically when `matchMedia('(pointer: coarse) and (max-width: 767px)')` matches at room start; no manual toggle in M1. Phaser is not loaded in mode HP. Wider touch devices (tablets) keep the map (best effort). |
| M8-02 | Client declares list mode in the first message: `auth {token, mode: "list"}` (default `"map"`). The mode is fixed per connection and survives travel; reconnect re-declares it. Server rejects `move` from list connections (`move_rejected` with official position). |
| M8-03 | List connections skip the door/exit proximity check in `travel`. All other admission rules stay (membership, Ruang Guru staff-only, schedule, capacity, lock, ban, travel serialization). Entering places the avatar at the map spawn; returning places it at the origin building's door, as today. Trade-off: a desktop client could forge list mode to skip walking; acceptable because the door rule is presence UX, not access control. |
| M8-04 | New client message `seat {zone}` (list mode only, classroom/garden only, max 1/s). Server puts the avatar on the centre walkable tile of that zone (first walkable tile scanning from the centre if blocked); `zone: ""` returns to the spawn ("Area umum"). Unknown zone → `bad_message`; campus or map connection → `forbidden`. Zone change then flows through the existing tick (`zone` message, snapshots), so map users see the phone avatar sit at the table. Gather still moves everyone to spawn. |
| M8-05 | No other protocol changes. Zone counts and tablemates are computed on the client from snapshot positions and map zones (rooms have no AOI). Campus "Di dekatmu" = avatars within 160 px of self from snapshots; the preview shows the latest `near` chat line. Building counts reuse `buildings`; room rows reuse `GET /api/orgs/{org_id}/rooms?building=` (`people`, `live`). |
| M8-06 | Teacher on a phone keeps the announcement tab and the existing teacher panel (gather, all-chat, lock, participants mute/kick/allow) in a collapsible section; Moderasi page already works on narrow screens. |

## Plan (after approval)

1. Design/OpenAPI-free contract update: `docs/design.md` slice 8 section, `message.go` (`AuthMsg.Mode`, `SeatMsg`,
   `TypeSeat`), `messages.ts` mirror.
2. Backend `realtime`: list mode on `conn`; travel check bypass; `seat` handling in the room loop with rate limit;
   reject `move` in list mode. Table-driven tests: seat each zone/area umum, unknown zone, campus, map client,
   rate limit, travel without door (list) vs `not_at_door` (map), lock/ban/staff-only still enforced, gather after seat.
3. Frontend: `useRoom.start(roomId, mapKey, mode)`; `MobileCampus.vue` and `MobileRoom.vue` (no Phaser) reusing
   `ChatPanel`, `DoorRooms`, `ReportDialog`; `CampusRoom.vue` picks mobile vs map. Pure helpers
   `zoneCounts(world, map)` / `nearby(world, radius)` with unit tests.
4. Checks: `make test && make lint`; `npm run lint && npm run test:unit -- --run && npm run build`.
5. Browser: Chrome device emulation (phone) + desktop: phone enters room from the list, picks Meja 3, desktop sees
   the avatar at Meja 3 and zone chat works both ways; lock/kick/ban still apply; back returns to campus door.
6. User review before commit (`slice 8`, no story ID; mention M1 mode HP).

## Deliberately out of scope

- Mic button / voice (slice 9, M2).
- Manual mode toggle, tablet-specific layout.

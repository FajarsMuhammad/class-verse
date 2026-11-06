# Slice 8 — Mode HP (M1)

Status: **reviewed; functional, phone layout polish deferred to the next MVP** (user decision: focus on web). M8-01…M8-06 approved and written to `docs/design.md`.
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

## Decisions (approved)

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

## Result

- Backend `realtime`: `AuthMsg.Mode` (`map`/`list`, other values close 4401), `conn.list`, travel skips the
  door/exit check for list connections, `seat {zone}` (1/s, classroom/garden, list only), list `move` rejected.
  `MapData.SeatPoint` picks a walkable tile inside the zone (zone centres are tables); parsing rejects zones without
  one. Tests: `seat_test.go` (every zone of classroom/garden + area umum, rejections, campus, zone chat + gather,
  hub travel list vs map + capacity), gateway (list mode reaches the conn, unknown mode rejected).
- Frontend: `MobileRoom.vue` (campus: Di dekatmu + building list with live/people + rooms; room: back, Pilih tempat
  with counts, tablemates, chat), `TeacherPanel.vue` extracted and shared with the map view, `places.ts`
  (zone counts/nearby from newest snapshots), `RealtimeClient` `mode`, `useRoom.seat`. Phaser is lazy-loaded
  (`GameCanvas` chunk), so phones do not download it. Tests: `places.spec.ts`, `MobileRoom.spec.ts`, client auth mode.
- Checks: `go test -race ./...` (11 packages ok) and `make lint` (0 issues); frontend lint ok, 90 tests,
  build ok (Phaser chunk-size warning remains, now isolated to `GameCanvas`).
- Chrome (puppeteer, phone 390×844 touch + desktop 1400×900, `/tmp/kv-e2e/mobile.mjs`): phone has no canvas,
  enters Biologi from the list, seats at Meja 3, zone chat, back to campus; desktop still must walk to the
  Kelas A door; phone counts the desktop user; room chat crosses devices; second seat within 1 s rate limited;
  teacher on phone uses Kontrol pengajar to lock; phone re-entry rejected `room_locked`; no runtime errors.

## Known limits

- AppShell header wraps to three rows on phones on this branch; `main` has `00a1d36` (nav on one row) not merged here.
- Mic (M2), manual mode toggle, tablet layout not done.
- "Di dekatmu" uses the campus snapshot (AOI 24 tiles), so it is accurate for the 160 px radius.

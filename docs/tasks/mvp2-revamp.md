# MVP 2 — Kelasverse revamp

## Approved direction

User approved full revamp from `kelasverse-assets/README.md` and four reference prototypes, new migrations, latest design features (including features outside MVP 1), and front-facing idle avatars (option a). Work branch: `mvp2-revamp`; baseline: `a78a482`.

This is MVP 2, not the previous MVP 1 voice milestone M2. Previous rejection of experimental campus/Gather art does not apply to this supplied asset pack. Do not restore the discarded stash.

## Naming

Use domain names for production tables, columns, symbols, and new migration descriptions. Do not prefix them with release stages such as `mvp2_`. Migration 00005 renames the already-applied legacy table to `room_location_history`, including PK/FK names. Migration 00004 stays unchanged so existing deployments and ordered rollback remain valid.

## Asset ownership

`kelasverse-assets/` is reference-only. Copy needed assets/data into `classverse-frontend/src/assets/` and replace existing production implementations in place. Do not import or read the reference folder from application code, tests, or build scripts. Frontend must build and test without that folder. Keep original reference assets unchanged; do not copy unused prototypes wholesale.

## Not built yet (and the assets kept for it)

| Feature | Reference | Kept assets | Blocked on |
| --- | --- | --- | --- |
| Modul, Kuis, Rapor pages | `Room.dc.html`, `Quiz.dc.html`, header nav | none copied | persisted modules/quizzes/reports, permissions |
| Panel belajar: XP/level, bintang, beruntun, lencana, Lanjutkan belajar, Misi hari ini | `Main.dc.html` left panel | none copied | progress + missions service; never show prototype numbers |
| Papan Misi and Panggung on the Alun-alun | `Main.dc.html`, Kiko chat line | `src/assets/world/pending/papan-misi.svg`, `panggung.svg` | missions/events + positions in `world.json` |
| Campus decoration | `world.json` reference | `pending/fountain.svg`, `pending/flowers.svg` | positions + collision via `generate_world.py` |
| Leaderboard, stars, bell in header | `Main.dc.html` header | none | weekly points service |
| Mic/camera in action bar | `Main.dc.html` action bar | none | camera feature; voice already in `VoiceBar` |
| `Server 1 · N/500` capacity chip | `Main.dc.html` HUD | none | backend admission capacity + load test (campus is 200 today) |
| Host spot and seat snapping/pose | `rooms.json` `host`, `seats` | seats already in maps | host semantics decision |
| Walk frames / directions | README rule 9 | none | not designed yet; idle front pose only |

`pending/` sits outside `sprites/`, so these files are not bundled or loaded. Avatar preset SVGs
(`avatar-*.svg`, `avatar-template.svg`) were removed: avatars are composed from `assets/avatars/palette.json`, and
the reference README calls presets examples only.

## Delivery slices

- [x] Tokens and locally hosted Plus Jakarta Sans; keep Fredoka. Existing components temporarily consume aliases of `--kv-*` tokens.
- [x] New `00004_mvp2_world.sql` migration maps old keys, keeps exact old locations for Down, preserves room IDs/owners and `ruang-guru` restricted key. Applied to local development DB only. Up/Down/Up verified on disposable DB; never applied to production.
- [ ] Generate server/client map geometry from supplied JSON: campus 2200×1500, rooms 1000×640 plus walls, entrances and seats reachable. Preserve server-authoritative movement. Pixel dimensions, 85px proximity, circular voice zones must agree across both sides (32px tile rounding alone cannot meet these requirements).
- [ ] Replace campus art with supplied sprites, text labels, depth ordering and place cards. Camera/HUD slice done: campus follows player with default zoom 0.5/range 0.3–1.0 and accessible zoom/reset controls; minimap reads actual map buildings and sampled self position, no fake crowd. Interior overview retained. Visible key hints removed, accessible canvas instructions retained. Campus sprites now copied into frontend assets and rendered from local JSON, including Ruang Guru reusing the new Gedung Kelas art. Place cards implemented: click a map building or sidebar choice to inspect it, live occupancy only when available, walk-to-door CTA, room selection enabled at matching entrance. Selected building takes precedence over nearby unrelated buildings. Closing remains dismissed until leaving its entrance or selecting again; participant staff-only CTA disabled (backend authorization unchanged).
- [ ] Four interiors rendered: kelas, diskusi, lab, baca with local SVG furniture, procedural floors and circular voice-zone containment. Layouts exposed through API/room creation. Seat points exported/reachability checked and served to the frontend. Interior learning panel now offers numbered seats with current/occupied states from actual avatar feet; selecting a free seat walks through existing pathfinding, never teleports. Occupancy is advisory, not exclusive reservation. Automatic seat snapping/pose and host-position semantics remain pending. Prototype occupied seats/names are not rendered as fake live users.
- [x] Compose idle avatars from palette with independent skin (6), hair (5), clothing (10) choices. `look-<skin>-<hair>-<shirt>` keys validated in backend and OpenAPI, legacy keys preserved without account migration. Phaser loads composed SVG textures on demand instead of preloading all 300 combinations. Profile previews and live world match. 113 frontend tests/build/full lint, Go tests/lint and real-DB auth integration pass; Chrome verifies save, immediate map update, persistence on reload, restores test profile.
- [ ] Collapsible learning/chat panels implemented with real profile/building occupancy. Desktop header still needs the new destinations; map HUD/action bar refinements pending. Mobile remains usable and accessible while adapting the new visual language.
- [ ] MVP 2 learning modules, quiz, report, leaderboard, stars, missions and camera: connect real persisted data/permissions, never present prototype scores or fake online counts as production state. Emotes delivered as transient, server-validated room reactions (see checkpoint).
- [ ] Verify lint, types, unit/integration tests, migration up/down on disposable DB, browser flows and visual comparisons, keyboard/mobile accessibility and capacity.

## Current checkpoint

Campus and four interiors now use copied production assets; generator `classverse-frontend/scripts/generate_world.py` exports server `.tmj` geometry. Extra Ruang Guru placed between Rumah Angka and Menara Sains; restricted key retained. Shared exact world bounds, 85px proximity, circle containment, and per-place entry radii added to server/client. Existing protocol tests use fixed MVP1 testdata; new `revamp_test.go`, frontend map regression and generator BFS check cover new maps.

Local migration applied and backend restarted. Chrome browser verified no page errors, collapsible learning panel, zoom/reset, walking to migrated Rumah Angka, entering new classroom and exiting. Screenshots `/tmp/kv-e2e/mvp2-campus.png` and `mvp2-interior.png`. Go tests and lint, frontend tests/build/type-check/full lint pass (final recheck pending after small visual polish). Migration Up/Down/Up also passed on disposable PostgreSQL DB at port5433. No commits yet. Main feature expansion (modules/quiz/etc.) remains unfinished; do not claim full MVP2 complete.

## Left sidebar checkpoint

Per `Main.dc.html`, the left learning panel no longer duplicates the building list; building selection stays on the map and in **Daftar gedung**. Mockup XP, streak, badges, continue-learning and mission values are not copied because no persisted service exists. Chrome travel and seat regressions pass via the directory. Black flash after travel fixed: Phaser 3.90 parses `backgroundColor: 'transparent'` as black, visible while the remounted scene loads assets. `GameCanvas.vue` now uses `transparent: true`; unit test guards it, Chrome screenshot 150 ms after exit shows campus without black.

## Emote checkpoint

Emote action bar now sends four fixed reactions (`wave`, `hand`, `quiz`, `cheer`) over authenticated room WebSocket with documented `kind` field. Server validates active connection, muted state and 1/second rate; campus delivery uses area of interest, interiors use room members. Identity always comes from the server connection. No persistence, chat insertion, or replay. Client shows a three-second bubble above actual avatars, clears reactions on leave/travel/reconnect, and exposes accessible labeled buttons with cooldown. Mobile toolbar scrolls horizontally instead of overflowing its fixed-height rail.

117 frontend tests, build/type-check and full lint pass. Go tests/full lint and realtime race tests pass; new emote handler has 100% statement coverage. Chrome with two isolated authenticated users verifies sender and peer receive the same reaction and it expires; desktop and 390px mobile screenshots checked. Full campus-to-classroom browser script initially timed out waiting for entrance; final dedicated seat/travel rerun passes continuous walking to seat, current-seat state and classroom exit with no page errors. Learning persistence, host/seating semantics and other pending revamp items remain unfinished.

## Seat-selection checkpoint

`MapData.seats` parses served TMJ seat objects, rejects nonfinite/out-of-bounds/blocked points. `SeatPicker.vue` uses actual self/remote positions; current and occupied seats disabled, offline/travel/leave/kick also disabled. Browser checks selection does not teleport, reaches first classroom seat, updates current-seat state and exits normally with no page errors. Frontend tests/build/type-check/full lint pass. No server protocol or admission change; simultaneous arrivals can still share a seat, explicitly not a reservation feature.

## Place-card checkpoint

`PlaceCard.vue` replaces the plain door prompt. Chrome verifies participant cannot use Ruang Guru CTA, Rumah Angka card can walk to migrated entrance, select room, enter/exit classroom, no page errors. 112 frontend tests, build/type-check/full lint pass. Prototype progress percentages not displayed because there is no persisted learning-progress service yet.

## Movement regression fix

Browser profiling found live campus ground Graphics (thousands of dots) caused 400ms average frame gaps / 544ms max in headless Chrome. Ground now baked once to a texture and Graphics destroyed; interiors use the same path. Phaser `fps.smoothStep` disabled so low-FPS movement uses elapsed time, still bounded by existing MAX_FRAME_MS. Browser before: ~39px/3s; after: ~912px/3s, p95 ~49ms, zero page errors or move rejections. 110 frontend tests, build/type-check and full lint pass. Real physical-device/high-refresh verification remains open.

## Risks to carry forward

- Existing `rooms.building_key` includes `ruang-guru`; access checks also hardcode that key in room service. Renaming alone can disclose staff rooms.
- Existing proximity is integer `near_tiles`; supplied 85px cannot be represented at 32px tile size.
- Existing zones are rectangles on server and client; circles need shared containment tests, not a cosmetic ring only.
- Existing campus capacity is 200; `N/500` must not be displayed before backend admission capacity and load verification agree.
- Reference HTML depends on missing `support.js`; treat as visual reference, not runnable production components.
- Source CSS imports Google Fonts; production must use local fonts instead.

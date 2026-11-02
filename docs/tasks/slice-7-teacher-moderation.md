# Slice 7 — Teacher controls and moderation (US-10, US-11, US-12)

Status: C7-01…C7-08 approved. Broadcast, gather, session recovery, kick/allow, mute and lock done; reports and admin audit log remain.
Branch: `slice-7-teacher-moderation`, based on layout commit `35fb28e`.

References: docs/spec.md (teacher controls, security 3/5), docs/design.md
(WS cmd/event, moderation_actions, reports, API contract), docs/visual.md.

## Existing requirements

- Teacher/admin broadcast reaches the whole room.
- Gather moves all avatars to a gathering point in one action.
- Kick prevents entry to the same session until teacher permission.
- Lock rejects new participants.
- Reports reach organization teachers/admins; moderation actions are audited.
- Voice remains M2. Lock/mute are in-memory per-session state as documented.

## Decisions proposed for approval

| ID | Proposed behavior |
| --- | --- |
| C7-01 | M1 mute blocks participant chat in every scope; unmute restores it. Teacher/admin cannot mute or kick other teachers/admins or themselves. |
| C7-02 | Gather moves all connected avatars and grace avatars to the map's existing walkable spawn point; clears client paths/held keys and resets movement budgets. Campus gather is unavailable. |
| C7-03 | Broadcast uses filtered chat with scope broadcast, teacher/admin only, bypasses all_chat_enabled but retains length/rate limits; appears in every recipient's visible chat and a distinct announcement area. |
| C7-04 | Kick closes the target WebSocket, as existing design requires. Ban is keyed by room session and user; reconnect can enter campus but cannot enter the banned session. Add explicit teacher command allow to revoke the ban, with audit. |
| C7-05 | Lock/unlock affects participants entering classroom/garden; teachers/admins and an existing participant reconnecting within grace may enter. Campus cannot be locked. |
| C7-06 | Muted IDs, bans and lock survive empty-room actor cleanup for the remainder of the open session. Process restart must not bypass session bans: persist bans by session. This requires schema/design extension; do not silently treat room-actor lifetime as session lifetime. |
| C7-07 | Reports accept a target user, optional room/message, and reason 1–1000 code points. Server derives message sender/room/org from the stored message, validates organization access and target membership; no client-supplied chat copy is stored. Campus or unsaved chat can report user only. |
| C7-08 | Teacher/admin can list and review reports within organization; admin alone reads audit log. Audit writes must succeed before acknowledging moderation success; return unavailable on persistence failure. Report text is not written to application logs. |

## Progress

- Broadcast server authorization added: teacher/admin only; bypasses all_chat_enabled while reusing filtering, rate limiting and persistence.
- Frontend provides teacher announcement input, announcement summary, and copies into all chat tabs with bounded history.
- Backend make test (race) and make lint pass. Frontend lint, 74 tests and build pass; Phaser chunk warning remains.
- Gather implemented via cmd {name:gather} / event {name:gather,by,participants}: teacher/admin only, campus rejected, all members moved to SpawnPoint(0), stride budgets reset; client resets positions, interpolation, path and keys. Backend authorization/position tests added; full backend test/lint and frontend lint/unit/build pass.
- Browser verification and dedicated client gather tests pending. Remaining moderation features not yet implemented.

## Implementation order

1. Approve decisions, update design, OpenAPI and WS mirror types; add migrations and sqlc queries.
2. Broadcast vertical slice: authorization, filter, persistence, rendering, tests.
3. Gather vertical slice: authoritative position reset, client prediction/input reset, tests.
4. Mute/unmute, lock/unlock, kick/allow with audit and session lifetime tests.
5. Report create/list/review and admin audit log; organization isolation tests and UI.
6. Full backend race/integration tests and lint; frontend lint/unit/build; browser validation with teacher and two participants.
7. User review before commit; no automatic push.

## Verification priorities

- Participant forged cmd, cross-org targets and cross-org report IDs rejected.
- Broadcast still works with room chat disabled; muted participant cannot bypass via scope changes.
- Gather cannot leave stale client path/prediction moving avatar immediately afterward.
- Kick survives reconnect/re-entry and server restart until session ends or teacher allows.
- Lock preserves existing occupants and rejects new participants without leaking reservations.
- Report remains after 30-day message purge with null message_id.
- No tokens, chat text or report text in application logs.

## Restart lifecycle blocker

Existing startup calls rtStore.CloseOpenSessions, ending every previous process session.
This conflicts with C7-06 restart-safe bans: persisting a ban by session alone cannot preserve enforcement
if startup ends that session. User chose A: restore open sessions across restart with attendance/timeout recovery.
Startup now closes interrupted attendance and restores empty room actors with a fresh five-minute
idle deadline. Normal shutdown leaves sessions open. Realtime tests and lint pass;
full race/integration rerun and explicit recovery regression coverage remain pending.
Schema 00003_session_bans and org-scoped ban/revoke/check sqlc queries added, with PostgreSQL isolation tests.
Backend make test and lint pass. Queries are not wired into moderation commands/admission yet.

## Kick / allow (done)

- Migration 00003: `session_bans (session_id, user_id)` + `mod_action` value `allow`.
- `KickFromOpenSession` / `AllowInOpenSession`: ban and `moderation_actions` row in one SQL statement;
  zero rows unless the session is open, actor is teacher/admin, target is a participant of the same org.
- Room validates live state, then writes on the hub worker (never blocks the room loop). Target is removed and
  its socket closed (4403, error `kicked`) only after commit; DB failure answers `unavailable` and keeps the target.
- Admission reads `banned` for the open session; travel and reconnect-resume refuse it. Ended sessions never ban.
- Staff receive `event {name: kick|allow, by, target, banned[]}`; teacher panel lists participants (confirm dialog)
  and kicked users with Izinkan. Kicked user sees an overlay and returns to the campus.
- Tests: unit (success, audit failure fails closed, role/self/target rejections), PostgreSQL (org isolation,
  audit rows, ban survives RecoverSessions, allow, closed session), gateway (travel and resume refuse banned),
  frontend gather/event tests. Backend `make test` (race, 0 skipped) and `make lint` pass; frontend lint, 76 tests,
  build pass. Migration up/down/up verified locally.
- Chrome (teacher + participant): kick shows overlay, re-entry rejected `kicked`, allow lets the participant back in;
  audit has kick and allow; no runtime errors.

## Mute / lock (done)

- In-memory per room actor (design note: lives for the session); audit via `RecordRoomAction` (zero rows unless the
  session is open, actor is staff, mute target is a participant of the same org, lock has no target).
- Room validates, writes on the hub worker, applies only after commit; DB failure answers `unavailable` and changes nothing.
- Mute: every chat scope rejected with `muted` (broadcast stays staff-only). Receiver gets personal `room_state.muted`;
  staff get `event.muted[]`. Kick clears mute.
- Lock: join and travel reservation reject non-staff with `room_locked` without leaking capacity; staff may enter;
  occupants reconnecting within grace keep their seat. Campus cannot be muted/locked.
- `room_state` and `welcome` now carry `locked` and the receiver's `muted`; toggling Semua keeps both.
- Store implements SessionRecorder and Moderator with compile-time assertions (a missing method previously
  failed silently through the runtime type assertion).
- Tests: unit (mute all scopes/unmute, applied only after audit, staff/not-muted rejections, failure, lock join/reserve/
  staff/reconnect/double lock, campus), PostgreSQL scoping table. Backend make test (race, 0 skipped) and lint pass;
  frontend lint, 78 tests, build pass.
- Chrome (teacher, two participants): mute disables input, server still rejects chat sent through the raw socket,
  unmute restores; lock rejects newcomer (`room_locked`), occupant reload keeps seat, unlock lets newcomer in; no runtime errors.

## Carried issue

Movement after travel remains unresolved; see `map-chat-layout.md`.
User requested proceeding to slice 7 after committing layout. Do not describe this bug as fixed.

# Slice 9 — Voice (M2, US-09, US-10 voice, US-12 mute)

Status: **implemented; local verification complete; awaiting user review**.
Branch: `slice-9-voice`, based on `origin/main` `365aad7` (slice 8 merged).
User approved room/zone audio without distance attenuation, server-enforced isolation, confirmed eviction with visible failure handling, and **self-hosted LiveKit for the MVP**. Mobile voice remains deferred.

References: `docs/spec.md`, `docs/design.md` slice 9, `docs/visual.md`.

## Decisions and implementation

| ID | Behavior |
| --- | --- |
| V9-01 | Voice in classroom/garden only. Each actor incarnation has a random epoch; zone rooms `<room UUID>:<epoch>:zone:<zone key>` and room-wide broadcast `<room UUID>:<epoch>:broadcast`. Identity `<user UUID>:<admission UUID>`. New actor epochs prevent old media sessions hearing a restarted room. |
| V9-02 | Authenticated `POST /api/rooms/{room_id}/voice-token` requires live WS membership; returns `{url, token, broadcast_token, identity, channel}`. 404 for nonmembers/campus, 409 `voice_pending` during eviction, 503 `voice_unavailable` when disabled. Microphone-only HS256 grants, 2-minute TTL, no data/own metadata, no-store. TTL does not revoke active connections. |
| V9-03 | Full-volume audio within the current zone; outside all zones is `general`. LiveKit room grants enforce isolation; browser uses auto-subscription inside its admitted rooms, not a distance filter. Cross-zone authoritative movement waits for old media eviction. |
| V9-04 | Staff `voice_broadcast` / `voice_broadcast_off` rotates admission. Only active staff broadcasters can publish in the broadcast room; zone publish disabled while broadcasting. Leaving clears broadcast. |
| V9-05 | Audited mute/unmute and kick revoke old admissions. Worker removes both connections and confirms absence with `GetParticipant`. Room retains failed/queue-full jobs and retries each second. Staff sees pending eviction; no false success. Hub prevents new voice admissions during cleanup even after travel to another room. |
| V9-06 | Self-hosted LiveKit v1.9.12 for MVP. `LIVEKIT_URL` is private upstream. Public signaling goes through `FRONTEND_URL/voice` and Go. Direct public port 7880 is forbidden. Compose binds it to loopback; media 7881/TCP and 7882/UDP remain accessible. No new Go dependency. |
| V9-07 | Mic off on entry/travel; accessible mic toggle, heard-channel label, speaking ring, staff broadcast switch, autoplay fallback, inline mic permission errors. Mic intent retained over confirmed same-room channel changes, not mute or travel. |

## Security boundary

- Gateway exposes only `/voice/rtc`, `/voice/rtc/v1` and their validation routes. Never forwards RoomService.
- Signature, issuer, expiration, origin, live membership, actor admission identity/channel and publish permission checked on every join/reconnect, including refreshed LiveKit tokens. Alternate Authorization/Cookie headers stripped.
- Handshake and eviction share an admission lock: old in-flight handshakes finish before eviction/absence confirmation. Network I/O never runs on a room actor.
- Cross-zone movement holds the authoritative position until old audio is removed; failed transitions stay pending. Gather/seat use this gate too.
- `welcome`/`room_state`: personal `voice_identity`, `voice_channel`; staff receives `voice_pending` IDs. Empty identity means admission revoked/pending. No client may infer permissions from avatar coordinates.
- Existing media may stay connected during a control API outage. UI says **not confirmed**, retries after recovery, and prevents fresh admission. A network failure cannot be honestly reported as confirmed disconnection.
- MVP state lives in the single process. On restart, new actor epochs isolate fresh audio from old media rooms; old admissions cannot rejoin. Durable cross-process cleanup and HA are not implemented.

## Verification

- Backend `make test` (race), `make lint` pass; measured package coverage: voice **88.5%**, realtime **80.3%**. Voice tests cover token grants, invalid/admin/forged/expired tokens, old/foreign admissions, student broadcast restriction, WebSocket binary proxy/replay rejection, confirmation probe failures, mute after audit, transition hold/retry and full-queue retention.
- Frontend lint, **96 unit tests / 19 files**, and build pass. `useVoice.spec.ts` protects against reconnecting both voice rooms on unrelated chat updates; VoiceBar covers unconfirmed eviction warning. Removed obsolete client-side `audible` filter/tests.
- Chrome with fake microphones and real local LiveKit: **21/21 checks** in `/tmp/kv-e2e/voice.mjs`: mic off, general audio, table isolation, same table far corners, teacher broadcast on/off, mute/unmute, kick audio removal, old zone token after transition rejected, old zone and broadcast tokens after kick rejected. No page runtime errors; expected SDK reconnect errors during admission revocation are not counted as application failures.
- Attack probe `/tmp/kv-e2e/voice-attack.mjs`: modified client directly requests another zone's track SID; **no audio delivered**.
- Outage probe `/tmp/kv-e2e/voice-outage.mjs`: stop local LiveKit before kick; staff sees unconfirmed eviction, old token denied immediately, restarting LiveKit triggers retry and clears pending only after absence confirmation.
- Dev Compose checked: LiveKit signaling port bound to `127.0.0.1:7880`, not all interfaces.

Temporary browser scripts are outside the repository and use disposable local test accounts. They must not print raw token URLs or credentials.

## Review follow-up (commit 68f459e review, findings 1 and 2)

- **Finding 1, avatar froze during a pending eviction.** `voicePosition` rejected every move while the old admission was being removed, so a LiveKit outage froze the avatar and desynced client prediction. Now moves that stay in the old channel are always allowed. Cross-zone moves stay blocked until removal is confirmed, and get `move_rejected` with the authoritative position. If removal fails, the queued destination is dropped (client resynced) instead of teleporting later. If the avatar moved in the old zone meanwhile, the stale destination is skipped. Test `TestVoicePendingEvictionDoesNotFreezeAvatar`; it fails on the previous code.
- **Finding 2, untested lock ordering.** `TestRemoveWaitsForInFlightHandshake` holds an admitted handshake upstream and checks `Remove` cannot finish until it completes. Removing the gateway `RLock` makes it fail with a clear message (verified, then restored).
- Browser re-run against real local LiveKit: normal suite 21/21, outage suite 19/19, cross-zone attack 8/8 stable over 3 runs. Attack script fixed: it previously opened a duplicate-identity publisher that LiveKit evicted (flaky, 2 of 3 runs aborted before the security check); it now reads the real mic track SID via loopback dev RoomService.
- Browser outage movement verified with real Chrome and local LiveKit stopped via Docker. `/tmp/kv-e2e/voice-outage-walk.mjs` observes authoritative positions through the other browser's received WebSocket frames: cross-zone entry blocked, same-channel movement remains usable, pending voice status visible, rejected click path stops, movement and voice recover after restart. Five consecutive runs passed 11/11; final run with an explicit correction-count assertion passed 12/12. Final normal 21/21, attack 8/8, kick/outage 19/19 also passed.
- Final checks: backend `make test` (race) and `make lint` passed; voice coverage 88.5%, realtime 80.4%. Frontend lint, 97 tests across 19 files, and build passed; existing Phaser large-chunk warning remains.
- Browser testing exposed a frontend regression: clearing every `move_rejected` route also stopped successful cross-zone corrections, leaving avatars in intermediate zones. Successful corrections now carry optional `continue_path: true`; actual rejections clear the path through `movementRevision`. Backend and frontend tests protect this distinction. No speculative reconnect change: the earlier recovery timeout did not recur after this fix.

## Remaining limits

- Production VPS ingress/firewall/TLS/TURN and real-device audio quality not verified. Must keep port 7880 private and route all public signaling through Go; no alternate ingress bypass.
- No mobile/campus voice, recording, spatial panning, device picker, push-to-talk, or noise-suppression settings.
- Failed cleanup jobs retry in memory; shutdown/crash is not durable eviction confirmation. New room epochs prevent stale sessions from joining future actor audio.
- `livekit-client` remains a lazy ~148 kB gzip chunk; existing Phaser large-chunk warning remains.
- User authorized a local commit. Push and production deployment remain pending.

## Sources

- https://docs.livekit.io/reference/other/roomservice-api/
- https://raw.githubusercontent.com/livekit/livekit/v1.9.12/pkg/rtc/room.go
- https://raw.githubusercontent.com/livekit/livekit/v1.9.12/pkg/rtc/subscriptionmanager.go
- https://raw.githubusercontent.com/livekit/livekit/v1.9.12/pkg/service/roommanager.go

`UpdateSubscriptions` is not an access-control allowlist: client/admin requests share the subscription path; `canSubscribe: false` blocks both. Publisher-controlled subscription permission is not authoritative against a modified publisher. Separate rooms plus server admission are used instead.

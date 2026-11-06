# Slice 9 — Voice (M2, US-09, US-10 voice, US-12 mute)

Status: **done; awaiting user review**. V9-01…V9-07 approved (V9-03 revised by the user).
Branch: `slice-9-voice`, based on `origin/main` `365aad7` (slice 8 merged). Web only; mode HP voice deferred with phone polish.

References: docs/spec.md (US-09, US-10, US-12, M2 row, security 6 "voice tidak direkam", NFR bandwidth),
docs/design.md (ADR-006 LiveKit, architecture, `POST /api/rooms/{room_id}/voice-token`), docs/visual.md
(mic 52×52 in room toolbar, speaking ring `--color-speaking`).

## Existing requirements

- Volume falls with distance and is silent outside the radius; mic off on entry; clear mute button; speaking icon.
- Same zone shares voice regardless of distance. Teacher broadcast voice reaches the whole room. Teacher mute.
- Audio is relayed live only, never recorded. Go only issues tokens and calls LiveKit for mute/kick.

## Decisions proposed for approval

| ID | Proposed behavior |
| V9-01 | **Where.** Voice only in classroom/garden. Campus stays text-only in M2 (200 people, no session/moderation). One LiveKit room per Kelasverse room, name = room UUID, identity = user UUID, display name from profile. |
| V9-02 | **Token.** `POST /api/rooms/{room_id}/voice-token` issues a token only if the hub says the caller is currently a connected member of that room actor. So door, lock, ban, schedule and Ruang Guru are already enforced by WS admission. Grant: `roomJoin`, `canSubscribe`, `canPublishSources: [microphone]` (`canPublish` false while muted), no data/metadata, TTL 2 min (self-host has no revocation; the client fetches a token right before connecting and LiveKit refreshes it while connected). Response `{url, token}`. 404 when not in the room, 503 `voice_unavailable` when LiveKit is not configured. |
| V9-03 | **Who hears whom (revised by user: no distance volume).** Voice channel = the zone the avatar stands in, or "area umum" when outside every zone. A listener hears everyone in the same channel at full volume, plus active broadcasters (V9-04). Enforced by the client: `autoSubscribe: false`, subscribe only to tracks whose speaker is in the listener's channel (from server positions + map zones). Accepted risk: a modified client in the same room could subscribe to other channels of that room; all are admitted members of one moderated room and nothing is recorded. |
| V9-04 | **Teacher broadcast voice.** New `cmd {name: "voice_broadcast"}` / `"voice_broadcast_off"` (teacher/admin, room only). Room keeps the broadcaster set and adds `voice_broadcast: [ids]` to `room_state`/`welcome`. Not audited (not moderation). Cleared when the teacher leaves the room. |
| V9-05 | **Teacher mute covers voice.** Existing mute/unmute (C7-01) also sets LiveKit permission `can_publish=false/true` via `UpdateParticipant`, run on the hub worker after the audit commit; LiveKit failure is logged and does not undo the chat mute (the token endpoint also refuses publish while muted). Kick and leaving the room call `RemoveParticipant` (best effort, async), so an evicted client cannot keep listening. |
| V9-06 | **Config and dev.** `LIVEKIT_URL` (browser `ws(s)://`), `LIVEKIT_API_KEY`, `LIVEKIT_API_SECRET`. All empty means voice disabled (UI hides the mic). Dev: `livekit/livekit-server --dev` added to `compose.yaml` (port 7880, dev key pair in `.env.example`). Production self-host vs Cloud is decided at deploy. Backend: no new dependency; the LiveKit token is an HS256 JWT (existing `golang-jwt/jwt/v5`, claims as in `livekit/protocol/auth`) and RoomService is Twirp JSON over `net/http`. Frontend: `livekit-client`, loaded lazily only in classroom/garden. |
| V9-07 | **UI.** Mic button 52×52 round Tinta in the room toolbar above chat, `aria-label` "Nyalakan mikrofon"/"Matikan mikrofon", `aria-pressed`. Mic always off on entry and after travel. Speaking avatars get a `--color-speaking` ring on the map (LiveKit active speakers). Teacher toggle "Bicara ke semua ruang". Muted participant sees a disabled mic with the existing mute notice. Mic permission denied → inline message, chat unaffected. |

## Plan (after approval)

1. Design, OpenAPI (`voice-token`), `message.go`/`messages.ts` (`voice_broadcast` cmd, `RoomStateMsg.VoiceBroadcast`); compose + env.
2. Backend `internal/voice` (handler/service; token issuer + LiveKit client behind a consumer interface), hub presence check,
   mute/kick/leave hooks on the hub worker, broadcast set in the room. Tests: token denied when not present / campus /
   unconfigured, grant contents (publish off when muted, mic only, TTL), broadcast cmd authorization and state,
   mute calls UpdateParticipant after audit only, kick/leave call RemoveParticipant.
3. Frontend: `realtime/voice.ts` (pure `audible(world, map, selfZone, broadcasters) → Set<id>`, unit-tested),
   `useVoice` (connect/disconnect per room, subscribe/volume each 200 ms, mic toggle), mic + broadcast UI, speaking ring.
4. Checks: `make test && make lint`; `npm run lint && npm run test:unit -- --run && npm run build`.
5. Chrome with fake media (`--use-fake-device-for-media-stream`) against local LiveKit: two participants hear each other
   near, not far; same zone far apart hear; teacher broadcast heard by all; mute stops publishing; kick drops audio.
6. User review before commit.

## Deliberately out of scope

- Mode HP voice, campus voice, stereo panning/spatial WebAudio, push-to-talk, device picker, noise suppression settings.

## Result

- Backend: package `voice` (LiveKit token signer + RoomService Twirp client on `net/http`, handler/service with a
  `Presence` consumer interface), `Hub.VoiceMember`, `Hub.SetVoice` with a voice worker, mute → `UpdateParticipant`
  after audit, removal → `RemoveParticipant`, `voice_broadcast` set in the room, OpenAPI `voice-token`, config,
  compose `livekit`. Tests: token claims/TTL/mic only/no publish while muted, RoomService calls (paths, admin grant,
  `not_found` ok, 401 fails), handler (member, muted, not in room, bad id, disabled 503, error 500, no-store),
  realtime (mute follows audit, not on failure; kick/leave remove; broadcast state + welcome + cleared on leave;
  campus forbidden; VoiceMember campus/stranger/other room). Signature verified against real LiveKit (wrong secret → 401).
- Frontend: `voice.ts` (`channelAt`, `audible`), `useVoice` (lazy `livekit-client`, manual subscriptions, mic, speaking,
  autoplay), `VoiceBar.vue`, speaking ring in `CampusScene`, `roomApi.voiceToken`, mirrored types.
  Tests: `voice.spec.ts`, `VoiceBar.spec.ts`.
- Checks: `go test -race ./...` (12 packages ok), `make lint` 0 issues; frontend lint ok, 99 tests, build ok
  (`livekit-client` own chunk 148 kB gzip; Phaser chunk warning remains).
- Chrome (3 desktop users, fake mic, local LiveKit, `/tmp/kv-e2e/voice.mjs`), 18/18 pass: mic off on entry; area
  umum hears area umum; table separates from area umum; same table far corners hear each other; teacher outside does
  not hear the table; teacher broadcast heard at the table and stops when off; teacher mute disables the mic and
  stops audio, unmute restores; kick removes audio both ways; no runtime errors.

## Known limits

- Mode HP and campus have no voice. Channel enforcement is client-side (V9-03 accepted risk).
- Production LiveKit (TURN/TLS, self-host vs Cloud) not set up; dev compose only.

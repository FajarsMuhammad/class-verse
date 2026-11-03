# US-07 — Map and chat layout follow-up

Status: layout reviewed and accepted by user; approved for commit before slice 7.

- Home workspace expands to viewport; chat stays on the right (320 px).
- Building directory opens on demand; near-door card opens room choices.
- Travel still requires proximity to the door; no teleport.
- Narrow screens offer Peta / Chat views; full mobile mode remains slice 8.
- Frontend lint, 72 unit tests, and build passed. Phaser chunk-size warning remains.
- Browser automation for the revised chooser did not complete; user accepted appearance.
- Selected-door map highlight remains pending (destination text exists).

## Fixed: movement after travel

User reports delayed/choppy avatar movement after entering and leaving rooms with two users.
Refreshing restores smooth movement temporarily; repeating travel reproduces the symptom.
Root cause (confirmed): every travel remounts GameCanvas and creates a new Phaser.Game. Phaser's
destroy never calls WEBGL_lose_context, so each room left a live WebGL context behind. After ~15 trips
Chrome hit its 16-context limit ("Too many active WebGL contexts. Oldest context will be lost") and
started force-dropping contexts, which shows up as delays and choppy movement; reload cleared them.
Measured with /tmp/kv-e2e/stutter.mjs (9 rounds, two users): before the fix live contexts after GC grew
3→16 with the Chrome warning; after the fix exactly 1 live context every round, no warning. FPS 61, worst
frame 17 ms, 0 move_rejected, snapshots every ~100 ms in both cases (no server or network cause).
Fix: GameCanvas loses the context on Phaser's DESTROY event. Regression test GameCanvas.spec.ts fails
without the fix.

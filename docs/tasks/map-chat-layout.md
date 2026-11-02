# US-07 — Map and chat layout follow-up

Status: layout reviewed and accepted by user; approved for commit before slice 7.

- Home workspace expands to viewport; chat stays on the right (320 px).
- Building directory opens on demand; near-door card opens room choices.
- Travel still requires proximity to the door; no teleport.
- Narrow screens offer Peta / Chat views; full mobile mode remains slice 8.
- Frontend lint, 72 unit tests, and build passed. Phaser chunk-size warning remains.
- Browser automation for the revised chooser did not complete; user accepted appearance.
- Selected-door map highlight remains pending (destination text exists).

## Open bug: movement after travel

User reports delayed/choppy avatar movement after entering and leaving rooms with two users.
Refreshing restores smooth movement temporarily; repeating travel reproduces the symptom.
Root cause not confirmed. No movement fix is included in this commit.
Investigate Phaser lifecycle, frame timing, and move_rejected corrections across repeated travel.
Automated reproduction stalled at room selection; do not claim runtime verification passed.

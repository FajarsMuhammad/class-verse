# A1 — Class assignments and live-room learning pins

Status: user approved recipient freezing, independent zero-start ordered progress, and frozen module content/order.
Branch: `a1-assignments`, stacked on C1 `9b9d284`. Implemented; verification below records delivered scope.

## Existing behavior and invariants

- Modules remain a school-wide open catalog. M2 completion is free-order and keyed by school/user/step.
- Q1 passing a quiz completes all linked active quiz steps in published modules. Reusing this completion as
  assignment progress would allow learners to bypass a required sequence through the open catalog.
- Module authoring edits published content in place; step IDs survive archives. Quiz attempts snapshot questions
  and grading threshold at start, not when a module links the quiz.
- C1 records historical class enrollments; moving closes one enrollment and opens another. Active school year
  is an admin choice, not inferred from calendar or avatar location.
- Room sessions survive process restarts; an empty interior session closes after five minutes. Teacher/admin
  room commands already run through the room actor and must not block it on database work.
- Existing `internal/report` is moderation reporting, not academic Rapor.

## Approved decisions

1. **Recipients frozen at assignment creation.** Assign to students currently in the selected class. A class
   transfer does not delete or reattribute their assignment/results. New arrivals are not added automatically;
   teacher can create another assignment. Reject an empty recipient roster. Assigning to the same class/module
   again deliberately creates another task, not an overwrite. Persist a client-generated request ID so a network
   retry cannot create duplicates.
2. **Assignment progress independent of catalog progress.** Start assignment at zero, retain M2 progress intact.
   Require prior assignment steps to be complete before opening/completing the next assignment step. Plain-text,
   file and video steps still use explicit self-reported completion, not a claim of measured consumption. Quiz
   steps require a passing attempt started through that assignment after its preceding steps are complete; an
   older or unrelated catalog pass does not finish a locked task. Assignment attempts retain Q1 grading rules,
   immutable results, answer-key secrecy and unlimited retries. Completed assignment steps are monotonic; no undo
   in A1, so later steps cannot remain completed behind a newly locked predecessor.
3. **Freeze assigned learning content and order.** Snapshot the published module's active steps at creation:
   title/plain text, file reference, video ID, quiz reference, and order. Editing/reordering/archiving catalog
   content later does not rewrite an issued task. Referenced files remain protected from deletion. Quiz questions
   still snapshot at attempt start using existing Q1 rule; this proposal does not silently change Q1 into an
   assignment-time quiz revision system. If linked quiz is unavailable, block starting it with a clear error;
   retain prior results. Teacher cancels task and issues a replacement when content must change.

User approved all three decisions before implementation.

## Remaining narrow defaults

- Only teacher/admin creates/cancels assignments and reads their school-wide recipient progress. Participants
  read and act only on their own assignments. Foreign school/class/module/user IDs return 404.
- Only a published, nonarchived module with at least one active step can be assigned. Class/year/module IDs are
  scoped by composite school foreign keys. No new account or role tables.
- Optional deadline: input explicitly labelled WIB, store UTC. Deadline labels overdue work but never locks it
  or deducts grades. No reminders, points, late penalties, CSV or Rapor page in A1.
- Cancellation is soft: stop new work, keep recipients, snapshots, attempts and progress for history.
- A single pin per interior room session; any current teacher/admin there can replace/remove it. Accept a
  published module with optional active step in the same school. No campus pin, no class-membership gate.
- Pin broadcasts room state and appears to later joiners/reconnects. Preserve it across process restarts within
  the same session; clear when that session ends. Recheck publication/readability when opening a pin. A pin is
  a link to current catalog content, not an assignment snapshot or an automatic task completion.
- "Buka bersama" is an explicit button, never forced navigation, autoplay or a claim of synchronized playback.
  Desktop and phone list mode both receive it. Leaving the room clears that room's pin from the client.

## Implementation slices (delivered on one A1 branch)

1. Add tests for recipient freezing, duplicate retries, tenant/role isolation and historical independence.
   Migration after 00014: assignment, recipient, snapshot step and assignment-progress records, composite school
   foreign keys, request-id uniqueness; sqlc queries/service with short transactions. Serialize issuance against
   roster changes so recipients reflect one consistent roster, not a mixture during concurrent transfers.
2. Add own-task read/ordered completion APIs and assignment-aware Q1 attempt ownership/context. Verify file
   read authorization and deletion references for snapshots; neither free-order M2 nor Q1 catalog behavior changes.
3. Add teacher assignment form/progress and student Tugas panel using existing module/file/video/quiz readers.
   No false completion on navigation. Show deadline/cancellation/unavailable content and retry states accessibly.
4. Extend existing session/actor state for pin validation, persistence, broadcast, welcome/reconnect and expiry.
   Wire teacher picker and participant Buka bersama on desktop and phone without changing movement/voice.
5. Generate OpenAPI/Go/TS types, update source docs, review connected code, run verification and commit explicit
   A1 files. Do not push. Do not stage unrelated `docs/screens/sidemenu.png`.

## Acceptance evidence

- Postgres integration with `-race`: tenant/role/recipient ownership, snapshot after author edits/archive,
  recipient class transfer/new arrival, concurrent issuance/retry/completion, no out-of-order writes or quiz
  bypass through catalog, rollback on failure, independent repeated assignments, cancellation and file references.
- Session/WS tests: nonstaff/campus rejection, wrong module-step pairing, unpin, late join, store restoration
  with a fresh Store, session close/reopen. Existing actor tests cover empty-session closure. No DB I/O in room actor.
  Browser WS test travels to a new interior and receives its pin on late join.
- UI tests: assignment form/deadline, ordered steps, errors preserve server truth, no implicit completion,
  keyboard/focus and phone layouts. Real Chrome teacher/student/outsider/class-transfer flows and live pin.
- Disposable migration Up/Down/Up; never rollback operator DB. Full backend race/lint, frontend tests/lint/build,
  existing quiz/module/progress/library/map regressions. Report measured results only.

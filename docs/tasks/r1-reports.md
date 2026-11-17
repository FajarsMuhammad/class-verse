# R1 — Rapor belajar

Branch `r1-reports`, stacked on A1 `c48f4cc`. Read-only slice: no migration, grade rewrites, points, or fake Arena data.

## Behavior and invariants

- Participants read only their own school report; teacher/admin read participant roster and individual reports
  in that school. Membership is checked on every JSON/CSV request. Foreign school/year/class/content/student IDs
  return 404; participant requesting another student's data is forbidden.
- Year/class filter selects **current enrollment in that selected academic year**, not a date range for scores.
  Year labels have no start/end dates. UI explicitly labels this roster meaning; never reattribute prior results
  to a new class. Assignment class/year remains its issuance snapshot after transfer.
- Catalog module progress is M2, active steps only. Archived content with saved activity remains in individual
  history, labelled archived; published modules with no activity show not started. No fabricated consumption proof.
- Quiz last = latest finished attempt ordered by finished_at/id; best = highest finished score. An unattempted quiz
  has null score, never 0. Mean = mean of best scores of attempted quizzes (one weight per quiz), not points and
  not a mean treating unattempted quizzes as zero. All recorded contexts count toward quiz score history; ordered
  assignment completion stays independent of catalog completion.
- Assignment report uses frozen A1 recipients/title/class/year and independent done/total, cancellation/deadline;
  separate from catalog progress. No aggregate academic grade invented for modules/assignments.
- Staff roster supports school/year/class/building/module/quiz filters and pagination. Content filters must
  belong to school; combined module/quiz filters refer to their respective metrics, not infer participation.
  CSV exports the same filtered roster summaries, not hidden attempts/answer keys or email/birth date.
- CSV uses stdlib encoding/csv, UTF-8, neutralizes formula prefixes including leading whitespace/control chars.
  Maximum 10,000 students per export; fail explicitly instead of silently truncating. JSON roster pages max 50.
- Student report lists catalog modules, quizzes and issued assignments per building. History Arena explicitly
  unavailable until Q2. UI route `/rapor` in AppShell works desktop/390px with labelled filters/loading/error/retry.

## Verification plan

1. Failing real-Postgres service tests for permissions, current/year/class roster vs frozen assignment history,
   best/last/null/average, module completion/archival and deterministic pagination.
2. sqlc aggregation queries and `internal/gradebook` service/HTTP (existing `internal/report` is moderation).
3. OpenAPI + generated Go/TS, Rapor route/view, current-school nav, same filter JSON/CSV.
4. CSV injection unit cases, HTTP own/other/org/filter/CSV tests; Vue tests for filters/null scores/errors/download.
5. Full backend race/lint, frontend tests/lint/build; Chrome teacher/student/390px and existing learning regressions.
   Commit R1 files only, no push unless requested. Unrelated `docs/screens/sidemenu.png` stays untouched.

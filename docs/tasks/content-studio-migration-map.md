# CS migration map and rollout

CS0 mapping, 2026-11-18. Skills: incremental-implementation, golang-database. Repository-approved migration contract overrides community skill's prohibition on schema changes. Memex unavailable in worker tool allowlist.

## Ownership and transition

- `modules`/`module_steps` and `quizzes`/`quiz_questions`: existing author working rows; IDs remain stable. New published snapshots owned by same feature packages, not course/enrollment.
- Module catalog sends contents in list response (no existing detail endpoint). Reading catalog contents establishes user's revision pin; listing draft author contents does not establish learner progress. Pin alone is not completion.
- `module_progress` preserves original step IDs/completion timestamps; pin selects revision's stable step set. Resume and gradebook must use pinned step set, not mutable draft.
- `quiz_attempts.questions`, score/pass_score remain authoritative. Module context binds quiz revision server-side. Standalone uses latest published revision; assignment context uses issued revision, never current module pin. Existing open attempts retain snapshot.
- Assignment issuance copies published module step snapshot; existing assignment rows remain own snapshot. Rich content and quiz revision references must survive subsequent draft edits.
- File authorization checks published snapshot references (latest or caller's pin), never author draft. Retention protects every historical published reference and assignment snapshot. File locking before snapshot creation must serialize against deletion.
- Learning pins must validate published step sets rather than working rows; no bypass of membership/module archive.
- Gradebook reads historical pinned revisions; attempts are not regraded. G1 routes remain outside academic RequireUser endpoints.

## Expand / backfill / verify

Migrations `00020`/`00021` additive; 00020 immutable after local apply; 00001–00019 unchanged. Backfill published quizzes first, then published modules with quiz binding. Legacy text becomes literal rich-text paragraphs; HTML is not interpreted. Backfill module pins only for users with existing progress; retain all attempts, assignments and stable IDs. New saves carry optimistic `expected_version`; stale edits receive structured conflict. Published snapshot rows immutable.

Deploy API and frontend together after migration. Old binary must not keep writing after cutover: old writer does not create snapshots or validate optimistic versions. Maintenance window required; additive schema alone does not make old writers safe. No dual-authoring compatibility promise. Verify migrations, historical progress, file retention and endpoint guest rejection before reopening authoring.

## Rollback

Disposable empty database: Up/Down/Up supported. Down must fail closed once versioned content exists; do not drop academic data. Production rollback: stop writes, retain database, restore backed-up binary/database pair only under reviewed backup runbook. Never downgrade by deleting revisions or results. No destructive contraction authorized.

## Editor evaluation

GitHub CLI unavailable in parent (`gh`, exit 127). Direct npm registry, official https://tiptap.dev/docs/editor/getting-started/install/vue3 and GitHub API https://api.github.com/repos/ueberdosis/tiptap checked. Tiptap Vue 3 v3.34.0: MIT, repository not archived, active upstream. Vue adapter + ProseMirror schema/history/paste reuse preferred over hand-rolled contenteditable. Only allowlisted extensions used; no raw HTML storage or execution. Build passes; Vite reports >500 kB chunks (LibraryManager, GameCanvas).

## Evidence

Implemented locally, not deployed. `00020` restored to exact applied content; hardening in `00021`. Canonical upgrade `make migrate`: goose to 20, idempotent `db/upgrades/content_studio_preflight.sql`, then 21. Direct `goose up` across 20→21 unsupported. Disposable proofs: fresh19→20→preflight→21, existing20→21, post-21 preflight retry, fail-closed Down. Local runtime 20→21 applied with ownership preflight; no manual schema SQL or deletion. Browser E2E and G1 regression passed.

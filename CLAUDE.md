# Kelasverse

Ruang belajar virtual 2D (mirip Gather Town) untuk SMA dan kampus. Peserta muncul di peta sekolah,
berjalan ke gedung, lalu masuk kelas; ngobrol lewat proximity chat (M1) dan voice (M2).

Sumber kebenaran produk dan teknis:

- `docs/spec.md` — scope MVP, user stories (US-xx), aturan keamanan, NFR.
- `docs/design.md` — arsitektur, struktur package, protokol WebSocket, auth, skema SQL, API contract, ADR.
- `docs/visual.md` + `docs/screens/*.png` — arah visual, alur layar, spesifikasi komponen, aturan aksesibilitas.
  Lihat screenshot layar terkait sebelum mengerjakan UI.

Jika kode dan dokumen bertentangan, berhenti dan tanyakan; jangan mengubah keputusan di ADR diam-diam.

## Stack

- Backend: Go (satu binary, modular monolith), `chi`, `coder/websocket`, PostgreSQL 16, `pgx/v5`, `sqlc`, `goose`, `log/slog`.
- Frontend: Vue 3 + Vite + TypeScript + Pinia, Phaser 3 untuk peta, map dari Tiled (`.tmj`).
- Dev lokal: Docker Compose untuk Postgres dan Mailpit (SMTP untuk OTP).
- Voice (M2): LiveKit. Deploy pilot: satu VPS, Docker Compose + Caddy.

## Struktur

Monorepo dengan dua project yang berdiri sendiri, agar nanti bisa dipisah menjadi repo terpisah:

```
docs/                        dokumen produk & teknis bersama (spec, design, visual, tasks)
CLAUDE.md                    instruksi ini

classverse-backend/          Go module github.com/FajarsMuhammad/class-verse/classverse-backend
    cmd/server/              wiring: config, db, router, hub; tidak ada logika bisnis
    internal/<fitur>/        auth, org, invite, room, realtime, moderation, voice
        handler.go           HTTP/WS: decode, validasi bentuk input, panggil service, encode
        service.go           aturan bisnis dan otorisasi
        store.go             akses database lewat query sqlc
    internal/platform/       config, db, httpx, logx — hanya hal lintas fitur yang kecil
    internal/platform/db/query/  hasil generate sqlc (jangan diedit)
    db/migrations/           goose, SQL murni
    db/queries/              query sqlc per fitur (users.sql, orgs.sql, ...)
    maps/                    file Tiled
    compose.yaml, Makefile, .env.example, sqlc.yaml, .golangci.yml

classverse-frontend/         Vue 3 + Vite + Phaser
    src/styles/tokens.css    design token
    src/assets/fonts/        font self-hosted
    src/assets/avatars/      avatar preset
    src/realtime/messages.ts cermin tipe pesan WebSocket
```

Aturan:

- Kedua project tidak saling mengimpor file; kontraknya hanya HTTP API, protokol WebSocket, dan `docs/`.
  Setiap project punya `README.md`, `.gitignore`, dan konfigurasinya sendiri.

- Package-by-feature. Fitur tidak mengimpor `store` fitur lain; pakai interface yang didefinisikan di sisi pemakai.
- Nama package satu kata, sesuai domain. Jangan buat package bernama `infra`, `common`, `utils`, atau `helpers`.
- Tidak ada ORM. Semua query lewat `sqlc`; generated code jangan diedit manual.
- Tidak ada global state selain yang dirakit di `cmd/server`. Dependensi lewat constructor (`NewService(store, clock, log)`).

## Konvensi kode Go

- Sederhana dulu: tanpa abstraksi sampai ada pemakai kedua.
- Error dibungkus dengan konteks: `fmt.Errorf("create invite: %w", err)`. Error domain didefinisikan sebagai sentinel atau tipe di package fiturnya.
- Handler mengembalikan `error`; konversi ke respons HTTP hanya lewat `httpx` (satu bentuk error JSON, lihat `docs/design.md` bagian API contract).
- `context.Context` selalu parameter pertama untuk operasi I/O.
- Logging hanya `slog`, structured, tanpa data pribadi (email, isi chat) di log.
- Waktu selalu UTC; untuk logika yang bergantung waktu, injeksi `clock` agar bisa dites.
- Test: table-driven untuk service, integration test store dengan Postgres asli (testcontainers atau database dari compose). Jalankan `make test` dan `make lint` (backend) serta `npm run lint`, `npm run test:unit -- --run`, `npm run build` (frontend) sebelum menyatakan selesai.

## Konvensi frontend

- Semua warna, font, ukuran teks, radius, dan spacing diambil dari `classverse-frontend/src/styles/tokens.css`. Jangan menulis hex atau ukuran font langsung di komponen.
- Font di-host sendiri dari `classverse-frontend/src/assets/fonts/`; jangan memuat Google Fonts.
- Avatar preset di `classverse-frontend/src/assets/avatars/`. UI shell (lobby, panel, chat) di Vue; peta dan avatar yang bergerak di Phaser.
- Elemen interaktif minimal 44×44 px, pakai `<button>`/`<a>`/`<input>` asli, ikon-only wajib `aria-label`.

## Keamanan (wajib, lihat spec bagian Keamanan)

- Semua data dibatasi per organisasi: setiap query yang menyentuh data organisasi wajib memfilter `org_id` dan memeriksa membership.
- Access token JWT Ed25519 15 menit, claim minimal (`sub`, `iat`, `exp`, `jti`), role tidak di token.
- Refresh token opaque, disimpan sebagai hash SHA-256, rotasi dengan deteksi reuse per family, di cookie `HttpOnly; Secure; SameSite=Strict; Path=/api/auth`.
- Tidak ada DM antar peserta. Voice tidak direkam. Chat dihapus setelah 30 hari.
- Jangan pernah log token, password, OTP, atau isi chat.

## Real-time

- Server-authoritative. Satu goroutine per room (inbox channel + ticker 100 ms), state room tanpa mutex.
- Satu koneksi WebSocket per user sepanjang sesi; pindah room lewat pesan `travel`.
- Envelope pesan: `{"t": "...", "seq": n, "d": {...}}`. Tipe pesan didefinisikan sekali di `classverse-backend/internal/realtime/message.go` dan dicerminkan di `classverse-frontend/src/realtime/messages.ts`.

## Perintah

Backend, dijalankan dari `classverse-backend/`:

```
make up          # jalankan Postgres + Mailpit
make down        # hentikan
make migrate     # goose up
make sqlc        # generate query
make run         # jalankan server
make test        # semua test
make lint        # golangci-lint
make migrate-down              # rollback satu migration
make migrate-new name=add_foo  # buat migration baru
```

Frontend, dijalankan dari `classverse-frontend/` (Vite mem-proxy `/api` dan `/ws` ke `localhost:8080`):

```
npm install
npm run dev        # dev server di localhost:5173
npm run test:unit  # vitest
npm run lint
npm run build
```

## Skills

Semua skill project ada di `.agents/skills/` (file asli, di-commit, dibaca juga oleh agent lain) dan tercatat di
`skills-lock.json`. `.claude/skills/` hanya berisi symlink relatif ke sana agar terbaca Claude Code.

- Tambah skill: `npx skills add <owner/repo> -a codex -y -s <skill>` (target `codex` menulis ke `.agents/skills`
  dan lock), lalu `ln -s ../../.agents/skills/<skill> .claude/skills/<skill>`.
- Hapus skill: `npx skills remove -y -s <skill>`, lalu hapus symlink-nya di `.claude/skills/`.
- Update: `npx skills update -p -y`.

- Sebelum mengerjakan tugas, cek skill yang relevan dan pakai; sebutkan skill yang dipakai di rencana singkat.
- Aturan di file ini dan di `docs/` lebih tinggi dari isi skill. Jika skill bertentangan dengan dokumen
  (misalnya menyarankan library, struktur, atau alur lain), ikuti dokumen dan sebutkan konfliknya.
- Paket superpowers hanya terpasang `using-superpowers`. Ganti `superpowers:brainstorming` dengan
  `spec-driven-development` / `planning-and-task-breakdown`, dan `superpowers:systematic-debugging` dengan
  `debugging-and-error-recovery`.

| Situasi | Skill |
| --- | --- |
| Mulai slice atau story baru | `spec-driven-development`, `planning-and-task-breakdown`, lalu `incremental-implementation` |
| Pakai library atau API eksternal (chi, pgx, goose, sqlc, coder/websocket, LiveKit, Phaser, Vite) | `source-driven-development`, `golang-popular-libraries`, `golang-pkg-go-dev` |
| Kode Go | `golang-code-style`, `golang-naming`, `golang-error-handling`, `golang-context`, `golang-structs-interfaces` |
| Database, migration, query sqlc | `golang-database` |
| Realtime (room goroutine, hub, WebSocket) | `golang-concurrency`, `golang-safety`, `golang-performance` |
| Test Go | `golang-testing` |
| Lint, modernisasi, refactor Go | `golang-lint`, `golang-modernize`, `golang-refactoring` |
| Endpoint REST, OpenAPI, protokol WS | `api-and-interface-design` |
| Auth, token, OTP, input pengguna, moderasi | `security-and-hardening`, `golang-security` |
| Logging dan metrik | `observability-and-instrumentation`, `golang-observability` |
| UI Vue | `frontend-design`, `frontend-ui-engineering` (tetap patuhi `tokens.css` dan `docs/visual.md`) |
| Cek tampilan atau alur di browser | `browser-testing-with-devtools` |
| Bug atau test gagal | `debugging-and-error-recovery`, `golang-troubleshooting` |
| Keputusan desain baru, ADR, README | `documentation-and-adrs` |
| Branch, commit, PR | `git-workflow-and-versioning` |

## Cara kerja

- Kerjakan satu user story atau satu tugas per branch; sebut ID-nya di commit (`US-03: create invite`).
- Sebelum menulis kode untuk story baru, baca bagian terkait di `docs/` dan sebutkan rencana singkat.
- Jika ada keputusan desain baru yang belum ada di `docs/design.md`, usulkan dulu, jangan langsung diterapkan.
- Bahasa UI: Bahasa Indonesia. Identifier, komentar kode, dan pesan commit: English.

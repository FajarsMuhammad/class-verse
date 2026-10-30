# Fase 3 — Setup project

Status: **selesai** (Oct 28, 2026, branch `phase-3-setup`). Lihat bagian *Revisi saat implementasi* di bawah
untuk hal yang berbeda dari rencana awal.

Baca `CLAUDE.md`, `docs/spec.md`, dan `docs/design.md` dulu. Tugas ini hanya menyiapkan kerangka repo;
belum ada fitur (auth, org, realtime dikerjakan di Fase 4).

Repo berbentuk monorepo dengan dua project yang berdiri sendiri (lihat ADR-010 di `docs/design.md`):
`classverse-backend/` (Go) dan `classverse-frontend/` (Vue). Semua path backend di bawah relatif terhadap
`classverse-backend/`, semua path frontend relatif terhadap `classverse-frontend/`.

## Tujuan

Backend bisa dijalankan dari nol dengan (dari `classverse-backend/`):

```
cp .env.example .env
make up && make migrate && make sqlc && make run
curl localhost:8080/healthz   # {"status":"ok"}
curl localhost:8080/readyz    # {"status":"ok"} jika database terhubung
```

dan `make test` serta `make lint` lulus.

## Yang dibuat

1. **Go module**
   - `go.mod` dengan module path `github.com/FajarsMuhammad/class-verse/classverse-backend`.
   - Versi Go stabil terbaru yang terpasang di mesin ini (1.26.2).
   - Tool `goose` dan `sqlc` didaftarkan lewat directive `tool` di `go.mod` (`go get -tool ...`), lalu dipanggil dengan `go tool goose` / `go tool sqlc`.

2. **`cmd/server`**
   - `main.go`: `run() error` dengan `signal.NotifyContext` (SIGINT, SIGTERM), load config, buat logger, koneksi pool, router, `http.Server` dengan `ReadHeaderTimeout`, graceful shutdown 10 detik.
   - Jangan set `WriteTimeout` global karena nanti ada WebSocket; batasi per handler.
   - `router.go`: chi dengan middleware RequestID, client IP (`ClientIPFromXFF`, lihat revisi), Recoverer, request logging via slog; route `GET /healthz` dan `GET /readyz` (ping database).

3. **`internal/platform`**
   - `config`: baca environment variable tanpa library tambahan; kumpulkan semua variabel wajib yang kosong dan kembalikan dalam satu error. Hanya variabel yang dipakai sekarang: `APP_ENV`, `HTTP_ADDR`, `DATABASE_URL`, `FRONTEND_URL`, `SMTP_ADDR`, `SMTP_FROM`.
   - `logx`: slog JSON di production, text di development.
   - `db`: `Connect(ctx, url)` mengembalikan `*pgxpool.Pool` yang sudah di-ping, dengan konfigurasi pool yang masuk akal.
   - `httpx`: tipe `Error{Status, Code, Message, Fields}`, helper `JSON`, `WriteError`, dan adaptor `Handle(log, func(w, r) error) http.HandlerFunc` sehingga semua handler memakai alur error yang sama (bentuk JSON sesuai `docs/design.md`, bagian API contract). Error tak dikenal → 500 `internal_error` dan di-log.
   - Unit test untuk `config` dan `httpx`.

4. **Database**
   - `db/migrations/00001_init.sql`: salin skema persis dari `docs/design.md` (termasuk `building_key` dan `rooms_campus_uq`), lengkap dengan bagian `-- +goose Down` yang membalik semuanya.
   - `sqlc.yaml` (v2, engine postgresql, `sql_package: pgx/v5`), output ke `internal/platform/db/query`.
   - `db/queries/users.sql` berisi `CreateUser` dan `GetUserByEmail` sebagai contoh, supaya `make sqlc` menghasilkan kode.

5. **Docker Compose (dev saja)** — `classverse-backend/compose.yaml`
   - `postgres:16-alpine` dengan healthcheck, volume bernama, port 5432.
   - `axllent/mailpit` untuk SMTP (1025) dan UI (8025).
   - Server Go dijalankan langsung di host lewat `make run`, bukan di container.

6. **Makefile** dengan target dari `CLAUDE.md`, plus `migrate-down` dan `migrate-new name=...`. `DATABASE_URL` dibaca dari `.env`.

7. **Lain-lain**
   - `.env.example` dengan nilai dev yang bekerja dengan compose.
   - `.gitignore` per project (backend: binary, `.env`; frontend: `node_modules`, `dist`).
   - `.golangci.yml` (format v2), linter standar ditambah `errorlint`, `gocritic`, `revive`.
   - Folder `maps/` dan `internal/{auth,org,invite,room,realtime,moderation}` belum perlu dibuat; buat saat fiturnya dikerjakan.

8. **Frontend** — `classverse-frontend/`
   - Dibuat dengan `npm create vue@latest` (TypeScript, Router, Pinia, Vitest, ESLint, Prettier), lalu `npm i phaser@^3`.
   - Vite dev server mem-proxy `/api`, `/ws` (dengan `ws: true`), dan `/healthz` ke `localhost:8080`, supaya frontend dan backend satu origin (dibutuhkan cookie `SameSite=Strict`; tidak perlu CORS).
   - Pindahkan aset desain dari folder `design/` (lalu hapus folder itu):
     `design/tokens.css` → `src/styles/tokens.css`, `design/fonts/*` → `src/assets/fonts/`,
     `design/avatars/*` → `src/assets/avatars/`. Import `tokens.css` sekali di `main.ts` dan buang CSS bawaan template.
   - Halaman awal cukup menampilkan status dari `/healthz`, memakai token (latar Kabut, judul Fredoka, satu avatar preset)
     sebagai bukti token dan font termuat. Bandingkan dengan `docs/screens/01-style-guide.png`.

9. **README.md**: satu di root (gambaran monorepo + mulai cepat) dan satu per project (prasyarat, langkah menjalankan dari nol, daftar target Makefile / perintah npm).

## Selesai jika

- [x] Langkah di bagian Tujuan berjalan mulus dari clone bersih.
- [x] `make test` dan `make lint` lulus.
- [x] `npm run dev` di `classverse-frontend/` menampilkan status backend.
- [x] Semua file mengikuti konvensi di `CLAUDE.md`.

## Revisi saat implementasi

| Rencana awal | Yang diterapkan | Alasan |
| --- | --- | --- |
| Satu repo datar: `cmd/`, `internal/`, `web/` di root | Monorepo `classverse-backend/` + `classverse-frontend/`; `docs/` dan `CLAUDE.md` tetap di root | Keputusan pemilik produk agar mudah dipisah jadi dua repo (ADR-010) |
| Module `github.com/<user>/kelasverse` | `github.com/FajarsMuhammad/class-verse/classverse-backend` | Sesuai repo GitHub dan folder di monorepo |
| Middleware `chi` `RealIP` | `middleware.ClientIPFromXFF` dengan prefix loopback/privat | `RealIP` deprecated di chi v5.5.0 karena rentan spoofing IP |
| `npm i phaser` | `phaser@^3` (3.90.0) | Versi terbaru npm adalah Phaser 4; stack di dokumen menetapkan Phaser 3 |
| Proxy Vite `/api` dan `/ws` | Ditambah `/healthz` | Halaman awal membaca `/healthz` lewat origin yang sama |
| Port compose tetap 5432/1025/8025 | Default sama, bisa diubah lewat `POSTGRES_PORT`, `MAILPIT_SMTP_PORT`, `MAILPIT_UI_PORT` di `.env` | Port sering sudah dipakai service lain di mesin developer |

Versi yang dipakai: Go 1.26.2, chi v5.5.0, pgx v5.11.0, goose v3, sqlc (tool di `go.mod`), Mailpit v1.32,
golangci-lint 2.6.1, Vue 3.5, Vite 8, Pinia, Phaser 3.90.0. npm 10.9 bisa crash saat me-resolve dependency baru;
pakai `npx npm@11 install <paket>` untuk menambah paket (lihat README frontend).

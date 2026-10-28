# Fase 3 — Setup project

Baca `CLAUDE.md`, `docs/spec.md`, dan `docs/design.md` dulu. Tugas ini hanya menyiapkan kerangka repo;
belum ada fitur (auth, org, realtime dikerjakan di Fase 4).

## Tujuan

Repo bisa dijalankan dari nol dengan:

```
cp .env.example .env
make up && make migrate && make sqlc && make run
curl localhost:8080/healthz   # {"status":"ok"}
curl localhost:8080/readyz    # {"status":"ok"} jika database terhubung
```

dan `make test` serta `make lint` lulus.

## Yang dibuat

1. **Go module**
   - `go.mod` dengan module path yang saya tentukan: `github.com/<user>/kelasverse` (tanyakan jika belum jelas).
   - Versi Go stabil terbaru yang terpasang di mesin ini.
   - Tool `goose` dan `sqlc` didaftarkan lewat directive `tool` di `go.mod` (`go get -tool ...`), lalu dipanggil dengan `go tool goose` / `go tool sqlc`.

2. **`cmd/server`**
   - `main.go`: `run() error` dengan `signal.NotifyContext` (SIGINT, SIGTERM), load config, buat logger, koneksi pool, router, `http.Server` dengan `ReadHeaderTimeout`, graceful shutdown 10 detik.
   - Jangan set `WriteTimeout` global karena nanti ada WebSocket; batasi per handler.
   - `router.go`: chi dengan middleware RequestID, RealIP, Recoverer, request logging via slog; route `GET /healthz` dan `GET /readyz` (ping database).

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

5. **Docker Compose (dev saja)**
   - `postgres:16-alpine` dengan healthcheck, volume bernama, port 5432.
   - `axllent/mailpit` untuk SMTP (1025) dan UI (8025).
   - Server Go dijalankan langsung di host lewat `make run`, bukan di container.

6. **Makefile** dengan target dari `CLAUDE.md`, plus `migrate-down` dan `migrate-new name=...`. `DATABASE_URL` dibaca dari `.env`.

7. **Lain-lain**
   - `.env.example` dengan nilai dev yang bekerja dengan compose.
   - `.gitignore` (binary, `.env`, `node_modules`, `dist`).
   - `.golangci.yml` (format v2), linter standar ditambah `errorlint`, `gocritic`, `revive`.
   - Folder `maps/` dan `internal/{auth,org,invite,room,realtime,moderation}` belum perlu dibuat; buat saat fiturnya dikerjakan.

8. **Frontend**
   - `web/` dibuat dengan `npm create vue@latest` (TypeScript, Router, Pinia, Vitest, ESLint, Prettier), lalu `npm i phaser`.
   - Vite dev server mem-proxy `/api` dan `/ws` (dengan `ws: true`) ke `localhost:8080`, supaya frontend dan backend satu origin (dibutuhkan cookie `SameSite=Strict`; tidak perlu CORS).
   - Pindahkan aset desain dari folder `design/` (lalu hapus folder itu):
     `design/tokens.css` → `web/src/styles/tokens.css`, `design/fonts/*` → `web/src/assets/fonts/`,
     `design/avatars/*` → `web/src/assets/avatars/`. Import `tokens.css` sekali di `main.ts` dan buang CSS bawaan template.
   - Halaman awal cukup menampilkan status dari `/healthz`, memakai token (latar Kabut, judul Fredoka, satu avatar preset)
     sebagai bukti token dan font termuat. Bandingkan dengan `docs/screens/01-style-guide.png`.

9. **README.md** singkat: prasyarat, langkah menjalankan dari nol, daftar target Makefile.

## Selesai jika

- [ ] Langkah di bagian Tujuan berjalan mulus dari clone bersih.
- [ ] `make test` dan `make lint` lulus.
- [ ] `npm run dev` di `web/` menampilkan status backend.
- [ ] Semua file mengikuti konvensi di `CLAUDE.md`.

Laporkan ringkasan perubahan, perintah yang sudah kamu jalankan beserta hasilnya, dan keputusan yang kamu ambil sendiri (misalnya versi library) di akhir.

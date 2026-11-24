# Kelasverse — Arsitektur Kode (acuan pengembangan)

Dokumen ini menjelaskan cara kode **sekarang** disusun dan menjadi acuan untuk setiap fitur baru.
Keputusan produk ada di `docs/spec.md`, keputusan teknis dan ADR di `docs/design.md`, visual di `docs/visual.md`.
Jika dokumen ini bertentangan dengan ADR, ADR yang menang. Laporkan, jangan ubah diam-diam.

## 1. Gambaran

```
Browser (Vue 3 + Phaser 3)
  ├─ REST  /api/...  ─┐
  └─ WS    /ws       ─┼─ Caddy (prod) / Vite proxy (dev) ─► Go server (satu binary)
                      │                                       ├─ chi router → handler → service → sqlc query → PostgreSQL 16
                      │                                       ├─ realtime hub (goroutine per room, state in-memory)
                      │                                       └─ object storage (file belajar), LiveKit (voice)
```

- Satu monorepo, dua project mandiri: `classverse-backend/` dan `classverse-frontend/`. Keduanya tidak saling
  mengimpor file. Kontraknya hanya `classverse-backend/api/openapi.yaml`, protokol WebSocket, dan `docs/`.
- Backend adalah modular monolith: package-by-feature, satu proses.

## 2. Backend

### 2.1 Struktur

```
classverse-backend/
├── cmd/server/            main.go (wiring + shutdown), router.go (chi, middleware, mount Routes)
├── api/openapi.yaml       kontrak REST, sumber kebenaran tipe (Go + TS di-generate darinya)
├── db/migrations/         goose, SQL murni, NNNNN_nama.sql, immutable setelah di-commit
├── db/queries/<fitur>.sql query sqlc
└── internal/
    ├── <fitur>/           assignment, auth, class, file, gradebook, guest, invite, library,
    │                      moderation, module, org, quiz, realtime, report, room, voice
    └── platform/          api (model OpenAPI hasil generate), config, db (+ query/ hasil sqlc, dbtest),
                           httpx, logx, mail, ratelimit
```

### 2.2 Lapisan dalam satu fitur

| File | Tanggung jawab | Tidak boleh |
|---|---|---|
| `handler.go` | `Routes(r chi.Router)`, baca path/query/body, panggil service, ubah ke tipe `api.*`, `toAPIError` | aturan bisnis, SQL |
| `service.go` | otorisasi (`Authorize(ctx, user, org, minRole)`), validasi domain, transaksi, aturan bisnis | tahu soal HTTP |
| `store.go` (opsional) | bungkus query bila dipakai banyak method atau perlu transaksi bersama | — |
| file per sub-kasus | mis. `counts.go`, `results.go`, `loans.go`, `progress.go` bila `service.go` membesar | — |

Keputusan (ADR-014): **lapis data adalah query `sqlc`**. Service memegang `pool` dan memanggil `query.New(pool)`;
transaksi lewat `db.InTx(ctx, pool, fn)` (`internal/platform/db`). `store.go` hanya bila ada logika data yang dipakai
bersama, bukan wrapper kosong. Helper lintas fitur di `platform/db`: `InTx`, `Time`, `Contains` (pola `ILIKE` literal).

Alur request:

```
router → auth middleware (user di context) → handler.handle(fn)
       → org.PathUUID / httpx.Pagination / decode body
       → service.X(ctx, userID, orgID, input)
            Authorize → validate → query.New(...) (dalam tx bila >1 tulis) → map ke tipe domain
       → handler map domain → api.* → httpx.JSON
       → error: toAPIError (sentinel/ValidationError → httpx.Error) → satu bentuk JSON error
```

### 2.3 Aturan wajib

- **Org scope.** Setiap query data sekolah memfilter `org_id`, dan service memeriksa membership/role lebih dulu.
  Data yang tidak boleh dilihat dibalas `not_found`, bukan `forbidden`, agar keberadaannya tidak bocor.
- **Role** dari `org`: participant < teacher < admin, dicek dengan `role.AtLeast(...)`. Wali kelas dan Guru mapel
  adalah relasi kelas (`class`), bukan role org.
- **Validasi** di service lewat `ValidationError{Fields}`, yang di handler menjadi 400 `validation_failed` + `fields`.
  Pesan berbahasa Indonesia, sedangkan field dan kode error berbahasa Inggris.
- **Error domain** berupa sentinel di package fitur (`ErrNotFound`, `ErrBusy`, ...) dan dibungkus dengan
  `fmt.Errorf("list rooms: %w", err)`. Pemetaan ke HTTP hanya ada di `toAPIError`.
- **sqlc.** Tidak ada ORM atau SQL string di Go. Filter opsional ditulis `sqlc.narg(x)::type IS NULL OR ...`.
  `uuid` nullable menjadi `uuid.NullUUID`. Pencarian literal, maksimal 100 karakter:
  `db.Contains(q)` untuk `ILIKE`, atau `strpos(lower(..), lower(..))` di SQL.
- **Migration.** File baru = nomor berikutnya. Migration lama tidak pernah diedit. Sertakan `-- +goose Down` yang
  aman. Lalu `make migrate`, `make sqlc`.
- **Kontrak.** Ubah `api/openapi.yaml` lebih dulu, lalu `make api` (Go) dan `npm run api` (TS).
- **Waktu** UTC, `now func() time.Time` diinjeksi. Log memakai `slog` tanpa data pribadi.
- **Dependensi** lewat constructor (`NewService(pool, auth, now)`). Fitur tidak mengimpor fitur lain secara
  langsung. Kebutuhan lintas fitur memakai interface kecil di sisi pemakai (mis. `Authorizer`, `Uploader`).
- **Aturan bisnis tetap:** tugas membekukan penerima dan isi saat dibuat, rapor memakai nilai kuis tertinggi,
  revisi materi immutable (ADR-013), tahun aktif hanya ditentukan admin, file yang terpakai tidak bisa dihapus
  (`file_in_use`).

### 2.4 Daftar dan pagination

- **Offset** (`?limit=&offset=`, `httpx.Pagination`, default 20, maksimal 100): respons `{items, total}`.
  Jika `?counts=true`, respons juga berisi `counts` untuk tab status (dihitung dengan filter non-status yang sama,
  dalam satu request).
- **Cursor** (`?cursor=`, dipakai ruang dan perpustakaan): respons `{items, next_cursor}`. Cursor opaque dan
  urutannya stabil `(kolom, id)`.
- Filter, pencarian, dan agregat (hitungan, progres) **selalu dihitung di DB sebelum LIMIT**, tidak pernah dari
  halaman yang sedang tampil di frontend.

### 2.5 Realtime

`internal/realtime`: `gateway.go` (upgrade + auth), `hub.go` (registry), `room.go` (satu goroutine per room,
tick loop), dan `message.go` (kontrak). `classverse-frontend/src/realtime/messages.ts` mencerminkan
`message.go`, sehingga perubahan pada satu sisi harus diikuti sisi lainnya. State ruang ada di memory, sedangkan
hanya data yang harus bertahan (ban, pin modul) yang ditulis ke DB.

### 2.6 Test

- `service_test.go`: table-driven untuk validasi dan aturan murni.
- `integration_test.go`: Postgres asli lewat `dbtest.New(t)` (database baru + semua migration). Butuh
  `TEST_DATABASE_URL`.
- `handler_test.go` untuk pemetaan HTTP. Jalankan `make lint` dan `go test ./...` sebelum commit.

## 3. Frontend

### 3.1 Struktur

```
classverse-frontend/src/
├── api/          satu file per fitur (rooms.ts, library.ts, ...); client.ts (fetch + refresh token + ApiError);
│                 types.ts (alias dari schema.d.ts hasil `npm run api`); page.ts (PAGE, hasMore, lastPage)
├── components/   komponen + <Nama>.spec.ts di sebelahnya
├── views/        halaman per route (views/org/*, views/auth/*)
├── router/       vue-router
├── stores/       Pinia: auth (access token hanya di memory), org (org aktif), pending
├── realtime/     klien WS, useRoom, useVoice, messages.ts, peta/pathfind
├── game/         Phaser: scene, layout, art (tanpa UI form)
└── styles/       tokens.css (satu-satunya sumber warna/ukuran), forms.css, classes.css, authoring.css
```

### 3.2 Pola komponen

- **UI shell di Vue, peta di Phaser.** Phaser tidak merender form atau teks panjang.
- **Kelola** (`TeacherTools.vue`) memegang navigasi. `RoomTools.vue` memilih manager per tool. Tiap
  `*Manager.vue` memegang daftar, filter, dan form-nya sendiri. Form besar dipecah menjadi komponen sendiri
  (mis. `BookForm.vue`) yang menerima props dan meng-emit `saved`/`cancel`.
- Mode HP memakai route `/kelola-ruang?bagian=...` (`ManageRoomsView.vue`), dengan komponen manager yang sama.
- **Pemanggilan API** hanya lewat `src/api/<fitur>.ts`, tidak memanggil `fetch` langsung di komponen. Error
  ditampilkan dengan `errorMessage(err)`. Error field dari `ApiError.fields` dipasang ke `aria-invalid` dan
  `.field-error`.
- **Daftar:**
  - Kirim semua filter ke server.
  - Tampilkan `items` apa adanya, tanpa `.filter()` ulang di client.
  - Gunakan `hasMore(offset, page)` untuk menentukan halaman berikutnya.
  - Gunakan `lastPage(page, list)` untuk mundur jika halaman kosong setelah hapus.
  - Pagination memakai `RemotePagination` (offset/cursor) atau `ListPagination` (dengan total).
- **Picker bertingkat** memakai `PagedSelect {label, load(offset) => {items, more}}`, dan tab status memakai
  `StatusTabs {items, label, controls}`.
- **Request balapan** (filter berubah cepat) ditangani dengan generation counter. Search di-debounce 250 ms.
- **Upload** lewat `uploadFile()` (resumable, per chunk 5 MiB). Id upload sama dengan id file.

### 3.3 Visual dan aksesibilitas

- Teks khusus pembaca layar: kelas global `sr` / `sr-only` dari `styles/forms.css`; jangan definisikan ulang di komponen.

- Hanya token dari `tokens.css` (`--kv-*`, `--color-*`, `--radius-*`, `--space-*`, `--text-*`), tanpa hex dan
  tanpa ukuran font mentah.
- Kontrol `min-height: var(--touch-min)` (36/44 px). Garis 1px `--kv-line`. Tanpa kartu bersarang dan tanpa
  bayangan hitam.
- Elemen asli (`button`, `a`, `input`, `fieldset/legend` untuk grup radio). Tombol ikon wajib `aria-label`.
  Tombol berulang diberi nama unik lewat `<span class="sr">`. Status memakai `role="status"`, error memakai
  `role="alert"`.
- HP (≤ 640 px): daftar bertumpuk, bukan tabel lebar. Tidak boleh ada overflow horizontal.
- Mockup di `docs/design/` adalah acuan tata letak, bukan sumber CSS. Petakan ke token. Tanpa data atau kontrol
  palsu: elemen yang belum punya data dari server tidak ditampilkan.

### 3.4 Test

Vitest + Vue Test Utils, satu `*.spec.ts` per komponen. Mock API dengan
`vi.spyOn(xApi, 'list').mockResolvedValue(...)` atau `vi.fn<Type>()`. Untuk pemeriksaan, utamakan perilaku dan
argumen request (`toHaveBeenLastCalledWith`), lalu `find(...).exists()`. Jalankan `npm run lint`, `npm run build`
(termasuk vue-tsc), dan `npx vitest run`.

## 4. Alur menambah fitur (checklist)

1. Baca spec/ADR terkait. Jika butuh keputusan baru, tanyakan dulu.
2. Migration baru (bila perlu), lalu query di `db/queries/<fitur>.sql`, lalu `make migrate sqlc`.
3. `api/openapi.yaml`, lalu `make api` dan `npm run api`.
4. Service (otorisasi + validasi + aturan), lalu handler + `toAPIError`, lalu mount di `cmd/server/router.go`
   bila fitur baru.
5. Test: unit untuk validasi, integration untuk SQL/otorisasi, handler bila pemetaan HTTP berubah.
6. Frontend: `src/api/<fitur>.ts`, lalu komponen + spec, lalu pasang di `RoomTools`/view.
7. Verifikasi: `make lint`, `go test ./...`, `npm run lint`, `npm run build`, `npx vitest run`, serta cek browser
   desktop 1440 px dan HP 390 px.

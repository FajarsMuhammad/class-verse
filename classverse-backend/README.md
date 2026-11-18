# classverse-backend

Server Go Kelasverse: REST API (`/api`) dan WebSocket (`/ws`) dalam satu binary.
Desain dan aturan ada di `../docs/` dan `../CLAUDE.md`.

## Prasyarat

- Go 1.26+ (`goose` dan `sqlc` sudah terdaftar sebagai `tool` di `go.mod`, tidak perlu dipasang terpisah)
- Docker + Docker Compose
- [golangci-lint](https://golangci-lint.run/) v2 untuk `make lint`

## Menjalankan dari nol

```sh
cp .env.example .env
make keys            # salin dua baris hasilnya ke .env (JWT_PRIVATE_KEY, OTP_HMAC_KEY)
make up && make migrate && make sqlc && make run

curl localhost:8080/healthz   # {"status":"ok"}
curl localhost:8080/readyz    # {"status":"ok"} jika database terhubung
```

Mailpit (penampung email OTP) bisa dibuka di http://localhost:8025.

Jika port 5432, 1025, atau 8025 sudah dipakai di mesinmu, ubah `POSTGRES_PORT`, `MAILPIT_SMTP_PORT`,
`MAILPIT_UI_PORT` di `.env` beserta `DATABASE_URL` dan `SMTP_ADDR` yang sesuai.

## Target Makefile

| Target | Fungsi |
| --- | --- |
| `make up` | Jalankan Postgres + Mailpit, tunggu sampai sehat |
| `make down` | Hentikan service (volume data tetap) |
| `make migrate` | Terapkan migration lewat `scripts/migrate.sh` (goose sampai 00020, preflight Content Studio, lalu sisanya). Butuh `psql`. Jangan pakai `goose up` langsung melewati 00020→00021 |
| `make migrate-down` | Rollback satu migration |
| `make migrate-new name=...` | Buat file migration SQL baru |
| `make api` | Generate model Go dari `api/openapi.yaml` ke `internal/platform/api` |
| `make sqlc` | Generate kode Go dari `db/queries` ke `internal/platform/db/query` |
| `make run` | Jalankan server di host |
| `make test` | Semua test (dengan `-race`) |
| `make lint` | golangci-lint |
| `make keys` | Cetak nilai acak untuk `JWT_PRIVATE_KEY` dan `OTP_HMAC_KEY` |

Semua target membaca variabel dari `.env`.

## Test integrasi PostgreSQL

Test store dan service auth memakai PostgreSQL asli. `dbtest.New` membuat database sementara
(`kv_test_*`), menjalankan semua migration, lalu menghapusnya. Test ini di-skip jika `TEST_DATABASE_URL`
kosong; nilainya sudah ada di `.env.example` (user `kelasverse` dari compose boleh membuat database).

## Storage privat — fondasi F1

AIStor opsional, tidak dijalankan oleh `make up`. License **Free**, bukan trial, harus diperoleh operator dari
MinIO dan diterima sesuai [agreement](https://www.min.io/legal/aistor-free-agreement). Simpan di
`../.config/minio.license` (izin `600`), atau atur `STORAGE_LICENSE_FILE` ke file di luar repo.
Jangan commit license/kredensial; `../.config/.gitignore` dan root `.dockerignore` melindungi direktori lokal.
Jangan jalankan script dengan shell tracing (`bash -x`).

```sh
make storage-up
```

Target ini menjalankan image AIStor yang dipin digest (RELEASE.2026-09-19T17-05-25Z), mount license read-only,
membuat bucket privat `materials`, serta user aplikasi yang hanya punya izin bucket tersebut.
Secret root dan aplikasi dibuat acak sekali di `../.config/`, izin `600`; tidak dicetak. Console root hanya
untuk operator, **bukan** kredensial backend. Endpoint lokal `127.0.0.1:9100`, console `127.0.0.1:9101`;
bisa diubah dengan `STORAGE_PORT`/`STORAGE_CONSOLE_PORT`.

Nilai lokal untuk backend yang berjalan di host:

```dotenv
STORAGE_ENDPOINT=127.0.0.1:9100
STORAGE_BUCKET=materials
STORAGE_SECURE=false
```

`STORAGE_ENDPOINT` adalah alamat API S3 tanpa `http://`/`https://`, bukan alamat console `9101`.
`STORAGE_BUCKET` adalah nama bucket yang dibuat oleh `make storage-up`, bukan nama organisasi;
semua sekolah memakai bucket privat yang sama dengan isolasi akses di backend. Jika backend berjalan
sebagai container pada jaringan Compose yang sama, endpoint internal adalah `storage:9000`, bukan
`127.0.0.1:9100`.

Untuk lokal, gunakan otomatisasi `make run` di bawah; **jangan menyalin hanya tiga nilai di atas ke `.env`**.
Konfigurasi manual juga wajib menyertakan `STORAGE_ACCESS_KEY` dan `STORAGE_SECRET_KEY` akun aplikasi.
Sumber lokalnya `../.config/storage-app.env`: `TEST_STORAGE_ACCESS_KEY` dipetakan ke `STORAGE_ACCESS_KEY`,
dan `TEST_STORAGE_SECRET_KEY` ke `STORAGE_SECRET_KEY`. Jangan gunakan akun root atau commit nilainya.
Untuk deployment lain, endpoint, bucket privat, dan kredensial diperoleh dari operator/provider S3;
production wajib `STORAGE_SECURE=true` dengan endpoint TLS.

Smoke test benar-benar menjalankan PUT/GET/range/DELETE dengan akun aplikasi dan memastikan akses anonim
mendapat 403. Objek smoke dibersihkan, volume data dipertahankan. License/health bukan bukti operasi S3 selalu
berfungsi; expiry/capacity perlu dipantau terpisah. Free single-node tidak memberi HA atau encryption at rest
bawaan; backup di luar VPS tetap wajib sebelum pilot.

Sesudah `make storage-up`, `make run` dan `make seed-library` otomatis memakai kredensial aplikasi lokal
dari `../.config/storage-app.env` jika seluruh `STORAGE_*` belum diatur. Kredensial tidak disalin ke `.env`
atau dicetak. Jika konfigurasi `STORAGE_*` diberikan, konfigurasi tersebut dipakai utuh; konfigurasi parsial
tetap ditolak oleh backend, bukan dilengkapi dengan kredensial lokal. Perintah langsung `go run` tidak
memuat konfigurasi lokal Makefile ini.

```sh
make seed-library org=<org-id>
make run
```

Seed menambahkan satu e-book contoh CC0 ke sekolah yang dipilih; aman diulang. Restart server yang sudah
berjalan tanpa storage menggunakan `make run` agar endpoint file dan reader aktif. Upload, kuota, reader,
dan katalog kini tersedia; akses peserta dibatasi pada file buku terbit di sekolahnya.

Cek regresi konfigurasi lokal tanpa DB/storage: `python3 scripts/test_storage_make.py`.

## Real-time (slice 3)

`GET /api/maps/campus` publik menyajikan `.tmj` embed (ETag, revalidasi 304).
`GET /ws/rooms/{room_id}` hanya menerima Origin yang sama persis dengan origin `FRONTEND_URL`;
konfigurasi dev harus `http://localhost:5173`, bukan `127.0.0.1` jika browser memakai `localhost`.

**Akses dari perangkat lain di jaringan (development saja).** Tambahkan origin frontend ke
`DEV_ALLOWED_ORIGINS` (pisahkan koma), misalnya `DEV_ALLOWED_ORIGINS=http://10.80.3.43:5173`, lalu restart
backend dan buka alamat itu. Origin tersebut diterima untuk refresh login dan WebSocket peta; cookie
refresh untuk origin `http://` tersebut **tanpa `Secure`** karena browser membuang cookie `Secure` di HTTP non-localhost.
Server menolak start jika variabel ini diisi saat `APP_ENV=production`. Voice/mikrofon tetap butuh HTTPS atau
`localhost`; login Google lewat alamat ini tidak didukung.
Pesan pertama `auth` wajib dalam 5 detik. JWT dan membership diperiksa sebelum join.
Kontrak envelope dan close codes ada di `../docs/design.md`, aturan peta di `maps/README.md`.

## Login Google

Opsional di dev. Buat OAuth client tipe "Web application" di Google Cloud Console dengan redirect URI
`http://localhost:5173/api/auth/google/callback` (lewat proxy Vite), lalu isi ketiga `GOOGLE_*` di `.env`.
Jika kosong, tombol Google mengarah kembali ke halaman masuk dengan pesan "belum diaktifkan".

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
| `make migrate` | Terapkan semua migration (goose up) |
| `make migrate-down` | Rollback satu migration |
| `make migrate-new name=...` | Buat file migration SQL baru |
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

## Login Google

Opsional di dev. Buat OAuth client tipe "Web application" di Google Cloud Console dengan redirect URI
`http://localhost:5173/api/auth/google/callback` (lewat proxy Vite), lalu isi ketiga `GOOGLE_*` di `.env`.
Jika kosong, tombol Google mengarah kembali ke halaman masuk dengan pesan "belum diaktifkan".

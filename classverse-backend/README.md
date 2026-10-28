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

Semua target membaca variabel dari `.env`.

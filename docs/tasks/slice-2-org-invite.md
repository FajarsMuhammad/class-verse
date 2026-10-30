# Slice 2 — Organisasi & undangan (US-02, US-03)

Rujukan: `docs/spec.md` (US-01 aturan usia, US-02, US-03, Keamanan 1/7/8), `docs/design.md` (Skema, API contract,
Peta sekolah, ADR-005/009/010), `docs/visual.md` + `docs/screens/04-kelola-ruang.png`. Branch: `slice-2-org-invite`.

Tujuan: pengguna yang profilnya lengkap bisa membuat sekolah (otomatis jadi admin, peta sekolah otomatis dibuat),
admin/pengajar membuat dan mencabut kode undangan, orang lain bergabung lewat kode atau link, lalu memilih organisasi.
Belum ada peta Phaser, ruang selain campus, atau real-time.

## Keputusan yang perlu disetujui sebelum kode

| ID | Pertanyaan | Usulan |
| --- | --- | --- |
| D2-01 | Lokasi `openapi.yaml` (pertanyaan terbuka di `design.md`) | `classverse-backend/api/openapi.yaml` sebagai sumber. Frontend generate tipe dengan `openapi-typescript` dari path relatif `../classverse-backend/api/openapi.yaml` (setelah repo dipisah: URL/artefak). Backend: `oapi-codegen` hanya model, ke `internal/platform/api/` (generated, jangan diedit). Slice ini hanya mencakup endpoint org/invite/me; endpoint auth slice 1 ikut ditulis agar kontrak lengkap. |
| D2-02 | Siapa boleh membuat organisasi? | Login, email terverifikasi, profil lengkap, **usia ≥ 18** (Keamanan 7: di bawah 18 harus lewat undangan sekolah, dan admin bertanggung jawab atas persetujuan wali). Selain itu → 403 `forbidden`. |
| D2-03 | Aturan usia saat bergabung | Bergabung lewat undangan diperbolehkan untuk semua usia (undangan = undangan sekolah). Tanggal lahir wajib terisi (`needs_profile` → 403 `profile_incomplete`). Usia dihitung dari tanggal lahir terhadap tanggal UTC hari ini. |
| D2-04 | Slug | Dibuat otomatis dari nama (huruf kecil, `a-z0-9-`, maks 48), bisa diubah di form; unik → 409 `conflict` dengan pesan. Validasi `^[a-z0-9](?:[a-z0-9-]{1,46}[a-z0-9])$`. Nama 3–80 karakter. |
| D2-05 | Logo opsional | Hanya disimpan sebagai `logo_url` https opsional; **unggah file ditunda** (butuh storage). UI belum menampilkan input logo. |
| D2-06 | Bentuk kode undangan | 8 karakter dari alfabet tanpa huruf/angka mirip (`ABCDEFGHJKMNPQRSTUVWXYZ23456789`), `crypto/rand`, ditampilkan `ABCD-EFGH`; input case-insensitive dan mengabaikan `-`/spasi. Link: `FRONTEND_URL/gabung/{code}`. |
| D2-07 | Isi undangan | `role` (`teacher`/`participant`), `expires_in_days` 1–7 (default 7), `max_uses` opsional 1–500. Pengajar hanya bisa membuat undangan `participant`. Mencabut: admin semua; pengajar hanya undangan buatannya sendiri. |
| D2-08 | Status undangan saat dipakai | Kedaluwarsa, dicabut, atau kuota habis → 410 `invite_expired`. Kode tidak ada → 404 `not_found`. Pratinjau (`GET /api/invites/{code}`) hanya mengembalikan nama organisasi dan role, tanpa daftar anggota. Rate limit pratinjau+accept 10/menit per user (mencegah tebak kode). |
| D2-09 | Sudah anggota | `accept` → 409 `conflict` tanpa mengubah role dan tanpa menambah `used_count`. |
| D2-10 | Peta sekolah otomatis | Dalam transaksi yang sama dengan pembuatan organisasi: insert `rooms` (`map_key='campus'`, `name='Peta sekolah'`, `capacity=200`, `building_key` null). Endpoint ruang lain menunggu slice 5. |
| D2-11 | Organisasi aktif di frontend | `GET /api/me` mengisi `orgs: [{id, name, slug, role}]`. Pilihan organisasi aktif disimpan di `localStorage` sebagai id saja (bukan data pribadi); server tetap memeriksa membership pada setiap request. 0 org → layar "Buat sekolah / punya kode undangan?"; 1 org → langsung; >1 → pilih. |
| D2-12 | Cek role/membership | Role dibaca dari DB tiap request (ADR-005), tanpa cache dulu sampai ada bukti bottleneck. Non-anggota mendapat 404 `not_found` (tidak membocorkan keberadaan org), anggota dengan role kurang → 403 `forbidden`. |

## Scope

**Endpoint** (lihat tabel API contract di `design.md`):

| Method | Path | Role |
| --- | --- | --- |
| POST | `/api/orgs` | login (D2-02) |
| GET | `/api/orgs/{org_id}` | participant |
| POST | `/api/orgs/{org_id}/invites` | teacher (teacher → hanya participant) |
| GET | `/api/orgs/{org_id}/invites` | teacher, hanya undangan aktif |
| DELETE | `/api/orgs/{org_id}/invites/{invite_id}` | teacher (D2-07) |
| GET | `/api/invites/{code}` | login |
| POST | `/api/invites/{code}/accept` | login |
| GET | `/api/me` | `orgs` terisi |

Ditunda (bukan US-02/03 inti): `PATCH /api/orgs/{id}`, daftar/ubah/keluarkan anggota (slice 7 butuh kick lewat WS).

**Backend**
- `internal/org/` (handler, service, store): buat org + membership admin + room campus dalam satu transaksi;
  `Membership(ctx, orgID, userID)` dipakai fitur lain lewat interface di sisi pemakai.
- `internal/invite/` (handler, service, store, code.go): buat/list/cabut/pratinjau/accept. `accept` dalam satu transaksi:
  `SELECT … FOR UPDATE` undangan, cek status dan kuota, insert membership (`ON CONFLICT DO NOTHING` → deteksi sudah anggota),
  increment `used_count`.
- `auth.Service.Me` mengambil daftar org lewat interface `OrgLister` yang diimplementasikan `org`.
- Query sqlc: `db/queries/orgs.sql`, `memberships.sql`, `invites.sql`, `rooms.sql` (insert campus saja). Tidak perlu migration
  baru kecuali review menemukan index yang kurang (`invites` per `org_id` aktif → usulkan migration `00002`).
- Log: `org created`, `invite created`, `invite revoked`, `invite accepted` dengan id saja.

**Frontend**
- `api/orgs.ts`, `api/invites.ts` dengan tipe hasil generate OpenAPI; store `org` (org aktif).
- Layar: `/mulai` (0 org: buat sekolah atau masukkan kode), `/sekolah/baru`, `/gabung` dan `/gabung/:code`
  (pratinjau nama + role → tombol Gabung), `/pilih-sekolah`, dan halaman sementara "Kelola ruang" berisi panel
  undangan (buat, salin kode/link, cabut) — kartu ruang dari `04-kelola-ruang.png` menyusul di slice 5.
- Header aplikasi (logo, nama sekolah, nav Peta sekolah/Kelola ruang sesuai role, nama + keluar) mengikuti screenshot 04.
- Link `/gabung/:code` untuk tamu: arahkan ke `/masuk?redirect=/gabung/:code`.

## Test

- Service table-driven (clock diinjeksi): usia 17/18, profil belum lengkap, slug bentrok, undangan kedaluwarsa/dicabut/kuota habis,
  pengajar membuat undangan teacher → ditolak, pengajar mencabut undangan orang lain → ditolak, sudah anggota.
- Store (Postgres asli): transaksi buat org (rollback jika insert campus gagal), accept serentak pada `max_uses=1` → tepat satu
  berhasil, isolasi `org_id` (undangan org A tidak bisa dicabut lewat path org B).
- Handler: bentuk error, 404 untuk non-anggota, 403 untuk role kurang, rate limit pratinjau.
- Frontend: normalisasi kode, guard (0/1/>1 org), alur gabung lewat link.

## Selesai jika

- [x] Keputusan D2-01…D2-12 disetujui dan ditulis ke `docs/design.md`.
- [x] Buat sekolah → jadi admin → room campus ada di DB.
- [x] Admin membuat undangan pengajar dan peserta; pengajar hanya bisa membuat undangan peserta.
- [x] Akun lain membuka link undangan → masuk → gabung → organisasi terpilih.
- [x] Undangan dicabut/kedaluwarsa → pesan yang jelas; request kode ke-11 dalam semenit → 429.
- [x] Non-anggota tidak bisa melihat organisasi (404).
- [x] `make test`, `make lint`, `npm run lint`, `npm run test:unit -- --run`, `npm run build` lulus; demo di browser.
- [ ] Review manusia sebelum slice 3.

Catatan implementasi: log request kini mencatat pola route (`/api/invites/{code}`) alih-alih path mentah, agar
kode undangan tidak masuk log. `openapi-typescript` 7.13 meminta peer TypeScript 5; `package.json` memakai
`overrides` agar memakai TypeScript 6 proyek (generator saja, hasilnya `.d.ts` biasa).

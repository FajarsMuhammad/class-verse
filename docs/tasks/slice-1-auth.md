# Slice 1 — Auth (US-01)

Rujukan: `docs/spec.md` (US-01, Keamanan, NFR), `docs/design.md` (bagian Auth, Skema PostgreSQL, API contract),
`docs/visual.md` (komponen dasar). Branch: `slice-1-auth`.

Tujuan: pengguna bisa daftar dengan email + OTP atau login Google, tetap login lewat refresh token,
melengkapi profil (nama tampilan, tanggal lahir), dan logout. Belum ada organisasi, avatar, atau real-time.

## Backend

### Package dan file

```
internal/auth/
  handler.go     endpoint HTTP, set/clear cookie
  service.go     register, verify, login, refresh, logout, google login, complete profile
  store.go       query users, email_otps, refresh_tokens
  token.go       issue & verify access token (JWT EdDSA)
  password.go    argon2id hash & verify
  otp.go         generate & hash OTP
  google.go      OAuth2 + OIDC (Authorization Code + PKCE)
  middleware.go  RequireUser middleware + UserID(ctx)
internal/platform/mail/     SMTP sender (Mailpit di dev) + template teks Bahasa Indonesia
internal/platform/ratelimit/ limiter in-memory per key (cukup untuk satu instance, ADR-002)
db/queries/users.sql, refresh_tokens.sql, email_otps.sql
```

Fitur lain nanti hanya boleh mengimpor `auth.RequireUser` dan `auth.UserID` dari package ini.

### Config baru

`JWT_PRIVATE_KEY` (seed Ed25519 32 byte, base64), `OTP_HMAC_KEY` (base64, minimal 32 byte),
`GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URL`. Tambahkan ke `.env.example` dan
buat target `make keys` yang mencetak nilai acak untuk dua kunci pertama.

### Library

`golang-jwt/jwt/v5` (EdDSA), `golang.org/x/crypto/argon2`, `golang.org/x/oauth2` (termasuk `GenerateVerifier`
dan `S256ChallengeOption` untuk PKCE), `coreos/go-oidc/v3`. Jangan menambah library lain tanpa usul.

### Endpoint

| Method | Path | Body / hasil |
| --- | --- | --- |
| POST | `/api/auth/register` | `{email, password, display_name, birth_date}` → 202 |
| POST | `/api/auth/verify-email` | `{email, code}` → 200 `{access_token}` + cookie refresh (langsung login) |
| POST | `/api/auth/resend-otp` | `{email}` → 202 |
| POST | `/api/auth/login` | `{email, password}` → 200 `{access_token}` + cookie refresh |
| GET | `/api/auth/google/start` | redirect ke Google |
| GET | `/api/auth/google/callback` | redirect ke `FRONTEND_URL/auth/callback` (atau `?error=<code>`) + cookie refresh |
| POST | `/api/auth/refresh` | cookie → 200 `{access_token}` + cookie baru |
| POST | `/api/auth/logout` | cabut family, hapus cookie → 204 |
| GET | `/api/me` | profil + `needs_profile` + `orgs` (kosong sampai slice 2) |
| PATCH | `/api/me` | `{display_name?, birth_date?}` (avatar menyusul di slice 4) |

`resend-otp` adalah tambahan kecil dari tabel endpoint di `docs/design.md`; tambahkan juga ke dokumen itu.

### Aturan

**Registrasi dan OTP**
- Validasi: email valid dan dinormalisasi ke huruf kecil; password 10–128 karakter; `display_name` 3–24 karakter setelah trim;
  `birth_date` wajib, tidak di masa depan. Error validasi → 400 `validation_failed` dengan `error.fields`.
- Agar tidak membocorkan email mana yang terdaftar, `register` dan `resend-otp` **selalu** membalas 202.
  Jika email sudah terverifikasi, kirim email pemberitahuan "akun sudah ada" alih-alih OTP.
- OTP 6 digit dari `crypto/rand`, disimpan sebagai HMAC-SHA256 dengan `OTP_HMAC_KEY`, berlaku 10 menit, maksimal 5 percobaan,
  OTP lama dinonaktifkan saat OTP baru dibuat, cooldown kirim ulang 60 detik.
- Bandingkan hash dengan `subtle.ConstantTimeCompare`.

**Password dan login**
- argon2id dengan parameter OWASP: memory 19 MiB, iterations 2, parallelism 1, salt 16 byte, key 32 byte;
  simpan dalam format PHC (`$argon2id$v=19$m=...,t=...,p=...$salt$hash`) agar parameter bisa dinaikkan nanti.
- Login gagal karena email tidak ada atau password salah → 401 `invalid_credentials` yang sama.
  Jika email tidak ada, tetap jalankan verifikasi terhadap hash dummy agar waktu respons setara.
- Email belum terverifikasi → 403 `email_not_verified` (frontend mengarahkan ke halaman verifikasi).

**Access token**
- JWT EdDSA, umur 15 menit, claim `sub`, `iat`, `exp`, `jti`, `iss=kelasverse`, `aud=kelasverse-web`. Tidak ada role.
- `RequireUser` membaca `Authorization: Bearer`, memverifikasi signature, `exp`, `iss`, `aud`; gagal → 401 `unauthenticated`.

**Refresh token**
- 32 byte acak, base64url, cookie `kv_refresh` dengan `HttpOnly; Secure; SameSite=Strict; Path=/api/auth; Max-Age=2592000`.
  `Secure` tetap aktif di dev (browser menganggap `localhost` sebagai secure context).
- Database hanya menyimpan SHA-256 token.
- Rotasi dalam satu transaksi dengan `SELECT ... FOR UPDATE` pada baris token:
  - tidak ditemukan atau kedaluwarsa → 401 `unauthenticated`;
  - sudah pernah dipakai (`used_at` terisi) atau dicabut → **cabut seluruh family**, 401 `session_revoked`;
  - valid → isi `used_at`, buat token baru di family yang sama dengan `expires_at` = sekarang + 30 hari.
- Endpoint yang memakai cookie (`refresh`, `logout`) menolak request yang header `Origin`-nya bukan `FRONTEND_URL`.

**Login Google**
- `start`: buat `state` dan PKCE verifier, simpan di cookie `kv_oauth` (`HttpOnly; Secure; SameSite=Lax; Path=/api/auth/google; Max-Age=600`).
  Harus `Lax`, bukan `Strict`, karena cookie ini harus ikut terkirim saat Google me-redirect kembali (navigasi lintas situs).
- `callback`: cocokkan `state`, tukar code dengan verifier, verifikasi ID token (`iss`, `aud`, `exp`) dan wajibkan `email_verified`.
- Penautan akun:
  1. user dengan `google_sub` yang sama → login;
  2. user dengan email sama yang **sudah** terverifikasi → tautkan `google_sub`, login;
  3. user dengan email sama yang **belum** terverifikasi → hapus `password_hash`, tandai email terverifikasi, tautkan, login
     (mencegah orang lain mendaftarkan email korban lebih dulu lalu ikut memiliki akun itu);
  4. tidak ada → buat user baru dengan `display_name` dari nama Google (dipotong ke 24 karakter), `birth_date` kosong.
- Hapus cookie `kv_oauth` setelah dipakai, berhasil maupun gagal.

**Profil**
- `needs_profile = birth_date kosong`. Aturan usia di bawah 15 tahun ditegakkan di slice 2 saat bergabung ke organisasi,
  bukan di sini; slice ini hanya memastikan tanggal lahir terisi.

**Rate limit** (berdasarkan IP, dan email jika ada; lewat batas → 429 `rate_limited` dengan header `Retry-After`)
- login: 5/menit per IP dan 5/menit per email
- register dan resend-otp: 3/menit per IP
- verify-email: 10/menit per IP (di samping batas 5 percobaan per OTP)
- refresh: 30/menit per IP

**Logging**: log event (`user registered`, `login failed`, `refresh reuse detected`) dengan user id, tanpa email, token, password, atau OTP.

## Frontend

- Halaman: `/daftar`, `/verifikasi`, `/masuk` (email + tombol "Masuk dengan Google"), `/auth/callback`, `/lengkapi-profil`, dan satu halaman beranda sementara yang menampilkan nama pengguna + tombol keluar.
- Belum ada mockup untuk halaman auth: buat kartu di tengah layar dengan latar Kabut, judul Fredoka, input dan tombol sesuai `docs/visual.md`.
  Semua teks Bahasa Indonesia; pesan error dari `error.code` dipetakan ke kalimat yang ramah.
- Store Pinia `auth`: access token hanya di memory, tidak di localStorage.
- API client:
  - menyisipkan `Authorization: Bearer`;
  - saat 401, refresh sekali lalu ulangi request; jika refresh gagal, arahkan ke `/masuk`;
  - refresh bersifat single-flight dan dikoordinasikan antar tab dengan `navigator.locks.request('kv-refresh', …)`,
    supaya beberapa tab tidak memakai refresh token yang sama bersamaan (yang akan memicu deteksi reuse dan mengeluarkan user).
- Saat aplikasi dimuat, panggil `/api/auth/refresh` untuk memulihkan sesi, lalu `/api/me`.
- Route guard: halaman butuh login → `/masuk`; `needs_profile` → `/lengkapi-profil`.

## Test

- Service (table-driven, clock diinjeksi): OTP kedaluwarsa, salah 5 kali, cooldown; login salah/benar/belum verifikasi;
  rotasi refresh normal; reuse mencabut family; token kedaluwarsa; empat kasus penautan Google.
- Store: integration test dengan Postgres untuk query baru, termasuk rotasi di dalam transaksi.
- Handler: atribut cookie (`HttpOnly`, `Secure`, `SameSite`, `Path`), penolakan `Origin` asing, bentuk error JSON, 429.
- Google: callback diuji dengan fake OIDC provider (`httptest.Server` yang menyajikan discovery, JWKS, dan token endpoint).
- Frontend: unit test API client (refresh sekali lalu ulang; gagal → logout).

## Selesai jika

- [ ] Daftar → OTP muncul di Mailpit (http://localhost:8025) → verifikasi → langsung masuk.
- [ ] Login email, logout, login lagi.
- [ ] Login Google dengan OAuth client mode testing berhasil, termasuk user baru yang diarahkan ke lengkapi profil.
- [ ] Reload halaman tetap login; dua tab terbuka tidak saling mengeluarkan.
- [ ] Memakai refresh token lama secara manual (curl) mencabut sesi.
- [ ] Rate limit login terlihat bekerja (percobaan ke-6 dalam semenit → 429).
- [ ] `docs/design.md` diperbarui dengan `resend-otp` dan nama cookie.
- [ ] `make test` dan `make lint` lulus.

Catatan setup Google: buat OAuth client tipe "Web application" di Google Cloud Console dengan redirect URI
`http://localhost:5173/api/auth/google/callback` (lewat proxy Vite), lalu isi tiga variabel `GOOGLE_*` di `.env`.

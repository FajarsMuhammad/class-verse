# Kelasverse — Desain Teknis (Fase 2)

Oct 28, 2026 · @Fajar

## Keputusan utama

Kelasverse dibangun sebagai satu backend Go (modular monolith) yang melayani REST dan WebSocket, dengan PostgreSQL dan LiveKit untuk voice di M2.

| Area | Pilihan | Alasan |
| --- | --- | --- |
| Bentuk backend | Modular monolith, satu binary Go | Satu tim, deploy sederhana; batas package dijaga agar bisa dipecah nanti |
| HTTP router | `chi` | Ringan, kompatibel `net/http`, middleware jelas |
| WebSocket | `coder/websocket` | Mendukung context, API modern, terawat |
| Database | PostgreSQL 16 + `pgx` + `sqlc` | Query eksplisit dan type-safe, tanpa magic ORM |
| Migration | `goose` | SQL murni, mudah dibaca |
| State real-time | In-memory per room (goroutine per room) | Latency rendah; cukup untuk pilot satu instance |
| Auth | JWT access token + refresh token + Google OAuth | Ringan untuk MVP, tanpa server identitas terpisah |
| Voice (M2) | LiveKit (self-host atau Cloud) | SFU siap pakai, ditulis dalam Go, SDK web tersedia |
| Frontend | Vue 3 + Vite + TypeScript + Pinia | UI shell: lobby, panel, chat |
| Game client | Phaser 3 + map dari Tiled | Rendering tile map 2D dan avatar |
| Deploy pilot | Satu VPS (Jakarta/Singapura), Docker Compose + Caddy | Murah, HTTPS otomatis, WebSocket aman |

## Arsitektur

```
Browser (Vue 3 + Phaser 3) --HTTPS/WSS--> Caddy --/api--> Go server: REST API (chi)
                                                 --/ws---> Go server: Realtime (gateway + hub, goroutine per room)
Go server --SQL--> PostgreSQL
Go server --OAuth--> Google (tukar code, cek ID token)
Go server --server API--> LiveKit (M2)
Browser  --WebRTC audio--> LiveKit (M2)
```

Browser hanya bicara ke satu domain lewat Caddy; audio voice di M2 mengalir langsung antara browser dan LiveKit, sementara server Go hanya menerbitkan token dan memanggil API LiveKit untuk mute dan kick.

## Struktur repo

Monorepo dengan dua project yang berdiri sendiri agar nanti bisa dipisah menjadi dua repo (ADR-010). Keduanya tidak saling mengimpor file; kontraknya hanya HTTP API, protokol WebSocket, dan dokumen di `docs/`.

```
class-verse/
├── docs/                     # spec, design, visual, screens, tasks (bersama)
├── CLAUDE.md
├── classverse-backend/       # Go module github.com/FajarsMuhammad/class-verse/classverse-backend
└── classverse-frontend/      # Vue 3 + Vite + Phaser
```

Dev lokal: server Go jalan di host (`make run`, port 8080), Postgres dan Mailpit lewat `classverse-backend/compose.yaml`, dan Vite dev server (port 5173) mem-proxy `/api`, `/ws`, dan `/healthz` ke server Go sehingga browser tetap melihat satu origin, sama seperti di belakang Caddy saat produksi.

## Struktur package Go

Setiap fitur adalah satu package dengan tiga lapis yang konsisten: `handler` (HTTP/WS), `service` (aturan bisnis), dan `store` (akses database). Package fitur tidak saling mengimpor store milik fitur lain; komunikasi lewat interface service.

```
classverse-backend/
├── cmd/server/
│   ├── main.go               # wiring: config, db, router, room manager; graceful shutdown
│   └── router.go             # chi + middleware, /healthz, /readyz
├── internal/
│   ├── auth/                 # login, Google OAuth, JWT, refresh token
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── store.go
│   │   └── token.go
│   ├── org/                  # organisasi, membership, role
│   ├── invite/               # kode & link undangan
│   ├── room/                 # CRUD ruang, jadwal
│   ├── realtime/             # WebSocket gateway + room hub (state in-memory)
│   │   ├── gateway.go        # upgrade, auth, baca/tulis pesan
│   │   ├── hub.go            # registry room aktif
│   │   ├── room.go           # satu goroutine per room, tick loop
│   │   ├── message.go        # tipe pesan (kontrak protokol)
│   │   └── mapdata.go        # collision & zona dari file Tiled
│   ├── moderation/           # report, mute, kick, log, filter chat
│   ├── voice/                # token LiveKit (M2)
│   └── platform/             # hal lintas fitur yang kecil saja
│       ├── config/
│       ├── db/               # pool pgx, transaksi
│       │   └── query/        # hasil generate sqlc (jangan diedit)
│       ├── httpx/            # error response, middleware
│       └── logx/             # slog setup
├── db/
│   ├── migrations/           # goose
│   └── queries/              # sqlc
├── maps/                     # file Tiled (.tmj)
├── compose.yaml              # Postgres + Mailpit untuk dev
├── Makefile                  # up, down, migrate, sqlc, run, test, lint
└── sqlc.yaml
```

Struktur frontend:

```
classverse-frontend/
├── src/
│   ├── api/                  # pemanggil REST (fetch), satu file per fitur
│   ├── assets/avatars/       # avatar preset (SVG)
│   ├── assets/fonts/         # Fredoka, Nunito (SIL OFL, self-hosted)
│   ├── realtime/             # klien WebSocket; messages.ts mencerminkan internal/realtime/message.go
│   ├── router/
│   ├── stores/               # Pinia (access token hanya di memory)
│   ├── styles/tokens.css     # design token, satu-satunya sumber warna/font/ukuran
│   └── views/
└── vite.config.ts            # proxy dev ke :8080
```

**Endpoint operasional.** `GET /healthz` selalu `{"status":"ok"}` selama proses hidup; `GET /readyz` mem-ping database dan mengembalikan 503 `unavailable` jika gagal. Keduanya di luar `/api` dan tanpa auth.

**Log request.** Satu baris per request dengan pola route chi (misalnya `/api/invites/{code}`), bukan path mentah, dan tanpa query string atau header, sehingga kode undangan, token, dan data pribadi tidak masuk log.

**IP klien.** Router memakai `middleware.ClientIPFromXFF` (bukan `RealIP` yang rentan spoofing): `X-Forwarded-For` hanya dipercaya dari peer di loopback atau jaringan privat (Caddy di VPS yang sama). IP dibaca dengan `middleware.GetClientIP`, misalnya untuk rate limit login.

Aturan konsistensi: nama package satu kata dan sesuai domain, error dibungkus dengan `fmt.Errorf("...: %w", err)`, dan semua handler mengembalikan error lewat satu helper di `httpx`.

## Protokol real-time

Server adalah sumber kebenaran: client mengirim posisi yang diinginkan, server memvalidasi lalu menyiarkan snapshot 10 kali per detik.

**Koneksi.** Client membuka `wss://<host>/ws/rooms/{roomId}` dan mengirim access token di pesan pertama (`auth`), bukan di URL, agar token tidak tercatat di log proxy. Server menutup koneksi jika `auth` tidak datang dalam 5 detik.

**Envelope.** Semua pesan JSON dengan bentuk yang sama:

```json
{ "t": "move", "seq": 42, "d": { "x": 120, "y": 88, "dir": "left" } }
```

`t` = tipe pesan, `seq` = nomor urut dari pengirim (untuk ack dan deteksi pesan hilang), `d` = payload.

**Client → server**

| `t` | Payload | Aturan server |
| --- | --- | --- |
| `auth` | `token`, `mode?` (`map`/`list`) | Validasi JWT dan membership organisasi; `list` = mode HP (slice 8) |
| `move` | `x`, `y`, `dir` | Maks 15 pesan/detik; ditolak jika menembus collision atau melebihi kecepatan maksimum |
| `chat` | `text`, `scope` (`near` / `zone` / `room`) | Maks 3 pesan/detik; difilter; `room` ditolak jika chat semua ruangan dimatikan |
| `emote` | `kind` | Maks 1/detik |
| `cmd` | `name` (`broadcast`, `gather`, `mute`, `kick`, `lock`, `voice_broadcast`, `voice_broadcast_off`), `target?` | Hanya role pengajar/admin |
| `ping` | `ts` | Dibalas `pong` untuk mengukur latency |
| `travel` | `room_id` | Hanya ke ruang yang boleh dimasuki dan saat avatar berada di pintu gedung (kecuali mode HP); koneksi dipindah tanpa reconnect |
| `seat` | `zone` | Mode HP saja, di kelas/taman; maks 1/detik (slice 8) |

**Server → client**

| `t` | Payload | Kapan |
| --- | --- | --- |
| `welcome` | id diri, map, daftar peserta, state ruang | Setelah `auth` berhasil |
| `snapshot` | daftar `{id, x, y, dir}` yang berubah | Setiap tick (100 ms), hanya jika ada perubahan |
| `joined` / `left` | data peserta | Peserta masuk/keluar |
| `zone` | `zoneId` atau kosong | Peserta masuk/keluar zona |
| `chat` | `from`, `text`, `scope`, `at` | Dikirim hanya ke penerima yang memenuhi radius/zona |
| `event` | `name`, `by`, data | Hasil `cmd` pengajar (misalnya gather, mute) |
| `error` | `code`, `message`, `seq` | Pesan ditolak |
| `traveled` | isi welcome untuk ruang tujuan | Setelah travel berhasil, termasuk saat kembali ke peta sekolah |
| `buildings` | jumlah orang dan kelas live per gedung | Setiap 5 detik, hanya di peta sekolah |

**Model eksekusi.** Setiap room berjalan di satu goroutine dengan inbox channel dan ticker 100 ms, sehingga state room tidak butuh mutex. Setiap koneksi punya goroutine tulis dengan buffer 64 pesan; jika buffer penuh, koneksi dianggap lambat dan ditutup agar tidak menghambat room.

**Reconnect.** Client otomatis menyambung ulang dengan backoff 1, 2, 4, 8 detik. Jika kembali dalam 30 detik, posisi terakhir dipertahankan dan peserta lain tidak melihat `left`.

**Proximity.** Penerima chat `near` dihitung server dari jarak tile (default radius 5). Zona dibaca dari layer objek di file Tiled. Untuk 40 orang per room, perhitungan O(n²) per tick masih sangat murah, jadi spatial index belum diperlukan.

**Peta sekolah.** Setiap organisasi punya satu room dengan `map_key = 'campus'`, dibuat otomatis saat organisasi dibuat; semua peserta muncul di sini setelah masuk. Kapasitasnya 200, jadi `snapshot` hanya berisi avatar dalam radius 24 tile dari penerima (area of interest berbasis grid sel 8×8 tile), agar bandwidth per peserta tetap kecil. Gedung dan pintunya didefinisikan di layer objek file Tiled peta sekolah; setiap ruang menyimpan `building_key` gedungnya. Satu koneksi WebSocket dipakai sepanjang sesi: `travel` memindahkan peserta antar room di dalam hub, dan keluar dari ruang mengembalikannya ke pintu gedung asal.

**Detail slice 3 (real-time inti).**

- *Peta.* File Tiled `.tmj` ada di `classverse-backend/maps/`, di-embed ke binary, dan disajikan `GET /api/maps/{map_key}` (publik, bukan data organisasi; `ETag` + `Cache-Control: no-cache`). Server dan Phaser membaca file yang sama. Tile 32 px. Layer tile `ground`, layer tile `collision` (tile ≠ 0 tidak bisa dilewati), layer objek `spawn` (persegi) dan `buildings` (persegi per gedung dengan properti `key`, `name`, `roof`, `staff_only`, plus objek titik `door` yang properti `building`-nya menunjuk `key`). Urutan id tile adalah kontrak di `maps/README.md`. Revisi D3-02 disetujui sesudah review visual: frontend menggambar ilustrasi flat utuh beresolusi 2× dari token `--map-*`/`--roof-*` dan bounds gedung Tiled, bukan atlas tile untuk tampilan gedung. Collision/spawn/pintu tetap mengikuti Tiled. Kamera overview menampilkan seluruh sekolah dan menyesuaikan resize; avatar dan label mempertahankan ukuran layar.
- *Room peta sekolah.* `GET /api/orgs/{org_id}` mengembalikan `campus_room_id`; client membuka `/ws/rooms/{campus_room_id}`.
- *Auth WebSocket.* Upgrade hanya dari Origin `FRONTEND_URL`. Pesan pertama wajib `auth {token}` dalam 5 detik; server memverifikasi JWT dan membership organisasi pemilik room lalu mengirim `welcome`. Gagal → `error` lalu close 4401 (token), 4403 (tidak boleh), 4404 (room tidak ada/bukan anggota).
- *Satu koneksi per user.* Koneksi baru user yang sama menggantikan yang lama; yang lama ditutup dengan 4409 `replaced` dan client tidak reconnect otomatis.
- *Movement.* Koordinat pixel dunia (integer, titik kaki avatar), `dir` `up`/`down`/`left`/`right`. Kecepatan maksimum per peta dari properti peta Tiled `walk_speed` (px/detik, bilangan bulat 80–400, default 160; disetujui setelah review slice 5 karena peta sekolah tampil di zoom ±0,4 sehingga 160 px/detik terasa lambat): `campus` 320, `classroom`/`garden` 160. Server dan client membaca angka yang sama dari file peta. Kecepatan ditegakkan sebagai budget jarak (token bucket) agar jitter jaringan tidak menolak gerak sah: budget bertambah `walk_speed` × 1,25 px per detik, maksimal setengah detik gerak, dan setiap `move` memakai jaraknya; `move` diterima jika jarak ≤ budget + 8 px, tujuan bukan tile collision dan di dalam peta, dan lintasan (dicek tiap 8 px) tidak melewati collision. Lebih dari 15 `move`/detik dibuang. Ditolak → `error {code: "move_rejected", seq, x, y}` berisi posisi resmi.
- *Pesan.* `welcome {self_id, room_id, map_key, participants: [{id, name, avatar, role, x, y, dir}]}`; `joined` = satu participant; `left {id}`; `snapshot {p: [{id, x, y, dir}], out: [id]}` dengan `out` = id yang keluar dari area of interest penerima; `ping {ts}` → `pong {ts}`. `seq` server naik per koneksi.
- *Area of interest* hanya di peta sekolah: grid sel 8×8 tile, radius 24 tile; penerima mendapat avatar yang berubah di radius dan avatar yang baru masuk radius. `joined`/`left` dikirim ke seluruh room.
- *Reconnect.* Putus → avatar ditahan 30 detik dan tetap terlihat. Kembali ke room yang sama dalam 30 detik → posisi lama tanpa `left`/`joined`. Client backoff 1, 2, 4, 8 detik (8 berulang).
- *Siklus room.* Hub (registry, satu mutex) membuat goroutine room saat orang pertama masuk dan menghentikannya setelah kosong 60 detik. Shutdown menutup koneksi dengan 1001. Sesi/kehadiran menyusul di slice 5.
- *Client.* Klik untuk berjalan memakai A* di grid collision di client; server tetap memvalidasi setiap `move`. Avatar lain diinterpolasi dengan jeda render 100 ms. Name tag: pill putih, diri sendiri Tinta, pengajar/admin Nila.

JSON dipilih untuk MVP karena mudah di-debug. Jika bandwidth jadi masalah, `snapshot` bisa dipindah ke format biner tanpa mengubah tipe pesan lain.

## Auth: JWT + refresh token + Google OAuth

Access token berumur pendek di memory browser, refresh token panjang di cookie HttpOnly, dan Google OAuth diproses sepenuhnya di server.

**Token**

| Token | Bentuk | Umur | Disimpan di |
| --- | --- | --- | --- |
| Access token | JWT, ditandatangani Ed25519 | 15 menit | Memory frontend (state Pinia), bukan localStorage |
| Refresh token | String acak 32 byte (opaque) | 30 hari, geser saat dipakai | Cookie `kv_refresh` dengan `HttpOnly; Secure; SameSite=Strict; Path=/api/auth`; di database hanya hash SHA-256 |

Claim JWT dibuat minimal: `sub` (user id), `iat`, `exp`, `jti`. Role tidak dimasukkan ke token karena role berbeda per organisasi dan bisa dicabut kapan saja; role dicek dari database (dengan cache singkat) pada setiap request yang butuh otorisasi.

**Rotasi refresh token.** Setiap `POST /api/auth/refresh` menerbitkan refresh token baru dan menandai yang lama sebagai terpakai. Semua token dari satu login berada dalam satu *family*. Jika token yang sudah terpakai dipakai lagi, itu tanda pencurian: seluruh family dicabut dan user harus login ulang.

**Login Google (Authorization Code + PKCE)**

1. Frontend memanggil `GET /api/auth/google/start`.
2. Server membuat `state` dan `code_verifier`, menyimpannya di cookie `kv_oauth` (`HttpOnly; Secure; SameSite=Lax; Path=/api/auth/google`, 10 menit; `Lax` agar ikut terkirim saat Google me-redirect kembali), lalu redirect ke Google.
3. Google redirect ke `GET /api/auth/google/callback?code=...&state=...`.
4. Server mencocokkan `state`, menukar `code` dengan token Google, lalu memverifikasi ID token (`iss`, `aud`, `exp`, `email_verified`) memakai `coreos/go-oidc`.
5. Server mencari user berdasarkan `google_sub`; jika belum ada, mencocokkan email terverifikasi untuk menautkan akun, atau membuat user baru.
6. Server menyetel cookie refresh token dan redirect ke frontend, yang lalu memanggil `/refresh` untuk mendapat access token pertama.

User baru dari Google tetap wajib mengisi tanggal lahir sebelum bisa bergabung ke organisasi; di bawah 15 tahun hanya bisa lewat undangan sekolah.

**Login email.** Password di-hash dengan argon2id. Registrasi mengirim OTP 6 digit (berlaku 10 menit, maks 5 percobaan) untuk verifikasi email.

**Endpoint**

| Method | Path | Fungsi |
| --- | --- | --- |
| POST | `/api/auth/register` | Daftar dengan email + password |
| POST | `/api/auth/verify-email` | Verifikasi OTP, langsung login |
| POST | `/api/auth/resend-otp` | Kirim ulang OTP (selalu 202, cooldown 60 detik) |
| POST | `/api/auth/login` | Login email, menyetel cookie refresh |
| GET | `/api/auth/google/start` | Mulai OAuth Google |
| GET | `/api/auth/google/callback` | Callback OAuth Google |
| POST | `/api/auth/refresh` | Rotasi refresh token, mengembalikan access token |
| POST | `/api/auth/logout` | Mencabut family refresh token saat ini |
| GET | `/api/me` | Profil dan daftar organisasi |

**WebSocket.** Access token hanya dicek saat `auth` di awal koneksi. Sesi yang sudah berjalan tetap hidup walau token kedaluwarsa; pencabutan akses (kick, dikeluarkan dari organisasi) ditegakkan langsung oleh server dengan menutup koneksi.

**Proteksi tambahan.** Rate limit 5 percobaan login per menit per IP dan per email; CSRF aman karena endpoint cookie hanya menerima `POST` dengan `SameSite=Strict` dan cek header `Origin`.

## ADR

Setiap keputusan besar dicatat singkat agar alasan dan batasnya tidak hilang.

| ID | Keputusan | Alasan | Dievaluasi ulang jika |
| --- | --- | --- | --- |
| ADR-001 | Modular monolith, bukan microservices | Satu tim, pilot kecil, deploy dan debug lebih sederhana | Tim bertambah atau realtime perlu diskalakan terpisah |
| ADR-002 | State room in-memory di satu instance | Latency terendah, tanpa Redis | Butuh lebih dari satu instance (pindah ke sharding room per instance + Redis untuk presence) |
| ADR-003 | Server-authoritative movement | Mencegah teleport/cheat, collision konsisten | — |
| ADR-004 | Auth sendiri (JWT + refresh + Google OAuth), bukan Keycloak | Lebih ringan untuk MVP, tanpa server tambahan | Butuh SSO sekolah/kampus (SAML, Microsoft) atau multi-aplikasi |
| ADR-005 | Role tidak disimpan di JWT | Role per organisasi dan bisa dicabut seketika | Query role jadi bottleneck |
| ADR-006 | LiveKit untuk voice | Tidak membangun SFU sendiri | Biaya Cloud terlalu tinggi (pindah self-host) |
| ADR-007 | JSON untuk protokol WS | Mudah di-debug selama MVP | Bandwidth `snapshot` jadi masalah |
| ADR-008 | `sqlc` + `pgx`, bukan GORM | Query eksplisit, performa terprediksi, konsisten dengan prinsip kode sederhana | — |
| ADR-009 | Peta sekolah adalah room biasa (`map_key = campus`) dengan area of interest | Memakai ulang hub dan protokol yang sama, tanpa service terpisah | Satu sekolah butuh lebih dari 200 orang di peta bersamaan |
| ADR-010 | Monorepo berisi dua project mandiri (`classverse-backend`, `classverse-frontend`) | Satu tempat untuk dokumen dan perubahan lintas sisi selama MVP, tapi batasnya sudah siap untuk dipisah | Tim backend dan frontend terpisah, atau siklus rilis keduanya berbeda |

## Data model

```
Akun:       users <- email_otps, refresh_tokens
Organisasi: organizations <- memberships (-> users), invites, reports
Ruang:      rooms (-> organizations) <- room_sessions <- session_attendances, chat_messages
            rooms <- moderation_actions
Tabel aktivitas (sesi, chat, laporan, moderasi) juga merujuk users.
```

Setiap data milik satu organisasi: memeriksa `org_id` dan membership di service layer sudah cukup untuk menjamin ruang tertutup per organisasi.

## Skema PostgreSQL

Migration awal berisi 12 tabel. Semua id berupa UUID kecuali `chat_messages` yang memakai bigint agar urutan dan index lebih efisien; semua waktu `timestamptz` dalam UTC.

```sql
-- +goose Up
CREATE TYPE org_role AS ENUM ('admin', 'teacher', 'participant');
CREATE TYPE chat_scope AS ENUM ('near', 'zone', 'room', 'broadcast');
CREATE TYPE mod_action AS ENUM ('mute', 'unmute', 'kick', 'lock', 'unlock');
CREATE TYPE report_status AS ENUM ('open', 'reviewed', 'dismissed');

CREATE TABLE users (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email             text NOT NULL,
    email_verified_at timestamptz,
    password_hash     text,              -- null untuk akun Google saja
    google_sub        text UNIQUE,
    display_name      text NOT NULL,
    birth_date        date,              -- wajib diisi sebelum join organisasi
    avatar_key        text NOT NULL DEFAULT 'default',
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_uq ON users (lower(email));

CREATE TABLE email_otps (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash   bytea NOT NULL,
    attempts    int NOT NULL DEFAULT 0,
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz
);

CREATE TABLE refresh_tokens (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    family_id  uuid NOT NULL,
    token_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX refresh_tokens_family_idx ON refresh_tokens (family_id);

CREATE TABLE organizations (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL,
    slug       text NOT NULL UNIQUE,
    logo_url   text,
    created_by uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE memberships (
    org_id    uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id   uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role      org_role NOT NULL,
    joined_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, user_id)
);
CREATE INDEX memberships_user_idx ON memberships (user_id);

CREATE TABLE invites (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    code       text NOT NULL UNIQUE,
    role       org_role NOT NULL CHECK (role <> 'admin'),
    max_uses   int,                      -- null = tanpa batas
    used_count int NOT NULL DEFAULT 0,
    created_by uuid NOT NULL REFERENCES users(id),
    expires_at timestamptz NOT NULL,     -- maks 7 hari, dicek di service
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE rooms (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id           uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name             text NOT NULL,
    map_key          text NOT NULL,      -- 'campus' | 'classroom' | 'garden'
    building_key     text,               -- gedung di peta sekolah; null untuk peta sekolah
    capacity         int NOT NULL DEFAULT 40,
    all_chat_enabled boolean NOT NULL DEFAULT true,
    scheduled_at     timestamptz,
    created_by       uuid NOT NULL REFERENCES users(id),
    created_at       timestamptz NOT NULL DEFAULT now(),
    archived_at      timestamptz
);
CREATE INDEX rooms_org_idx ON rooms (org_id) WHERE archived_at IS NULL;
CREATE UNIQUE INDEX rooms_campus_uq ON rooms (org_id) WHERE map_key = 'campus'; -- satu peta sekolah per organisasi

CREATE TABLE room_sessions (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id    uuid NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    started_at timestamptz NOT NULL DEFAULT now(),
    ended_at   timestamptz
);

CREATE TABLE session_attendances (
    session_id uuid NOT NULL REFERENCES room_sessions(id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at  timestamptz NOT NULL,
    left_at    timestamptz,
    PRIMARY KEY (session_id, user_id, joined_at)
);

CREATE TABLE chat_messages (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    session_id uuid NOT NULL REFERENCES room_sessions(id) ON DELETE CASCADE,
    sender_id  uuid NOT NULL REFERENCES users(id),
    scope      chat_scope NOT NULL,
    zone_key   text,
    body       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX chat_messages_created_idx ON chat_messages (created_at); -- purge > 30 hari

CREATE TABLE moderation_actions (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id    uuid NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    actor_id   uuid NOT NULL REFERENCES users(id),
    target_id  uuid REFERENCES users(id),  -- null untuk lock/unlock
    action     mod_action NOT NULL,
    reason     text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE reports (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id         uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    room_id        uuid REFERENCES rooms(id) ON DELETE SET NULL,
    reporter_id    uuid NOT NULL REFERENCES users(id),
    target_user_id uuid NOT NULL REFERENCES users(id),
    message_id     bigint REFERENCES chat_messages(id) ON DELETE SET NULL,
    reason         text NOT NULL,
    status         report_status NOT NULL DEFAULT 'open',
    reviewed_by    uuid REFERENCES users(id),
    reviewed_at    timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX reports_org_status_idx ON reports (org_id, status);
```

Catatan desain: status ruang terkunci dan daftar peserta di-mute hanya hidup di memory room (berlaku per sesi), sedangkan jejaknya tercatat di `moderation_actions`. Pesan chat dihapus otomatis setelah 30 hari oleh job harian; laporan yang menunjuk pesan tersebut tetap ada dengan `message_id` kosong.

## API contract

Kontrak REST ditulis lebih dulu di `classverse-backend/api/openapi.yaml`. Tipe TypeScript di-generate dengan `openapi-typescript` ke `classverse-frontend/src/api/schema.d.ts` dari path relatif `../classverse-backend/api/openapi.yaml` (setelah repo dipisah: URL atau artefak rilis), dan tipe Go (model saja) dengan `oapi-codegen` ke `internal/platform/api/`, sehingga frontend dan backend memakai satu sumber kebenaran. Hasil generate tidak diedit manual.

**Konvensi**

- Semua endpoint di bawah `/api`, JSON, field `snake_case`, waktu ISO 8601 UTC.
- Auth lewat header `Authorization: Bearer <access_token>`.
- List memakai cursor: `?limit=20&cursor=<opaque>`, respons `{ "items": [...], "next_cursor": "..." }`.
- Semua error memakai satu bentuk:

```json
{ "error": { "code": "invite_expired", "message": "Kode undangan sudah kedaluwarsa" } }
```

| HTTP | `code` contoh | Kapan |
| --- | --- | --- |
| 400 | `validation_failed` | Input tidak valid (detail per field di `error.fields`) |
| 401 | `unauthenticated` | Token tidak ada atau kedaluwarsa |
| 403 | `forbidden` | Role tidak cukup atau bukan anggota organisasi |
| 404 | `not_found` | Resource tidak ada atau tidak terlihat oleh user |
| 409 | `conflict` | Slug terpakai, sudah menjadi anggota |
| 410 | `invite_expired` | Undangan kedaluwarsa atau dicabut |
| 429 | `rate_limited` | Melewati batas request |
| 500 | `internal_error` | Error tak terduga; detail hanya di log server, tidak ke client |
| 503 | `unavailable` | Dependensi (database) belum siap, dipakai `/readyz` |

**Endpoint**

| Method | Path | Role minimal | Fungsi |
| --- | --- | --- | --- |
| PATCH | `/api/me` | login | Ubah nama tampilan, avatar, tanggal lahir |
| POST | `/api/orgs` | login | Buat organisasi, pembuat jadi admin |
| GET | `/api/orgs/{org_id}` | participant | Detail organisasi |
| PATCH | `/api/orgs/{org_id}` | admin | Ubah nama, logo |
| GET | `/api/orgs/{org_id}/members` | teacher | Daftar anggota |
| PATCH | `/api/orgs/{org_id}/members/{user_id}` | admin | Ubah role |
| DELETE | `/api/orgs/{org_id}/members/{user_id}` | admin | Keluarkan anggota (menutup koneksi WS-nya) |
| POST | `/api/orgs/{org_id}/invites` | teacher | Buat undangan (teacher hanya untuk role participant) |
| GET | `/api/orgs/{org_id}/invites` | teacher | Daftar undangan aktif |
| DELETE | `/api/orgs/{org_id}/invites/{invite_id}` | teacher | Cabut undangan |
| GET | `/api/invites/{code}` | login | Pratinjau: nama organisasi dan role |
| POST | `/api/invites/{code}/accept` | login | Bergabung ke organisasi |
| GET | `/api/orgs/{org_id}/rooms` | participant | Daftar ruang |
| POST | `/api/orgs/{org_id}/rooms` | teacher | Buat ruang dari map bawaan |
| GET | `/api/rooms/{room_id}` | participant | Detail ruang + status live (jumlah peserta) |
| PATCH | `/api/rooms/{room_id}` | teacher | Ubah nama, jadwal, chat semua ruangan |
| DELETE | `/api/rooms/{room_id}` | teacher | Arsipkan ruang |
| GET | `/api/rooms/{room_id}/sessions` | teacher | Riwayat sesi dan kehadiran |
| POST | `/api/reports` | participant | Laporkan user atau pesan |
| GET | `/api/orgs/{org_id}/reports` | teacher | Daftar laporan, filter status |
| PATCH | `/api/reports/{report_id}` | teacher | Ubah status laporan |
| GET | `/api/orgs/{org_id}/moderation-log` | admin | Log aksi moderasi |
| POST | `/api/rooms/{room_id}/voice-token` | participant | Token LiveKit (M2) |

Sesi ruang tidak punya endpoint start/stop: sesi dibuka otomatis saat orang pertama masuk lewat WebSocket dan ditutup setelah ruang kosong selama 5 menit.

**Organisasi dan undangan (slice 2).**

- Membuat organisasi: email terverifikasi, tanggal lahir terisi, dan usia minimal 18 tahun; selain itu 403 `forbidden`. Pengguna di bawah 18 tahun hanya masuk lewat undangan sekolah (Keamanan 7). Bergabung lewat undangan boleh untuk semua usia, tetapi tanggal lahir wajib terisi (403 `profile_incomplete`). Usia dihitung dari tanggal UTC hari ini.
- Nama organisasi 3–80 karakter. Slug dibuat dari nama dan bisa diubah: `^[a-z0-9](?:[a-z0-9-]{1,46}[a-z0-9])$`, unik (409 `conflict`). Logo hanya `logo_url` https opsional; unggah file ditunda.
- Organisasi, membership admin pembuat, dan room peta sekolah (`map_key='campus'`, nama "Peta sekolah", kapasitas 200) dibuat dalam satu transaksi.
- Kode undangan 8 karakter dari `ABCDEFGHJKMNPQRSTUVWXYZ23456789` (`crypto/rand`), ditampilkan `ABCD-EFGH`; input tidak peka huruf besar dan mengabaikan `-`/spasi. Link: `FRONTEND_URL/gabung/{code}`.
- Undangan berisi `role` (`teacher`/`participant`), `expires_in_days` 1–7 (default 7), `max_uses` opsional 1–500. Pengajar hanya membuat undangan `participant` dan hanya mencabut undangan buatannya; admin mencabut semua.
- Kode tidak ada → 404 `not_found`; kedaluwarsa, dicabut, atau kuota habis → 410 `invite_expired`; sudah anggota → 409 `conflict` tanpa mengubah role atau kuota. Pratinjau hanya berisi nama organisasi dan role. Pratinjau dan accept dibatasi 10/menit per user.
- Non-anggota mendapat 404 `not_found` untuk resource organisasi (tidak membocorkan keberadaannya); anggota dengan role kurang 403 `forbidden`. Role dibaca dari database setiap request.
- `GET /api/me` mengembalikan `orgs: [{id, name, slug, role}]`. Frontend menyimpan id organisasi aktif di `localStorage` (bukan data pribadi); server tetap memeriksa membership.

Contoh potongan OpenAPI untuk satu endpoint:

```yaml
/api/invites/{code}/accept:
  post:
    operationId: acceptInvite
    parameters:
      - { name: code, in: path, required: true, schema: { type: string } }
    responses:
      "200":
        description: Bergabung ke organisasi
        content:
          application/json:
            schema: { $ref: "#/components/schemas/Membership" }
      "409": { $ref: "#/components/responses/Conflict" }
      "410": { $ref: "#/components/responses/InviteExpired" }
```

## Slice 4 — Profil dan avatar (P4-01…P4-05 disetujui)

- Nama: Unicode NFC, trim dan gabungkan whitespace; 3–24 code point, tolak karakter kontrol/format.
  Nama lama tidak dimigrasi; validasi berlaku saat nama dikirim. Registrasi memakai validator sama;
  nama OAuth tidak valid diganti `Pengguna`. Filter awal daftar kecil milik proyek Indonesia/English,
  case-insensitive kata utuh (huruf/angka sebagai batas), bukan substring; belum mendeteksi semua obfuscation.
- `PATCH /api/me`: `display_name?`, `avatar_key?`, `birth_date?`; omitted tidak berubah, null ditolak,
  validasi atomik sebelum update. Avatar allowlist enam aksesori × `nila`/`mangga`, key `<aksesori>-<warna>`;
  `default` tetap sah dan fallback renderer. Tidak ada upload/URL bebas.
- Setelah DB commit, publish identitas melalui inbox room: `profile_updated {id, name, avatar}`.
  DB sumber kebenaran; publish gagal tidak mengubah sukses simpan. Reconnect membaca identitas terbaru.
  Room hanya memperbarui peserta yang ada, tanpa mengubah posisi/kehadiran.
- Header membuka dialog Profil: preview, nama, 12 radio avatar, Simpan/Batal; fokus native dialog,
  loading, error per field dan status sukses. Nilai tersimpan hanya diganti respons server.

## Slice 5 — Ruang dan travel (R5-01…R5-08 disetujui)

- *Akses masuk.* Semua role wajib berdiri di pintu gedung; tidak ada teleport dari daftar/kelola ruang.
  CTA kelola memakai **Lihat di peta**. Server memakai posisi resmi room asal, radius 48 px dari titik `door`.
  Ruang Guru (`staff_only`) hanya teacher/admin. Non-member dan room organisasi lain tetap `not_found`.
- *Jadwal.* `scheduled_at` null bisa dimasuki sekarang. Jadwal masa depan menolak participant (`scheduled`);
  teacher/admin boleh masuk lebih awal. **Buka sekarang** = PATCH `scheduled_at: null`. Sesi dimulai saat kedatangan
  pertama yang berhasil, bukan saat jam jadwal.
- *Peta ruang.* `classroom` dan `garden` punya layer `ground`, `collision`, `spawn`, dan objek `exit`.
  Kembali campus ke titik pintu gedung asal. Map/gedung ditetapkan saat create dan tidak bisa diubah.
- *Travel atomik.* Travel per user diserialkan; DB admission dilakukan sebelum perubahan room. Hub memesan slot tujuan,
  lalu meminta room asal melepas peserta setelah validasi posisi. Bila asal menolak/berhenti, reservasi dibatalkan dan
  peserta tetap di asal. Commit memindahkan owner koneksi ke tujuan, observer asal menerima `left`, observer tujuan
  menerima `joined`, client menerima `traveled` dengan payload `welcome` + `building_key`/`room_name`.
  Kapasitas menghitung peserta terhubung, avatar grace, dan reservasi.
- *Reconnect.* Hub menyimpan room terakhir yang sudah commit selama grace 30 detik. Reconnect apa pun dari room ID
  organisasi yang sama memulihkan room itu bila masih sah; selain itu fallback ke campus. Cold WS langsung ke non-campus
  ditolak `not_at_door`; URL WS tidak menjadi bypass.
- *Sesi dan live.* Jumlah = terhubung + grace + reservasi. Live = sesi terbuka dengan jumlah > 0. Sesi kosong
  ditutup setelah 5 menit; masuk kembali sebelum itu meneruskan sesi. Kehadiran dicatat sesudah commit dan ditutup saat
  pindah/grace habis. Penulisan DB dilakukan worker hub; kegagalan dicatat tanpa memblokir room loop.
  Index unik parsial menjamin satu sesi terbuka per room.
- *REST.* Nama ruang 3–60 code point (NFC/trim). `GET /api/orgs/{org_id}/rooms?building=&cursor=&limit=`, POST,
  GET/PATCH/DELETE `/api/rooms/{room_id}`. Kapasitas create selalu 40. PATCH hanya `name` dan `scheduled_at`.
  DELETE soft archive hanya bila tidak ada peserta/reservasi/sesi terbuka (`409 room_busy`). Campus tidak bisa dibuat,
  diubah, atau diarsip lewat endpoint ini. List/detail hanya untuk member; Ruang Guru disembunyikan dari participant.
- *WS.* `travel {room_id}`; error `room_full`, `scheduled`, `not_at_door`, `forbidden`, `not_found`, `travel_in_progress`.
  `buildings {items:[{building_key,people,live_rooms}]}` setiap 5 detik hanya untuk room campus dan sesudah filter role.

## Slice 6 — Chat dan zona (C6-01…C6-10 disetujui)

- *Dekat.* Penerima `near` = avatar terhubung dengan jarak Euclid titik kaki ≤ 160 px (5 tile) saat pesan diterima server;
  pengirim selalu menerima salinannya. Berlaku di peta sekolah juga. Avatar dalam grace reconnect tidak menerima.
- *Zona.* Layer objek `zones` pada classroom/garden: persegi dengan properti `key` unik, nama objek = label. Zona aktif =
  persegi yang memuat titik kaki, dihitung server tiap tick; perubahan dikirim `zone {key, name}` hanya ke pemiliknya
  (`key` kosong = keluar zona). Peta sekolah tanpa zona.
- *Scope.* Tab Zona (hanya saat di zona) / Dekat / Semua. `zone` ditolak `not_in_zone` jika pengirim tidak di zona saat
  diterima. `broadcast` untuk slice 7.
- *Payload.* Client `chat {text, scope}`. Server `chat {id?, from:{id,name,role}, text, scope, zone?, at}`; `id` ada bila
  pesan tersimpan. Riwayat tidak dikirim saat masuk/reconnect.
- *Batas.* 1–500 code point setelah NFC + trim; karakter kontrol ditolak kecuali baris baru (dipadatkan maks 2 berturut).
  3 pesan/detik, burst 5 (token bucket) → `rate_limited`.
- *Filter.* Disamarkan, bukan ditolak: kata kasar (daftar proyek yang sama dengan nama tampilan, kata utuh) → `****`;
  nomor HP Indonesia (`08…`, `+62…`, `62…`, 9–13 digit, boleh dipisah spasi/titik/strip) dan email → `[disembunyikan]`.
  Alamat belum disaring otomatis (keterbatasan tercatat). Yang disimpan dan disiarkan hanya hasil filter.
- *Penyimpanan.* Pesan di classroom/garden disimpan ke `chat_messages` (sesi terbuka) oleh worker hub, tanpa memblokir
  room. Chat peta sekolah tidak disimpan (tidak ada sesi). Purge harian `created_at` > 30 hari dari `cmd/server`
  (sekali saat start lalu tiap 24 jam). Log tanpa isi chat.
- *Chat semua ruangan.* PATCH `/api/rooms/{room_id}` menerima `all_chat_enabled` (teacher/admin). Hub meneruskan ke room;
  room menyiarkan `room_state {all_chat_enabled}`. Saat mati, scope `room` ditolak `all_chat_disabled`. Peta sekolah
  selalu aktif. `welcome`/`traveled` membawa `all_chat_enabled`.
- *Tampilan.* Zona: border putus-putus `--color-accent-line` + isi `--color-accent-soft` tipis; zona sendiri solid lebih
  tebal + label "· kamu di sini". Lingkaran radius dekat putus-putus Nila di avatar sendiri. Panel kanan berisi tab chat.
- *Aksesibilitas.* Log `role="log"` `aria-live="polite"`, input berlabel tersembunyi, Enter kirim / Shift+Enter baris baru,
  tombol kirim 44×44 `aria-label`. Maks 200 pesan per tab di memori.

## Berikutnya

Status Fase 2:

- [x] Data model: ERD + skema PostgreSQL (user, organisasi, membership, undangan, ruang, sesi, pesan, refresh token, log moderasi)
- [x] API contract: OpenAPI untuk endpoint REST di luar auth
- [x] Desain visual: style guide, konsep avatar, tile map, wireframe (termasuk mode HP)

Status Fase 3: kerangka repo selesai (lihat `docs/tasks/phase-3.md`).

Pertanyaan terbuka akibat ADR-010, perlu diputuskan sebelum slice yang membutuhkannya:

- [x] Lokasi `openapi.yaml`: `classverse-backend/api/openapi.yaml` (diputuskan untuk slice 2, lihat API contract).
- [x] Cara frontend mendapat file peta Tiled: backend menyajikan `GET /api/maps/{map_key}` dari `maps/` yang di-embed ke binary (diputuskan untuk slice 3, lihat Protokol real-time).

## Slice 7 — approved teacher controls and moderation

| ID | Approved behavior |
| --- | --- |
| C7-01 | M1 mute blocks participant chat in every scope; unmute restores it. Teacher/admin cannot mute or kick other teachers/admins or themselves. |
| C7-02 | Gather moves all connected avatars and grace avatars to the map's existing walkable spawn point; clears client paths/held keys and resets movement budgets. Campus gather is unavailable. |
| C7-03 | Broadcast uses filtered chat with scope broadcast, teacher/admin only, bypasses all_chat_enabled but retains length/rate limits; appears in every recipient's visible chat and a distinct announcement area. |
| C7-04 | Kick closes the target WebSocket, as existing design requires. Ban is keyed by room session and user; reconnect can enter campus but cannot enter the banned session. Add explicit teacher command allow to revoke the ban, with audit. |
| C7-05 | Lock/unlock affects participants entering classroom/garden; teachers/admins and an existing participant reconnecting within grace may enter. Campus cannot be locked. |
| C7-06 | Muted IDs, bans and lock survive empty-room actor cleanup for the remainder of the open session. Process restart must not bypass session bans: persist bans by session. This requires schema/design extension; do not silently treat room-actor lifetime as session lifetime. |
| C7-07 | Reports accept a target user, optional room/message, and reason 1–1000 code points. Server derives message sender/room/org from the stored message, validates organization access and target membership; no client-supplied chat copy is stored. Campus or unsaved chat can report user only. |
| C7-08 | Teacher/admin can list and review reports within organization; admin alone reads audit log. Audit writes must succeed before acknowledging moderation success; return unavailable on persistence failure. Report text is not written to application logs. |

### Slice 7 — implementation notes

- Reports and the audit log live in package `report` (not `moderation`, which stays the pure text filter used by
  auth and realtime) to avoid mixing HTTP/DB code into a dependency-free package.
- `allow` was added to `mod_action` (migration 00003) to audit revoking a kick; bans are rows of `session_bans`
  keyed by session, so they end with the session and survive restarts (sessions are restored on startup).
- Kick, allow, mute, unmute, lock and unlock take effect only after their audit row commits; a failed write
  changes nothing and answers `unavailable`.

## Slice 8 — Mode HP (M8-01…M8-06 approved)

- *Detection.* Mode HP is automatic when `matchMedia('(pointer: coarse) and (max-width: 767px)')` matches at room
  start; no manual toggle. Phaser is not loaded. Tablets keep the map (best effort).
- *Protocol.* `auth {token, mode?}` with `mode` `map` (default) or `list`; any other value closes 4401. Fixed per
  connection, survives travel; reconnect re-declares. `move` from a list connection → `move_rejected` with the
  official position.
- *Travel.* List connections skip the door/exit proximity check (exception to slice 5 *Akses masuk*). Membership,
  Ruang Guru, schedule, capacity, lock, ban and travel serialization still apply. Arrival at spawn, return to the
  origin building door as before. Accepted risk: a desktop client can forge `list` to skip walking; the door is
  presence UX, not access control.
- *Seat.* Client → server `seat {zone}` (list mode, classroom/garden only, 1 per second → `rate_limited`). Server
  places the avatar on a walkable tile inside the zone (zone centres are tables, so not the literal centre; spread
  like spawn); `zone: ""` returns to the spawn ("Area umum"). Unknown zone → `bad_message`; campus or map
  connection → `forbidden`. Zone/snapshot follow on the next tick; gather still moves everyone to spawn. Map parsing
  rejects zones without a walkable tile.
- *Client-side counts.* Zone counts, tablemates and "Di dekatmu" (≤160 px) are computed from welcome/snapshot
  positions; building counts reuse `buildings`, room rows reuse the rooms list (`people`, `live`).
- *Teacher.* Phone keeps the announcement tab and the teacher panel (collapsible).

## Slice 9 — Voice (V9-01…V9-07 approved; V9-03 revised by the user)

- *Where.* Classroom/garden only; campus and mode HP stay text-only. One LiveKit room per Kelasverse room
  (name = room UUID), identity = user UUID, name = display name.
- *Channels (V9-03).* No distance volume. Channel = the zone the avatar stands in, or "area umum" outside every zone.
  Everyone in the same channel is heard at full volume, plus staff in `voice_broadcast`. Enforced by the client
  (`autoSubscribe: false`, subscriptions resynced every 250 ms from server positions). Accepted risk: a modified
  client inside the same room could subscribe to other channels of that room.
- *Token.* `POST /api/rooms/{room_id}/voice-token` → `{url, token}` only while the caller has a live WS connection
  in that classroom/garden (hub probe); otherwise 404. 503 `voice_unavailable` when LiveKit is not configured.
  HS256 JWT signed with the LiveKit secret, claims as `livekit/protocol/auth`: `roomJoin`, `room`, `canSubscribe`,
  `canPublish` (false while muted), `canPublishSources: [microphone]`, no data/metadata. TTL 2 minutes (self-host has
  no revocation; LiveKit refreshes connected participants). `Cache-Control: no-store`.
- *Moderation.* After a mute/unmute audit commits, the hub voice worker calls RoomService `UpdateParticipant`
  (`can_publish`). Every removal from a classroom/garden (travel, grace expiry, kick) calls `RemoveParticipant`.
  Calls are best effort, logged on failure, never block a room; `not_found` (already gone) is success.
- *Broadcast voice.* `cmd voice_broadcast` / `voice_broadcast_off` (staff, not campus, not audited). The room keeps
  the set; `welcome` and `room_state` carry `voice_broadcast: [user_id]`. Leaving the room clears it.
- *Config.* `LIVEKIT_URL` (`ws(s)://`), `LIVEKIT_API_KEY`, `LIVEKIT_API_SECRET`; all empty disables voice (mic hidden).
  RoomService base URL is derived from `LIVEKIT_URL` (`ws`→`http`). Dev: `livekit/livekit-server:v1.9 --dev` in
  `compose.yaml` (7880 signal/API, 7881 TCP, 7882/udp media). Production self-host vs Cloud is decided at deploy.
  No new Go dependency; frontend `livekit-client` is a lazy chunk loaded only in a room.
- *UI.* Mic 52×52 round Tinta (`aria-pressed`, off on entry and after travel), status line naming the heard channel,
  teacher switch "Bicara ke semua ruang", "Aktifkan suara" when autoplay is blocked, `--color-speaking` ring on
  active speakers (LiveKit active speakers).

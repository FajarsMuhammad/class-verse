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

Setiap fitur adalah satu package dengan lapis `handler` (HTTP/WS), `service` (otorisasi + aturan bisnis), dan akses data lewat `sqlc` Queries (ADR-014): `store.go` opsional. Package fitur tidak saling mengimpor; kebutuhan lintas fitur lewat interface kecil di sisi pemakai. Pola kode rinci: [`architecture.md`](architecture.md).

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
│   ├── assets/avatars/       # legacy SVG + sporty PNG spritesheets
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
| `auth` | `token`, `mode?` (`map`/`list`) | Validasi JWT dan membership organisasi; `list` warisan mode HP (slice 8, tidak dipakai klien web) |
| `move` | `x`, `y`, `dir` | Maks 15 pesan/detik; ditolak jika menembus collision atau melebihi kecepatan maksimum |
| `chat` | `text`, `scope` (`near` / `zone` / `room`) | Maks 3 pesan/detik; difilter; `room` ditolak jika chat semua ruangan dimatikan |
| `emote` | `kind` (`wave`, `hand`, `quiz`, `cheer`) | Maks 1/detik; ditolak saat dibisukan; hanya koneksi aktif |
| `cmd` | `name` (`broadcast`, `gather`, `mute`, `mute_all`, `unmute_all`, `kick`, `lock`, `voice_broadcast`, `voice_broadcast_off`), `target?` | Hanya role pengajar/admin |
| `ping` | `ts` | Dibalas `pong` untuk mengukur latency |
| `travel` | `room_id` | Hanya ke ruang yang boleh dimasuki dan saat avatar berada di pintu gedung (kecuali koneksi `list` warisan); koneksi dipindah tanpa reconnect |
| `seat` | `zone` | Koneksi `list` warisan saja, di kelas/taman; maks 1/detik (slice 8, tidak dipakai klien web) |

**Server → client**

| `t` | Payload | Kapan |
| --- | --- | --- |
| `welcome` | id diri, map, daftar peserta, state ruang | Setelah `auth` berhasil |
| `snapshot` | daftar `{id, x, y, dir}` yang berubah | Setiap tick (100 ms), hanya jika ada perubahan |
| `joined` / `left` | data peserta | Peserta masuk/keluar |
| `zone` | `zoneId` atau kosong | Peserta masuk/keluar zona |
| `chat` | `from`, `text`, `scope`, `at` | Dikirim hanya ke penerima yang memenuhi radius/zona |
| `event` | `name`, `by`, data | Hasil `cmd` pengajar (misalnya gather, mute) |
| `emote` | `id`, `kind` | Reaksi sementara 3 detik; satu ruang, campus hanya area of interest. Tidak disimpan atau diputar ulang saat reconnect/travel |
| `error` | `code`, `message`, `seq` | Pesan ditolak |
| `traveled` | isi welcome untuk ruang tujuan | Setelah travel berhasil, termasuk saat kembali ke peta sekolah |
| `buildings` | jumlah orang dan kelas live per gedung | Setiap 5 detik, hanya di peta sekolah |

**Model eksekusi.** Setiap room berjalan di satu goroutine dengan inbox channel dan ticker 100 ms, sehingga state room tidak butuh mutex. Setiap koneksi punya goroutine tulis dengan buffer 64 pesan; jika buffer penuh, koneksi dianggap lambat dan ditutup agar tidak menghambat room.

**Reconnect.** Client otomatis menyambung ulang dengan backoff 1, 2, 4, 8 detik. Jika kembali dalam 30 detik, posisi terakhir dipertahankan dan peserta lain tidak melihat `left`.

**Proximity.** Penerima chat `near` dihitung server dari jarak Euclid titik kaki. Properti peta Tiled `near_radius` (px, 16–320) menentukan radius; MVP 2 memakai 85 px di semua peta (paket desain revamp). Peta tanpa `near_radius` masih memakai `near_tiles` × ukuran tile (rentang 1–10, default 5). Server dan frontend membaca properti yang sama. Zona dibaca dari layer objek di file Tiled. Interior berkapasitas 40 tetap full-room. Snapshot idle dilewati setelah visibility tersinkronisasi; kampus memakai indeks sel AOI ketika ada perubahan. Receiver hanya mengambil kandidat sel sekitar serta memeriksa daftar `seen` untuk avatar keluar; konversi UUID posisi dibagi per tick. Kerumunan semua-terlihat tetap berbiaya kuadratik. Benchmark dan batas kapasitas: [`tasks/realtime-capacity.md`](tasks/realtime-capacity.md).

**Peta sekolah.** Setiap organisasi punya satu room dengan `map_key = 'campus'`, dibuat otomatis saat organisasi dibuat; semua peserta muncul di sini setelah masuk. Kapasitasnya 200, jadi `snapshot` hanya berisi avatar dalam radius 24 tile dari penerima (area of interest berbasis grid sel 8×8 tile), agar bandwidth per peserta tetap kecil. Gedung dan pintunya didefinisikan di layer objek file Tiled peta sekolah; setiap ruang menyimpan `building_key` gedungnya. Satu koneksi WebSocket dipakai sepanjang sesi: `travel` memindahkan peserta antar room di dalam hub, dan keluar dari ruang mengembalikannya ke pintu gedung asal.

**Detail slice 3 (real-time inti).**

- *Peta.* File Tiled `.tmj` ada di `classverse-backend/maps/`, di-embed ke binary, dan disajikan `GET /api/maps/{map_key}` (publik, bukan data organisasi; `ETag` + `Cache-Control: no-cache`). Server dan Phaser membaca file yang sama. Tile 32 px. Layer tile `ground`, layer tile `collision` (tile ≠ 0 tidak bisa dilewati), layer objek `spawn` (persegi) dan `buildings` (persegi per gedung dengan properti `key`, `name`, `roof`, `staff_only`, plus objek titik `door` yang properti `building`-nya menunjuk `key`). Urutan id tile adalah kontrak di `maps/README.md`. Revisi D3-02 disetujui sesudah review visual: frontend menggambar ilustrasi flat utuh beresolusi 2× dari token `--map-*`/`--roof-*` dan bounds gedung Tiled, bukan atlas tile untuk tampilan gedung. Collision/spawn/pintu tetap mengikuti Tiled. Kamera overview menampilkan seluruh sekolah dan menyesuaikan resize; avatar dan label mempertahankan ukuran layar.
- *Room peta sekolah.* `GET /api/orgs/{org_id}` mengembalikan `campus_room_id`; client membuka `/ws/rooms/{campus_room_id}`.
- *Auth WebSocket.* Upgrade hanya dari Origin `FRONTEND_URL` (plus `DEV_ALLOWED_ORIGINS` di development). Pesan pertama wajib `auth {token}` dalam 5 detik; server memverifikasi JWT dan membership organisasi pemilik room lalu mengirim `welcome`. Gagal → `error` lalu close 4401 (token), 4403 (tidak boleh), 4404 (room tidak ada/bukan anggota).
- *Satu koneksi per user.* Koneksi baru user yang sama menggantikan yang lama; yang lama ditutup dengan 4409 `replaced` dan client tidak reconnect otomatis.
- *Movement.* Koordinat pixel dunia (integer, titik kaki avatar), `dir` `up`/`down`/`left`/`right`. Kecepatan maksimum per peta dari properti peta Tiled `walk_speed` (px/detik, bilangan bulat 80–400, default 160; disetujui setelah review slice 5 karena peta sekolah tampil di zoom ±0,4 sehingga 160 px/detik terasa lambat): `campus` 320, interior (`classroom`/`garden`/`lab`/`baca`/`library`) 160. Batas dunia memakai `world_width`/`world_height` (px) bila ada; MVP 2: kampus 2200×1500, interior 1044×684 (lantai 1000×640 + dinding 22 px), kecuali `library` 1244×764 (lantai 1200×720 dari `floor_size`).
  Peta dengan properti `quiet: true` (`library`) adalah zona tenang: server tidak memberi identitas voice, `voice_broadcast`
  ditolak `forbidden`, chat tetap jalan. Server dan client membaca angka yang sama dari file peta. Kecepatan ditegakkan sebagai budget jarak (token bucket) agar jitter jaringan tidak menolak gerak sah: budget bertambah `walk_speed` × 1,25 px per detik, maksimal setengah detik gerak, dan setiap `move` memakai jaraknya; `move` diterima jika jarak ≤ budget + 8 px, tujuan bukan tile collision dan di dalam peta, dan lintasan (dicek tiap 8 px) tidak melewati collision. Lebih dari 15 `move`/detik dibuang. Ditolak → `error {code: "move_rejected", seq, x, y}` berisi posisi resmi dan menghentikan jalur klik. Koreksi posisi setelah perpindahan zona voice berhasil memakai envelope sama dengan `continue_path: true` agar jalur klik tetap berlanjut; field ini tidak dikirim untuk penolakan.
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
| Refresh token | String acak 32 byte (opaque) | 30 hari, geser saat dipakai | Cookie `kv_refresh` dengan `HttpOnly; Secure; SameSite=Strict; Path=/api/auth` (development: origin `DEV_ALLOWED_ORIGINS` berskema `http://` tanpa `Secure`, ditolak di production); di database hanya hash SHA-256 |

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
| ADR-011 | Redis hanya untuk leaderboard mingguan (sorted set per organisasi per minggu WIB); Postgres `point_events` tetap sumber kebenaran | Top-N dan peringkat per user O(log n) tanpa agregasi `SUM` tiap request; reset mingguan lewat kunci berganti tanpa job; bisa dibangun ulang dari Postgres | Redis dipakai untuk state room (ADR-002) atau data yang tidak bisa dibangun ulang |

## ADR-014 — Lapis data adalah sqlc Queries; `store.go` opsional (disetujui)

**Konteks.** Desain awal menetapkan `store.go` per fitur. Dalam praktik setiap `store.go` hanya `struct{ *query.Queries }`
plus `InTx`, bertipe konkret, dan tidak pernah di-fake; fitur yang lebih baru (assignment, class, library, module, quiz)
memanggil `query.New` langsung di service. Yang benar-benar berulang adalah helper transaksi (8 salinan).

**Keputusan.**
- Lapis data = query `sqlc` (`internal/platform/db/query`). Service boleh memegang `*pgxpool.Pool` dan memanggil
  `query.New(pool)` / `db.InTx(ctx, pool, fn)`.
- `store.go` hanya dibuat bila ada logika akses data yang dipakai bersama beberapa method (bukan wrapper kosong).
  `store.go` yang ada tidak dipindah.
- Transaksi hanya lewat `db.InTx`; konversi umum `db.Time`, pencarian `ILIKE` literal `db.Contains`.
- Interface hanya untuk dependensi lintas fitur atau sistem luar (Authorizer, Mailer, Uploader, Occupancy), didefinisikan
  di sisi pemakai. Perilaku SQL dan otorisasi diuji dengan Postgres asli (`dbtest`), bukan mock query.

**Konsekuensi.** Lebih sedikit lapisan tipis; aturan bisnis yang bergantung SQL hanya teruji lewat integration test.
**Dievaluasi ulang jika** perlu mengganti penyimpanan (bukan Postgres) atau tes service tanpa database menjadi kebutuhan.

## ADR-013 — Draft dan revisi materi immutable (CS, disetujui)

Menggantikan keputusan M1/M2 2026-11-13 tentang edit modul terbit langsung. Content Studio tetap
memakai modul/langkah/kuis existing, bukan course/lesson/enrollment baru. Rich text terstruktur wajib;
plain text, berkas, video, dan kuis lama tetap terbaca. Template hanya struktur awal satu editor/renderer.

Target implementasi: working draft berversi optimistik terpisah dari snapshot terbit immutable. Modul
mengikat revisi kuis terbit yang dipilih; publish modul tidak menerbitkan draft kuis bersama secara implisit.
Progres memin revisi modul, attempt tetap memakai snapshot, tugas tetap memakai konteks issuance sendiri.
Backfill additive mempertahankan ID langkah, riwayat, hasil, Rapor dan akses file. Revisi bukan bypass
membership/resource authorization. Tidak menghitung ulang nilai lama atau mereset completion.

Kebijakan Q1 tetap default lulus 60, unlimited attempts, bobot sama/floor, reveal setelah finish. Batas
attempt, bobot berbeda dan perubahan reveal bukan bagian persetujuan ini. Implementasi CS lokal selesai, belum deploy;
upgrade wajib `make migrate` (preflight antara migration 20 dan 21). Rencana dan gate ada di
[`class-verse_content_studio_implementation_plan.md`](tasks/class-verse_content_studio_implementation_plan.md).

## ADR-012 — Prinsipal tamu pertemuan (G1)

**Disetujui:** tamu mengikuti satu ruang lewat tautan dan persetujuan guru/admin sekolah ruang itu,
terpisah dari akun anggota dan kelas/rombel. Implementasi memakai `users.is_guest` agar attendance,
chat, moderation, dan voice tetap memakai satu `user_id`. Email tamu `NULL`; CHECK dan trigger database
menolak kredensial login, tanggal lahir, membership, OTP, serta refresh token untuk tamu. Lookup akun
mengecualikan tamu. Tidak ada konversi tamu menjadi akun pada G1.

`guest_links` mengikat organisasi dan ruang dengan FK komposit; bukan kampus atau Ruang Guru.
Link maksimal 24 jam dan 40 identitas tamu, tetap dibatasi kapasitas ruang bersama anggota.
Slot terpakai saat permintaan pertama dibuat; penolakan tidak mengembalikan slot. Retry dengan
UUID v4 rahasia yang sama memakai identitas lama tanpa slot tambahan. Server menyimpan hash kunci;
kunci tidak masuk URL/log, dan tidak bisa dipakai untuk link lain. Browser menyimpan kredensial hanya
in-memory; reload memulai permintaan baru. Usia tidak diminta atau disimpan; host/sekolah tetap
bertanggung jawab atas persetujuan wali dan kelayakan undangan.

Approval/deny dan revoke diserialkan lewat lock baris link. Keputusan dan audit ditulis satu transaksi.
`guest_decisions` terpisah karena audit admission merujuk link dan target opsional (revoke link),
sedangkan `moderation_actions` tetap memegang kick/mute sesi. Bukan perubahan enum moderasi lama.

JWT tamu memakai audience `kelasverse-guest`, paling lama 15 menit atau sisa usia link. Hanya endpoint
`/api/guest`, socket ruang tujuan, dan voice ruang itu menerimanya; API akun menolak audience tamu.
Polling status memperbarui token selama grant aktif. Admission diperiksa setiap koneksi/reconnect;
`travel` ditolak. Ruang mengakhiri presence live maupun yang ditahan saat link habis/dicabut,
sekaligus mencabut voice. Kick memakai ban sesi yang sama dengan anggota.

Frontend `/guest/{code}` berada di luar guard akun, dengan shell ruang minimal tanpa data belajar,
menu sekolah, atau navigasi antar-ruang. Peta, chat dan voice tersedia di semua lebar layar; pada layar
sempit peta mengisi bagian atas dan chat di bawahnya. Route lama belum dimigrasi.

Migration `00019` mengubah email nullable tanpa mengubah data anggota. Rollback menolak berjalan
bila identitas tamu sudah ada: penghapusan/export/retensi data memerlukan persetujuan terpisah.
Sebelum pilot, tinjauan PDP/persetujuan wali dan pengujian perangkat fisik tetap wajib.

## MVP 2 fitur belajar — keputusan dan batas perencanaan

Status: F1, L1, L2, M1 selesai; M2 progres modul diimplementasikan. Rencana dan checklist tunggal:
[`docs/tasks/mvp2-learning-plan.md`](tasks/mvp2-learning-plan.md). Keputusan ini tidak mengubah ADR-002:
state room tetap in-memory; Redis hanya proyeksi leaderboard (ADR-011).

- **Berkas:** MinIO dipilih; verifikasi upstream menghentikan penggunaan OSS legacy yang diarsipkan.
  Setelah evaluasi dan license dari pengguna, fondasi dev memakai **AIStor Free** single-node, image dipin digest
  di profile Compose `storage`. PDF + audio dibaca di aplikasi, EPUB ditunda, tanpa RAG. Metadata dan kuota di
  Postgres, objek privat di MinIO. Kuota **5.000.000.000 byte/org**, batas **50.000.000 byte/file**; reservasi
  upload ikut dihitung. Batas kapasitas VPS tetap perlu dipantau di luar kuota per sekolah.
- **Urutan:** F1 → L1 → L2 → M1 → M2 → Q1 → C1 → A1 → R1 → Q2 → B1 → B2. Langkah kuis ditambahkan setelah Q1, riwayat
  Arena di Rapor setelah Q2; migration tidak boleh bergantung tabel dari slice mendatang.
- **Leaderboard:** klasemen live per sesi kuis; ranking mingguan sekolah dengan filter gedung/mapel di Redis.
  Progres per modul di Rapor, ranking rombel ditunda. UTC untuk penyimpanan; minggu ditentukan `Asia/Jakarta`.
- **Identitas leaderboard:** inisial, detail hover/focus/tap nama lengkap + kelas; hanya anggota organisasi.
  `users.display_name` bukan jaminan nama legal; sumber nama tooltip masih perlu konfirmasi. Kelas berasal dari
  C1, bukan label bebas membership dan bukan room aktif.
- **Kelas/rombel (C1):** satu kelas aktif per siswa per organisasi per tahun ajaran. Migration `00014` memakai
  `academic_years`, `classes`, `class_enrollments` dengan FK komposit organisasi dan unique parsial penempatan
  aktif `(org_id, academic_year_id, user_id) WHERE ended_at IS NULL`. Pindah kelas atomik, riwayat tetap;
  pengajar/admin mengelola roster, admin memilih satu tahun ajaran aktif per sekolah (disetujui). Tahun lama
  tetap tersedia untuk Rapor, tanpa kenaikan otomatis. `internal/class` mengelola tahun, kelas, roster peserta,
  dan riwayat; panel Kelola memakai endpoint staff-only. Admin membuat tahun berlabel `YYYY/YYYY+1` dan
  mengaktifkan satu tahun; tahun pertama otomatis aktif. Pengajar/admin membuat/mengganti nama kelas dan
  mengarsipkan kelas kosong saja. Perpindahan mengunci membership peserta lalu kelas tujuan; arsip mengunci
  kelas sebelum menghitung roster. Unique parsial menjadi pengaman terakhir terhadap penempatan ganda.
  `PUT .../academic-years/{year_id}/roster/{user_id}` wajib `class_id` UUID atau `null` eksplisit untuk keluar;
  penempatan sama idempotent, penempatan lama ditutup dalam transaksi. Hasil belajar tidak diubah.
  Nama kelas unik tanpa beda huruf besar per tahun di antara kelas nonarsip. Riwayat menyimpan ID kelas
  (nama mengikuti perubahan nama), waktu masuk/keluar dan pembuat penempatan; membership deletion CASCADE.
  Belum ada tooltip kelas, assignment, auto-promosi, atau pembatasan room/katalog berdasarkan kelas.
  Tidak mengubah izin masuk ruang atau membuat leaderboard per kelas secara otomatis.
- **Modul (M1/M2):** katalog terbit per gedung/jenjang terbuka bagi anggota sekolah dari mana saja;
  bukan dibatasi rombel atau lokasi avatar (disetujui 2026-11-13). Runtime sebelum CS: modul terbit diedit langsung;
  langkah hanya diarsip. ADR-013: draft/revisi immutable terimplementasi lokal; belum deployed. `module_progress(org_id,user_id,step_id,completed_at)` menyimpan tanda selesai
  pemanggil; FK komposit ke membership dan langkah satu organisasi. Selesai idempotent mempertahankan waktu pertama,
  batal selesai menghapus tanda; server memeriksa akses dan mengunci modul dalam transaksi yang sama.
  Semua langkah bebas dibuka, termasuk yang belum selesai. Total menghitung langkah aktif saja. Lanjutkan belajar
  memilih modul terbit dengan tanda selesai terbaru dan langkah aktif belum selesai; tanpa aktivitas selesai
  tidak ada rekomendasi. Arsip tidak menghapus riwayat progres; keluar membership menghapus progres sesuai FK.
- **A1 (migration 00015–00016):** tugaskan modul ke rombel (tenggat opsional) dan sematkan modul/langkah saat sesi ruang;
  tidak menutup katalog bagi anggota lain. Urutan wajib berlaku untuk modul yang ditugaskan; detail skema
  dan aturan penugasan ada di `docs/tasks/a1-assignments.md`. Tugas menyimpan snapshot penerima/isi/order, progres
  terpisah monotonic dengan check urutan server; issuance retry-safe memakai UUID client. Lock roster school/year
  menyerialkan issuance terhadap perpindahan. Quiz attempt memiliki konteks tugas/step nullable; unique open attempt
  per quiz/user/context (`NULLS NOT DISTINCT`) menjaga katalog dan tugas terpisah. Pass konteks hanya menandai
  tugas, pass katalog tetap mengisi M2. Snapshot file tetap menjadi referensi delete dan aksesnya memeriksa penerima
  serta predecessor. Sematan disimpan di sesi, disebar `room_state.pin`; worker sesi existing melakukan DB I/O,
  bukan actor. `learning_pin` target module + optional step; `learning_unpin` menghapus. New session tanpa sematan.
- **Antre buku:** ditunda; tidak ada reservasi/denda dalam L2.

Rincian teknis **masih usulan**, tidak menimpa ADR lama: transport baca autentikasi/range (proxy vs presigned)
perlu spike; PDF.js/Redis belum dipin. SDK storage `minio-go/v7` v7.4.1 sudah diuji di AIStor lokal. Redis memakai proyeksi idempotent total absolut dari
Postgres dengan pekerjaan durable, bukan increment buta setelah commit; detail worker/rebuild harus diuji.
Nama package mengikuti domain (`file`, `library`, `module`, `quiz`, `leaderboard`). Package `report` yang sudah
ada menangani laporan moderasi, bukan Rapor. R1 memakai `internal/gradebook` read-only: `GET /api/orgs/{org_id}/gradebook`
(staf, paging), `/gradebook/me`, `/gradebook/students/{user_id}`, `/gradebook/csv` (staf, CSV formula-safe).
Tidak menambah service generik, ORM, atau akses langsung store fitur lain.

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
    map_key          text NOT NULL,      -- 'campus' | 'classroom' | 'garden' | 'lab' | 'baca' | 'library' (MVP 2)
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
- Kode tidak ada → 404 `not_found`; kedaluwarsa, dicabut, atau kuota habis → 410 `invite_expired`; undangan sekolah untuk yang sudah anggota → 409 `conflict` tanpa mengubah role atau kuota. Pratinjau dan accept dibatasi 10/menit per user.
- C2 undangan berkelas (migration 00018): `class_id` opsional, kelas aktif sekolah yang sama, hanya `participant`. Admin atau wali kelas aktif boleh membuatnya dari detail Kelas → Siswa. Pratinjau menyertakan nama kelas dan tahun ajaran. Saat diterima, server memeriksa ulang kelas serta hak pembuat, memakai lock roster yang sama dengan penempatan kelas. Keanggotaan tetap berhasil bila penempatan gagal aman: `placement_status` = `has_class`, `class_unavailable`, atau `not_student`; kelas/peran lama tidak diubah. `placed` berarti sudah/berhasil ditempatkan di kelas tujuan. Anggota peserta lama boleh menerima undangan kelas, tetapi penerimaan ulang yang tidak menambah keanggotaan/penempatan tidak memakai kuota lagi. Undangan sekolah tanpa kelas tidak berubah.
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
  validasi atomik sebelum update. Revisi US-04 setelah slice 9: avatar sporty memakai key
  `sporty-<gaya>-<warna>`; gaya `runner`, `cap`, `headphones`, `varsity`, `ponytail`, `hoodie`, `glasses`,
  warna `nila`, `mangga`, `zamrud`, `laut`, `koral`, `grafit` (42 kombinasi).
  Allowlist lama enam aksesori × `nila`/`mangga` dan `default` tetap diterima dan dirender tanpa migrasi paksa.
  Tidak ada upload/URL bebas. Kontrak OpenAPI dan tipe kedua project mencakup kedua kelompok.
- Setelah DB commit, publish identitas melalui inbox room: `profile_updated {id, name, avatar}`.
  DB sumber kebenaran; publish gagal tidak mengubah sukses simpan. Reconnect membaca identitas terbaru.
  Room hanya memperbarui peserta yang ada, tanpa mengubah posisi/kehadiran.
- Header membuka dialog Profil: preview, nama, tujuh radio gaya dan enam radio warna, Simpan/Batal;
  fokus native dialog, loading, error per field dan status sukses. Pilihan avatar lama tidak diganti jika
  pengguna hanya mengubah nama. Nilai tersimpan hanya diganti respons server.
- Peta memuat PNG spritesheet 32×48 px/frame, baris depan/kiri/kanan/belakang, lima kolom (diam + empat
  frame jalan 8 fps). Ukuran layar 32×48 px, anchor kaki (16,46), nearest-neighbor. Animasi mengikuti perubahan
  posisi render, bukan tombol saja; berhenti/kurangi gerakan menampilkan frame diam sesuai arah.
  Aset tidak mengubah collision, koordinat, protokol movement, atau voice. Avatar SVG lama tetap 36×36 px.

## Slice 5 — Ruang dan travel (R5-01…R5-08 disetujui)

- *Akses masuk.* Semua role wajib berdiri di pintu gedung; tidak ada teleport dari daftar/kelola ruang.
  CTA kelola memakai **Lihat di peta**. Server memakai posisi resmi room asal, radius `interact_radius` gedung dari titik `door` (16–200 px, default 48;
  MVP 2: 60 px, Arena 180 px; nilai di luar rentang menggagalkan load peta). Keluar ruang tetap 48 px dari titik `exit`.
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

- *Dekat.* Penerima `near` = avatar terhubung dengan jarak Euclid titik kaki ≤ radius peta saat pesan diterima server:
  85 px di semua peta MVP 2, dari properti Tiled `near_radius`; peta lama tanpa properti itu memakai `near_tiles` × tile.
  Radius tidak mengubah isolasi suara atau chat zona.
  Pengirim selalu menerima salinannya. Avatar dalam grace reconnect tidak menerima.
- *Zona.* Layer objek `zones` pada peta interior: properti `key` unik, nama objek = label; `shape: "circle"` berarti elips di
  dalam kotak objek (MVP 2 memakai lingkaran voice dari `rooms.json`), tanpa properti itu persegi. Zona aktif = bentuk yang memuat titik kaki, dihitung server tiap tick; perubahan dikirim `zone {key, name}` hanya ke pemiliknya
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

## Slice 8 — Mode HP (dicabut: HP memakai peta)

Mode HP (daftar tempat tanpa peta, `MobileRoom`) **dihapus pada revamp tugas/modul**: HP sekarang memakai peta
yang sama dengan desktop, karena interaksi peta sudah berbasis ketukan (ketuk gedung = pilih, ketuk dua kali =
jalan, seret = geser) dan beban render peta sudah diturunkan (tekstur 1x, lihat `docs/tasks/realtime-capacity.md`).

- *Tata letak sempit (≤ 900 px).* Rail menu di atas; membuka panel menutupi peta, menutupnya mengembalikan peta
  setinggi layar. Masuk ke ruang dimulai pada peta (panel tertutup). Kamera berhenti di zoom 0.3, jadi kampus
  digeser, bukan dikecilkan sampai gedung tak terbaca.
- *Yang masih berlaku dari slice ini:* protokol `auth {token, mode?}` dengan `mode` `map`/`list` dan perintah
  `seat {zone}` tetap ada di server dan tetap diuji, tetapi **tidak dipakai klien web**. Klien selalu mengirim
  `map`. Keduanya dipertahankan agar klien lama tidak rusak; hapus hanya lewat keputusan terpisah.
- *Akibat.* Masuk ruang di HP kini wajib berjalan ke pintu (tidak ada lagi pengecualian proximity), dan memilih
  kursi memakai pemilih kursi + jalan kaki, sama dengan desktop. Voice di HP memakai jalur yang sama dengan
  desktop; dukungan perangkat belum diuji luas.

## Slice 9 — Voice (self-hosted MVP; implemented, awaiting review)

The user approved server-enforced zone audio without distance attenuation, confirmed eviction with visible pending/failure handling, and self-hosted LiveKit. `docs/spec.md` records the requirements.

- *Where.* Classroom/garden only; the campus stays text-only. Each actor incarnation has a random epoch.
  Zone room names are `<room UUID>:<epoch>:zone:<zone key>` (outside zones: `general`); broadcast room is
  `<room UUID>:<epoch>:broadcast`. A new incarnation never reuses the audio rooms of a crashed process.
- *Channels.* Full volume within a zone. Room-scoped signed grants enforce isolation in LiveKit. Browser connects
  to zone and broadcast rooms with `autoSubscribe: true`; there is no client distance/subscription filter.
  Only an active staff broadcaster can publish in the broadcast room; while broadcasting, zone publication is disabled.
- *Token/admission.* `POST /api/rooms/{room_id}/voice-token` returns `{url, token, broadcast_token, identity, channel}`
  only for a live WS member (404 otherwise); 409 `voice_pending` while prior media eviction is outstanding,
  503 `voice_unavailable` when disabled. Identity is `<user UUID>:<random admission UUID>`, changed after every revocation.
  Microphone-only HS256 grants, no data/metadata, TTL 2 minutes, `Cache-Control: no-store`. TTL alone is not revocation.
  Gateway checks signature, issuer, expiry, current actor identity/channel and publish permission on every join/reconnect,
  including refreshed LiveKit tokens. RoomService endpoints and alternate authorization headers are not forwarded publicly.
- *Transitions/moderation.* Cross-zone movement waits for prior media eviction before authoritative destination is committed;
  no room goroutine blocks on network I/O. Gather/seat follow the same boundary. Broadcast toggles and audited mute/unmute
  revoke the old admission and reconnect with new grants. Travel/kick revoke immediately; the hub blocks new voice admissions
  across room actors until cleanup completes. Kick keeps its existing audited ban behavior.
  Room retains cleanup jobs across failures and queue backpressure, retries each second, and exposes `voice_pending` IDs to staff.
  Worker removes zone and broadcast participants and confirms absence via `GetParticipant`; failure is never reported as success.
  Gateway handshake and eviction share an admission lock, preventing an in-flight handshake from landing after absence confirmation.
  `welcome`/`room_state` carry personal `voice_identity`, `voice_channel`, and staff-facing `voice_pending`.
  During a control API outage an existing media connection may remain live: staff sees unconfirmed eviction until recovery.
- *Bisukan semua.* `cmd mute_all` / `unmute_all` (staff, not campus). The room expands it into one ordinary `mute`/`unmute` per participant or guest currently inside, so each change is audited separately and applies only after its audit row commits. Staff are never muted; people joining later are not muted. When nobody needs changing the sender gets `bad_message`.
- *Broadcast voice.* `cmd voice_broadcast` / `voice_broadcast_off` (staff, not campus, not audited). The room keeps
  the set; `welcome` and `room_state` carry `voice_broadcast: [user_id]`. Leaving the room clears it.
- *Config.* `LIVEKIT_URL` (private `ws(s)://` upstream), `LIVEKIT_API_KEY`, `LIVEKIT_API_SECRET`; all empty disables voice (mic hidden).
  Public signaling uses `FRONTEND_URL/voice` through Go's admission gateway; the LiveKit signaling/API port must not be publicly reachable. RoomService base URL is derived from `LIVEKIT_URL` (`ws`→`http`). Dev: `livekit/livekit-server:v1.9.12 --dev` in
  `compose.yaml` (loopback-only 7880 signal/API, 7881 TCP, 7882/udp media). MVP uses self-hosted LiveKit; production ingress/TLS remains to be configured.
  No new Go dependency; frontend `livekit-client` is a lazy chunk loaded only in a room.
- *UI.* Mic 52×52 round Tinta (`aria-pressed`, off on entry and after travel), status line naming the heard channel,
  teacher switch "Bicara ke semua ruang", "Aktifkan suara" when autoplay is blocked, `--color-speaking` ring on
  active speakers (LiveKit active speakers).

## Q1 — Kuis mandiri

- Tabel `quizzes`, `quiz_questions` (tepat 4 opsi, `answer` 0–3, boleh dihapus), `quiz_attempts` (snapshot soal di
  `questions jsonb`, `answers smallint[]` dengan -1 = kosong, `score`/`correct` ditulis sekali saat selesai). Satu
  attempt terbuka per anggota/kuis (unique index parsial); `Start` mengunci kuis lalu melanjutkan attempt terbuka.
- Kunci dan penjelasan tidak dikirim ke peserta sampai attempt selesai; daftar kuis peserta tidak memuat soal.
  Nilai `floor(benar × 100 / jumlah)`; lulus jika ≥ `pass_score` snapshot attempt. Nilai terbaik untuk Rapor.
- Langkah modul `kuis` (`module_steps.quiz_id`, FK org-scoped) hanya menunjuk kuis terbit; lulus menulis
  `module_progress` pada transaksi `Finish` yang sama. Penandaan manual ditolak 409 `quiz_step`.

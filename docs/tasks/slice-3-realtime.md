# Slice 3 — Real-time inti (US-05, US-13 sebagian)

Rujukan: `docs/spec.md` (US-05, US-13, NFR latency/tick/bandwidth/perangkat), `docs/design.md` (Protokol real-time,
Model eksekusi, Reconnect, Peta sekolah, ADR-002/003/007/009), `docs/visual.md` (Peta, Avatar, aksesibilitas),
`docs/screens/02-peta-sekolah.png`. Branch: `slice-3-realtime`.

Tujuan: setelah memilih sekolah, peserta muncul di alun-alun peta sekolah; dua browser saling melihat avatar
bergerak mulus (keyboard WASD/panah dan klik), avatar tidak menembus dinding, dan reconnect singkat tidak terlihat
oleh orang lain. Belum ada chat, zona, travel, jumlah per gedung, avatar pilihan, atau mode HP.

## Keputusan

D3-01…D3-13 disetujui pengguna sebelum kode; kontrak dicatat di `docs/design.md`.
Penegakan kecepatan memakai budget jarak untuk toleransi jitter, dengan slack sebagai utang
(bukan tambahan gratis per pesan). Rincian ada di desain dan hasil implementasi di bawah.

| ID | Pertanyaan | Usulan |
| --- | --- | --- |
| D3-01 | Cara frontend mendapat file peta (pertanyaan terbuka di `design.md`) | Backend menyajikan `GET /api/maps/{map_key}` dari file `.tmj` yang di-embed ke binary (`classverse-backend/maps/`, package kecil `maps` berisi `embed.FS`). Data peta bukan data organisasi, jadi endpoint publik dengan `ETag` + `Cache-Control: no-cache`. Server dan Phaser membaca file yang sama. |
| D3-02 | Gambar tile (tile art final belum ada, `visual.md`) | Tileset dibuat frontend saat runtime: kotak warna flat dari token `--map-*` / `--roof-*` (dibaca lewat `getComputedStyle`), sesuai gaya flat tanpa gradien. Peta `.tmj` hanya menyimpan id tile; urutan id tile = kontrak di `maps/README.md`. Tile art final bisa mengganti tileset ini tanpa mengubah peta. |
| D3-03 | Bentuk peta sekolah | 64×40 tile, tile 32 px. Layer tile `ground`; layer tile `collision` (tile ≠ 0 = tidak bisa dilewati); layer objek `spawn` (persegi alun-alun) dan `buildings` (persegi per gedung dengan properti `key`, `name`, `roof`, `staff_only`, dan objek titik `door`). Isi gedung mengikuti screenshot 02. Gedung dan pintu baru ditampilkan (label + pintu); travel di slice 5. |
| D3-04 | Room id peta sekolah untuk koneksi | `Organization` (REST) mendapat field `campus_room_id`. Client membuka `/ws/rooms/{campus_room_id}`. |
| D3-05 | Auth WebSocket | Upgrade hanya dari Origin `FRONTEND_URL` (mencegah cross-site WebSocket hijacking). Pesan pertama wajib `auth {token}` dalam 5 detik; server memverifikasi JWT dan membership di organisasi pemilik room, lalu mengirim `welcome`. Gagal → `error` lalu close 4401 (`unauthenticated`) / 4403 (`forbidden`) / 4404 (room tidak ada atau bukan anggota). |
| D3-06 | Satu koneksi per user | Koneksi baru milik user yang sama menggantikan yang lama: koneksi lama ditutup dengan code 4409 `replaced`, client lama tidak reconnect otomatis dan menampilkan "Kelasverse dibuka di tab lain". |
| D3-07 | Satuan dan aturan movement | Koordinat dalam pixel dunia (integer, titik kaki avatar). Kecepatan maksimum 160 px/detik (5 tile/detik); server menerima `move` jika jarak sejak posisi terakhir ≤ kecepatan × waktu berlalu × 1,25 + 8 px, tujuan tidak di tile collision, dan garis dari posisi lama ke baru (dicek tiap 8 px) tidak melewati collision. `move` lebih dari 15/detik dibuang tanpa balasan. Move ditolak → `error {code: "move_rejected", seq, x, y}` berisi posisi resmi agar client mengoreksi. `dir`: `up`/`down`/`left`/`right`. |
| D3-08 | Isi pesan | `welcome {self_id, room_id, map_key, participants: [{id, name, avatar, role, x, y, dir}]}`; `joined` = satu participant; `left {id}`; `snapshot {p: [{id, x, y, dir}], out: [id]}` — `out` berisi id yang keluar dari area of interest penerima (client menyembunyikannya). `ping {ts}` → `pong {ts}`. `seq` dari server naik per koneksi. |
| D3-09 | Area of interest di peta sekolah | Sesuai desain: grid sel 8×8 tile; penerima mendapat entri `snapshot` untuk avatar dalam radius 24 tile yang berubah, ditambah avatar yang baru masuk radius (walau diam). `joined`/`left` tetap ke semua orang di room (data kecil; 200 orang). Room selain campus (slice 5) tanpa AOI. |
| D3-10 | Reconnect | Sesuai desain: putus → avatar ditahan 30 detik (tetap terlihat diam). User yang sama kembali ke room yang sama dalam 30 detik → posisi lama, tanpa `left`/`joined` untuk orang lain. Lewat 30 detik → `left`. Client backoff 1, 2, 4, 8 detik (8 detik berulang), status koneksi ditampilkan. |
| D3-11 | Siklus hidup room | Hub (registry room, satu mutex) membuat goroutine room saat orang pertama masuk dan menghentikannya setelah room kosong 60 detik (termasuk tidak ada avatar yang ditahan). Shutdown server menutup semua koneksi dengan code 1001. Sesi/kehadiran (`room_sessions`) tetap di slice 5. |
| D3-12 | Klik untuk berjalan | Client mencari jalur A* di grid collision dan berjalan mengikuti jalur itu dengan aturan kecepatan yang sama; server tetap memvalidasi setiap `move`. Keyboard membatalkan jalur. |
| D3-13 | Rendering avatar | Avatar `default` memakai `round.svg` (preset lain menyusul slice 4). Name tag sesuai `visual.md`: pill putih teks 12 px, diri sendiri pill Tinta, pengajar/admin pill Nila (karena itu `role` ada di data participant). Avatar lain diinterpolasi dengan jeda render 100 ms. |

## Scope

**Backend**
- `maps/`: `campus.tmj`, `README.md` (urutan id tile dan aturan layer), `maps.go` (`//go:embed`).
- `internal/realtime/`: `mapdata.go` (parse `.tmj`: ukuran, collision grid, spawn, gedung), `message.go` (kontrak),
  `gateway.go` (upgrade, auth, goroutine baca/tulis, buffer tulis 64 — penuh = koneksi lambat ditutup),
  `hub.go` (registry room, penggantian koneksi per user), `room.go` (satu goroutine, inbox channel, ticker 100 ms,
  state tanpa mutex), `aoi.go`, `handler.go` (`GET /api/maps/{map_key}`).
- Room membutuhkan info room/membership/profil lewat interface di sisi pemakai (diimplementasikan `org` dan `auth`
  di `cmd/server`), tidak mengimpor store fitur lain.
- `org`: `campus_room_id` di `GET /api/orgs/{id}` (+ OpenAPI). Query sqlc baru: lookup room + org, profil peserta.
- Log: `ws connected`/`ws closed` (user id, room id, alasan), tanpa token. Metrik koneksi aktif per room ditunda
  (NFR observability) — dicatat sebagai hal yang ditunda.

**Frontend**
- `src/realtime/messages.ts` (cermin `message.go`), `client.ts` (koneksi, auth, backoff, `seq`, status),
  `world.ts` (state peserta + interpolasi, murni TS dan dites), `movement.ts` (prediksi + validasi kecepatan/collision
  yang sama dengan server), `pathfind.ts` (A*), `tiles.ts` (tileset dari token).
- `src/game/CampusScene.ts` (Phaser: tilemap, avatar, name tag, kamera mengikuti, input keyboard/klik),
  komponen `GameCanvas.vue` (memasang/membongkar Phaser) di `HomeView` sesuai layout screenshot 02:
  header sekolah + jumlah online, peta di kiri, panel daftar gedung di kanan (tanpa angka sampai slice 5),
  petunjuk "Gerak dengan WASD atau klik".
- Aksesibilitas: canvas punya label; status koneksi dan petunjuk sebagai teks; panel gedung berupa daftar biasa.

## Test

- Go: parse peta (ukuran, collision, spawn, gedung, error file rusak); validasi move table-driven (terlalu cepat,
  menembus dinding tipis, keluar peta, normal); AOI (masuk/keluar radius, `out`); room dengan clock/tick buatan
  (join, snapshot hanya jika berubah, reconnect dalam/luar 30 detik, room berhenti saat kosong); gateway dengan
  `httptest` + klien `coder/websocket` (auth timeout, token salah, bukan anggota, Origin asing, koneksi pengganti,
  buffer penuh). Semua dengan `-race`, tanpa goroutine bocor.
- TS: `client.ts` (auth pertama, backoff, `replaced` tanpa reconnect), `world.ts` (interpolasi, `out`),
  `movement.ts` (cocok dengan aturan server), `pathfind.ts`.
- Browser: dua akun di dua browser bergerak dan saling melihat; menabrak gedung tertahan; reload cepat tidak memicu
  `left` di browser lain.

## Selesai jika

- [x] Keputusan D3-01…D3-13 disetujui dan ditulis ke `docs/design.md`.
- [x] Dua browser di sekolah yang sama melihat avatar satu sama lain bergerak; sekolah lain hanya mendapat peserta sendiri.
- [x] Avatar tidak bisa menembus gedung atau keluar peta; gerak palsu ditolak dalam test server.
- [x] Reload dalam 30 detik tidak terlihat sebagai keluar-masuk di browser lain; tutup tab → hilang setelah 30 detik.
- [x] Membuka tab kedua menggantikan koneksi tab pertama dengan pesan yang jelas.
- [x] `make test` (dengan `-race`, PostgreSQL asli), `make lint` (0 issues), `npm run lint`,
  `npm run test:unit -- --run` (49 test), `npm run build` lulus.
- [ ] Review manusia sebelum slice 4.

## Revisi visual sesudah review

Pengguna menyetujui revisi D3-02: renderer tidak lagi memakai atlas tile kotak untuk tampilan gedung.
`src/game/campusArt.ts` menggambar ilustrasi utuh beresolusi 2× dengan bentuk membulat, dari bounds
Tiled dan CSS tokens. Tiled tetap sumber collision, spawn, dan pintu; protokol/movement tidak berubah.
Kamera overview menampilkan seluruh sekolah dan menyesuaikan resize. Avatar/label mempertahankan
ukuran layar; panel gedung dibatasi tinggi peta dengan scroll. Review visual ulang masih diperlukan.

## Hasil implementasi dan demo

- Backend: map embed + parser, endpoint ETag/304, admission organisasi lewat query sqlc milik realtime,
  hub, room loop 100 ms, collision/lintasan, rate limit gerak, AOI, koneksi pengganti, grace 30 detik, idle 60 detik.
- Frontend: `campus_room_id`, kontrak WS, reconnect 1/2/4/8 detik, offline event, World/interpolasi,
  A*, prediksi gerak pecahan (integer hanya saat kirim), Phaser + token tileset, daftar gedung dan status.
- Browser Chrome via Vite `npm run dev`, dua konteks akun: 12 snapshot selama sekitar 1,1 detik berjalan,
  tanpa `move_rejected`; reload mempertahankan posisi (912,528 menjadi 1079,528 sesudah berjalan),
  tanpa `left`/`joined`. Avatar berhenti di y=482 di depan Ruang Guru (gedung berakhir y=480).
  Klik tujuan menempuh 230 px. Offline tampil reconnect dan pulih tanpa keluar-masuk.
  Tab pengganti menampilkan pesan di tab lama. Tutup tab selama 31,5 detik menghasilkan satu `left`.
  Pindah sekolah baru: welcome satu peserta, tidak membawa avatar sekolah lama.
- Tidak ada exception aplikasi pada demo akhir. Respons 401 refresh sebelum login dan kegagalan WS saat
  offline memang diharapkan; favicon dev masih 404.
- Test mencakup Origin asing/kosong/same-host backend, auth timeout/token salah, nonmember/room hilang,
  archived room, slow consumer, idle room, reconnect, AOI awal avatar diam jauh, dan slack yang tidak boleh
  memberi gerak gratis berulang.
- Penundaan tetap: tile art final, avatar pilihan, chat/zona/travel, jumlah per gedung, mode HP,
  metrik koneksi aktif, pengukuran latency/bandwidth beban 200 peserta. Belum ada klaim NFR beban.
- Build memberi peringatan chunk Phaser sekitar 1,21 MB (326 kB gzip); HomeView sudah lazy-loaded
  sehingga halaman login tidak memuat Phaser. Optimasi bundle lanjutan belum dilakukan.
- Bukti lokal: `/tmp/kv-e2e/s3-map.png`, `/tmp/kv-s3-final3.txt`; skrip demo
  `/tmp/kv-e2e/slice3.mjs` (data akun uji lokal, bukan artefak commit).


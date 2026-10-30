# Slice 5 — Ruang & travel (US-06, US-13)

Status: **draft rencana; belum diimplementasikan**. Branch implementasi: `slice-5-rooms-travel`.
Prasyarat: slice 4 selesai dan direview. Branch dibuat dari hasil tersebut; tidak mengerjakan kedua slice sekaligus.

## Konteks dan rujukan

- `docs/spec.md`: US-06, US-13, Keamanan dan moderasi, NFR kapasitas.
- `docs/design.md`: protokol real-time, model eksekusi/reconnect, peta sekolah, skema `rooms`,
  `room_sessions`, `session_attendances`, API contract dan siklus sesi.
- `docs/visual.md`: alur layar, peta, komponen dan aksesibilitas.
- Screenshot: `docs/screens/02-peta-sekolah.png`, `03-ruang-kelas.png`, `04-kelola-ruang.png`.
- `docs/tasks/phase-4.md`, `docs/tasks/slice-4-profile-avatar.md`.

Baseline yang terlihat saat perencanaan masih `phase-3-setup`, bukan implementasi slice 1–3 yang disebut pengguna.
Nama file baru di bawah adalah kandidat; cocokkan struktur hub, peta, auth, serta kontrak aktual sebelum kode.
Keputusan OpenAPI slice 2 dan distribusi peta slice 3 harus sudah tertulis di `docs/design.md`.

## Hasil dan batas scope

Pengajar/admin membuat ruang kelas atau taman diskusi berkapasitas 40, memilih gedung dan jadwal atau buka langsung.
Peserta berjalan ke pintu, memilih ruang yang diizinkan, lalu berpindah tanpa reload/reconnect.
Keluar ruang mengembalikan peserta ke depan pintu gedung asal. Gedung menampilkan jumlah orang dan kelas live.
Ruang Guru hanya untuk pengajar/admin; isolasi organisasi tetap ditegakkan pada REST dan WebSocket.

Termasuk: daftar/detail, buat, ubah nama/jadwal, arsip ruang, dua peta bawaan, travel,
status gedung, siklus sesi otomatis dan pencatatan kehadiran dasar.
Tidak termasuk: chat/zona aktif (slice 6), broadcast/gather/mute/kick/lock dan UI moderasi (slice 7),
mode HP tanpa movement (slice 8), voice (slice 9), serta UI/endpoint riwayat sesi yang bukan syarat US-06/US-13.
Zona Tiled boleh ada sebagai metadata, tanpa mengklaim fitur diskusi selesai.
`all_chat_enabled` tetap default `true`; fungsi toggle mengikuti slice 6, bukan dibuat seolah chat sudah berfungsi.

## Keputusan yang memblokir implementasi

Semua usulan di tabel **menunggu persetujuan**. Catat hasilnya di `docs/design.md` sebelum kode.

| ID | Celah dokumen | Usulan untuk direview |
| --- | --- | --- |
| R5-01 | Screenshot kelola ruang punya “Masuk ruang” langsung; desain mewajibkan posisi di pintu. | Desktop semua role tetap wajib ke pintu; CTA kelola ruang menuntun kembali ke gedung, bukan bypass validasi. Klik daftar gedung tidak boleh teleport. Konfirmasi perubahan label/alur visual. |
| R5-02 | Arti jadwal dan “Buka lebih awal” belum punya aturan server. | `scheduled_at` kosong berarti dapat dimasuki langsung; jadwal masa depan menolak peserta sampai waktunya. Pengajar/admin dapat membuka lebih awal dengan PATCH menghapus jadwal. Sesi tetap baru dimulai ketika orang pertama masuk, bukan saat waktu jadwal tercapai. |
| R5-03 | Validasi pintu, spawn, akses Ruang Guru, dan pintu keluar belum punya format Tiled rinci. | Tetapkan schema objek untuk key gedung, area pintu/keluar, spawn aman, serta role yang boleh masuk; semua dibaca server dan renderer dari sumber peta slice 3. Hitung kedekatan dari posisi server, bukan koordinat kiriman travel. |
| R5-04 | Travel gagal setelah peserta dilepas dari room asal. | Reservasi kapasitas tujuan dan otorisasi sebelum melepas asal; koordinasikan transfer lewat hub/inbox dan ack, dengan rollback terdefinisi. Satu user hanya punya satu owner room; tidak berbagi map state antar goroutine. |
| R5-05 | Arti live/jumlah orang, masa reconnect 30 detik, dan timer kosong 5 menit. | Definisikan sumber angka dan apakah slot/kehadiran reconnect masih dihitung. Bedakan sesi belum berakhir dari jumlah koneksi aktif; jangan menyamakan `scheduled_at` dengan live. |
| R5-06 | Arsip ruang yang sedang terisi; perubahan gedung/map dan perlindungan campus. | Tolak arsip ruang yang masih terisi atau punya reservasi reconnect; arsip ruang kosong secara soft delete. Larang CRUD umum mengarsip/mengubah map campus; map dan gedung ruang tetap setelah dibuat. Tetapkan perilaku sesi kosong yang masih terbuka. |
| R5-07 | Reconnect saat/sesudah travel; cold join langsung ke room tujuan. | Simpan room/posisi/gedung asal yang sudah di-commit di hub; reconnect memulihkan lokasi itu selama grace period. Jangan biarkan URL WS menjadi bypass pintu/jadwal/role. Definisikan fallback aman ke campus saat state sudah hilang. |
| R5-08 | Payload rinci travel/buildings, error code, dan validasi nama ruang belum lengkap. | Tetapkan request/response REST, field state live, batas nama, cursor/filter gedung, payload `traveled` dan `buildings`, error terjadwal/penuh/tidak di pintu/terarsip, serta jaminan `seq`. Pakai envelope dan format error yang sudah ada. |

Kebijakan jadwal harus diuji dengan clock injeksi dan waktu UTC; UI menampilkan waktu lokal dengan zona jelas.
Kebijakan archive/jadwal serta metadata role gedung tidak boleh ditebak dari nama tampilan gedung.

## Peta kontrak dan file

### REST

| Method | Path | Scope slice |
| --- | --- | --- |
| GET | `/api/orgs/{org_id}/rooms` | Daftar non-arsip, cursor; filter gedung jika disetujui R5-08; status sesuai izin. |
| POST | `/api/orgs/{org_id}/rooms` | Pengajar/admin: nama, map `classroom`/`garden`, gedung sah, jadwal opsional; kapasitas ditentukan server 40. |
| GET | `/api/rooms/{room_id}` | Detail dan status live/jumlah peserta, setelah membership dan akses gedung diperiksa. |
| PATCH | `/api/rooms/{room_id}` | Pengajar/admin: nama/jadwal sesuai kontrak; tidak mengubah map/gedung/campus. |
| DELETE | `/api/rooms/{room_id}` | Pengajar/admin: arsip sesuai R5-06. |

Tidak ada endpoint start/stop sesi baru. Sesi dimulai otomatis saat peserta pertama masuk dan berakhir
setelah kosong 5 menit, sesuai desain; detail interaksi grace period mengikuti R5-05.

### WebSocket

Pakai satu koneksi sesi yang sudah ada: client `travel` berisi `room_id`; server `traveled` berisi state
setara welcome tujuan, `joined`/`left` untuk observer, `buildings` tiap 5 detik hanya untuk penghuni campus.
Keluar ruang memakai `travel` ke room campus organisasi, bukan endpoint/koneksi kedua.
Pertahankan `snapshot` 100 ms, buffer outbound 64, dan reconnect hasil slice 3.
Kontrak ditulis di `internal/realtime/message.go` dan dicerminkan di `src/realtime/messages.ts` dalam slice ini.

### File target awal

- Backend: `internal/room/{handler,service,store}.go` dan test; query `db/queries/rooms.sql`,
  `room_sessions.sql`, `session_attendances.sql`; perluas hub/room loop pada `internal/realtime/`;
  wiring constructor/router di `cmd/server/` tanpa logika bisnis.
- Dependensi: package room memeriksa membership lewat interface sisi pemakai; tidak mengimpor store fitur org.
  Live state diakses lewat batas query/inbox yang jelas, bukan membaca map state room langsung dari HTTP handler.
- Peta: `classverse-backend/maps/classroom.tmj`, `garden.tmj`, metadata gedung/pintu pada `campus.tmj`,
  beserta tileset/aset melalui mekanisme distribusi slice 3.
- Frontend: `src/api/rooms.ts`, state room/realtime yang sudah ada, view kelola ruang/form,
  panel pemilih ruang di pintu, scene kelas/taman, label/badge gedung, tombol kembali campus.
- OpenAPI: sumber hasil slice 2 dan generated model Go/TypeScript sesuai pipeline yang sudah ada.
- Database: tabel dasar sudah didefinisikan di desain. Jangan membuat ulang tabel atau mengubah
  `00001_init.sql`; migration baru hanya bila constraint/index tambahan dibutuhkan dan disetujui.
  Semua query data organisasi memfilter `org_id`; query via room/session harus mengikat organisasi melalui join/parameter.

## Urutan tugas

### 5.0 — Audit baseline dan finalisasi keputusan

- [ ] Pastikan slice 4 sudah direview; petakan kontrak, loader Tiled, hub, koneksi dan reconnect aktual.
- [ ] Selesaikan R5-01 sampai R5-08; tulis contoh payload dan state transisi, update desain dan OpenAPI.
- [ ] Pecah langkah backend/frontend berikut menjadi subtask kecil bila menyentuh lebih dari sekitar lima file.

Verifikasi: review matriks izin (participant/teacher/admin/non-member), diagram transfer berhasil/gagal,
status jadwal/sesi, dan path reconnect. Dependensi: slice 4. Ukuran: kecil per dokumen/kontrak.

### 5.1 — Buat dan lihat ruang lewat API

- [ ] Query/store untuk daftar, detail, create dengan filter organisasi, pagination, serta validasi key map/gedung.
- [ ] Service dan handler memeriksa membership/role dari server; kapasitas 40 tidak dapat dinaikkan oleh input.
- [ ] Test handler/service dan integration PostgreSQL: dua organisasi, role berbeda, gedung palsu,
  map campus untuk create ditolak, pagination dan nilai jadwal UTC.

File: query/store room, service/handler room, wiring dan test (pecah store, service, handler bila perlu).
Verifikasi: focused Go tests dan integration PostgreSQL asli. Dependensi: 5.0. Ukuran: sedang per subtask.

### 5.2 — UI kelola ruang dan jadwal

- [ ] Kartu daftar ruang dan form buat: nama, gedung, dua preset peta, buka langsung/jadwal; state kosong/loading/error.
- [ ] Integrasikan API; peserta tidak mendapat kontrol pengajar, tetapi server tetap menolak request palsu.
- [ ] Form tampil Bahasa Indonesia, memakai token, label, fokus keyboard dan target sentuh 44×44 px.

File: client rooms, view/form/kartu, router dan unit test.
Verifikasi: Vitest input/submission/error/role; browser pengajar membuat ruang lalu peserta melihat daftar.
Ukur alur pembuatan hingga siap dalam satu menit sesuai US-06. Dependensi: 5.1. Ukuran: sedang per komponen/integrasi.

### Checkpoint — Pembuatan ruang

- [ ] Kontrak REST sinkron, test PostgreSQL lulus, demo buat ruang langsung/terjadwal berhasil.
- [ ] Review jadwal dan izin sebelum menghubungkan travel.

### 5.3 — Dua peta dan metadata pintu

- [ ] Siapkan `classroom` dan `garden` dengan collision, spawn aman, pintu keluar, metadata zona untuk slice 6.
- [ ] Validasi metadata campus dan building key yang dipakai ruang; Ruang Guru punya aturan akses server.
- [ ] Renderer memuat kedua peta melalui distribusi slice 3; tidak menyalin source peta lintas project.

File: `.tmj`/tileset/aset, parser peta dan scene/loader.
Verifikasi: test parser untuk key hilang/duplikat, spawn/collision tidak valid; browser bergerak tanpa menembus objek.
Dependensi: 5.0, selesai sebelum 5.4. Ukuran: sedang per peta/parser.

### 5.4 — Travel atomik di server

- [ ] Validasi membership organisasi, status arsip/jadwal, akses gedung, posisi di pintu, dan kapasitas tujuan.
- [ ] Transfer terkoordinasi hub/inbox sesuai R5-04; tujuan penuh/gagal tidak menghilangkan peserta dari asal.
- [ ] Implementasikan keluar ke spawn aman depan pintu asal, cold join/reconnect sesuai R5-07,
  serta penanganan permintaan travel ganda/in-flight tanpa presence ganda.

File: hub, room loop/commands, tipe pesan dan test (pisahkan admission dan transfer/reconnect).
Verifikasi: test race dua user memperebutkan slot ke-40, travel lintas org, pintu salah/jauh,
Ruang Guru, tujuan hilang, cancel/disconnect, rollback dan reconnect setelah commit.
Dependensi: 5.1, 5.3. Ukuran: sedang per subtask; risiko tertinggi slice ini.

### 5.5 — UI pintu, scene tujuan, dan kembali

- [ ] Saat di pintu tampil daftar ruang gedung; pilihan menjadwalkan request travel, bukan mengubah posisi lokal.
- [ ] Setelah `traveled`, ganti scene/state dari server tanpa reload atau koneksi WS baru; bersihkan entity/listener scene lama.
- [ ] Tombol keluar mengirim travel ke campus; tujuan penuh/terjadwal/ditolak tetap di scene asal dengan pesan jelas.

File: panel ruang, state realtime, scene switcher, tombol kembali, unit test.
Verifikasi: dua browser bertemu, salah satu travel, observer asal/tujuan benar; kembali di pintu asal;
DevTools membuktikan koneksi WS tetap sama. Dependensi: 5.4. Ukuran: sedang per UI/scene.

### Checkpoint — Travel dua arah

- [ ] `go test -race ./...` lulus untuk transfer/reconnect; tidak ada goroutine/presence bocor.
- [ ] Demo campus → classroom/garden → campus berjalan; peserta tidak dapat bypass Ruang Guru atau pintu.
- [ ] Review sebelum menambah lifecycle/status yang bergantung transfer.

### 5.6 — Sesi otomatis dan kehadiran

- [ ] Mulai sesi pada kedatangan pertama yang berhasil; dua join serentak tidak membuat dua sesi aktif.
- [ ] Catat kehadiran masuk/keluar sesuai transfer dan reconnect yang disetujui; tutup sesi kosong setelah 5 menit.
- [ ] Gunakan clock/timer yang dapat diuji; tetapkan penanganan gagal tulis DB tanpa memblokir room loop;
  periksa kebutuhan constraint baru lewat migration baru setelah disetujui.

File: query/store sesi/kehadiran, koordinasi lifecycle, clock dan test.
Verifikasi: integration PostgreSQL, test timer tanpa sleep panjang, join ulang sebelum timeout,
rollback travel tidak mencatat kehadiran palsu, shutdown tidak membocorkan worker.
Dependensi: 5.4–5.5. Ukuran: sedang per persistence/koordinasi.

### 5.7 — Status gedung, ubah dan arsip ruang

- [ ] Agregat jumlah orang/kelas live menurut R5-05, kirim `buildings` tiap 5 detik hanya ke campus;
  filter akses agar data ruang terlarang tidak bocor melalui detail/count/list.
- [ ] Tampilkan badge/label dan status kartu; bedakan live dari terjadwal dan sesi kosong.
- [ ] Tambahkan PATCH nama/jadwal (termasuk buka awal jika disetujui) dan DELETE soft archive sesuai R5-06,
  dengan test race terhadap join/travel; ruang arsip tidak bisa dipakai lewat ID lama.

File: agregator hub, message mirror, badge/kartu frontend; service/store/handler dan test untuk update/archive.
Pisahkan status gedung dan update/archive menjadi subtask. Verifikasi: fake clock interval 5 detik,
API snapshot konsisten, akses non-member ditolak, archive vs admission terserialisasi.
Dependensi: 5.6. Ukuran: sedang per subtask.

### 5.8 — Regresi, demo, review

- [ ] Seluruh pemeriksaan wajib lulus; catat perintah dan hasil aktual.
- [ ] Demo pengajar: buat ruang langsung/terjadwal, buka lebih awal sesuai keputusan, ubah nama/jadwal, arsip ruang kosong.
- [ ] Demo dua peserta: ke pintu, pilih kelas/taman, bergerak, kembali; status gedung berubah dalam interval kontrak.
- [ ] Demo negatif: user lintas organisasi, peserta ke Ruang Guru, ruang penuh, jauh dari pintu,
  ruang terjadwal/arsip, reconnect saat travel; tidak ada perpindahan atau kebocoran data yang tidak sah.
- [ ] Catat hasil review manusia; commit menyebut `US-06`/`US-13`. Jangan mulai slice 6 sebelum review selesai.

## Pemeriksaan wajib saat implementasi

```sh
cd classverse-backend
make test && make lint
go test -race ./...

cd ../classverse-frontend
npm run lint && npm run test:unit -- --run && npm run build
npm run dev
```

Integration test memakai PostgreSQL asli. Demo browser memakai backend berjalan, dua organisasi,
akun participant/teacher/admin, serta pengamatan WebSocket lewat DevTools.
Laporan akhir: ringkasan perubahan, hasil perintah, keputusan yang disetujui, penundaan eksplisit.
Rencana ini belum menjalankan test aplikasi dan tidak menyatakan checklist implementasi selesai.

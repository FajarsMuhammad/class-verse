# Slice 6 — Chat & zona (US-07, US-08)

Status: **selesai dan direview pengguna; disetujui untuk commit**. Branch: `slice-6-chat-zones` (dari `0c8c3b1`).

Pengguna mengonfirmasi fitur bekerja. Revisi layout disetujui sebagai pekerjaan terpisah setelah commit slice 6:
chat tetap di kanan, peta diperluas, daftar gedung dibuka saat diperlukan, dan pilihan ruang kontekstual dekat pintu.
Aturan travel dan validasi pintu tetap; revisi layout belum termasuk implementasi slice ini.

## Hasil

- C6-01…C6-10 disetujui dan ditulis di `docs/design.md` (bagian *Slice 6*).
- Backend: package `moderation` (filter chat, daftar kata dipakai bersama nama tampilan), parser layer `zones`,
  routing chat near/zona/semua di room loop, rate limit token bucket, pesan `zone` dan `room_state`,
  penyimpanan lewat worker hub (pengiriman tidak memblokir room atau worker), purge 30 hari di `cmd/server`,
  PATCH `all_chat_enabled`.
- Frontend: `ChatState` (store murni, maks 200 per tab), `ChatPanel` (tab, log `aria-live`, Enter/Shift+Enter,
  sisa karakter), chat di bawah peta sekolah dan di sidebar ruang, toggle pengajar, zona + radius di Phaser.
  Token baru `--radius-xs` (6 px, sudut bubble dari `visual.md`).

## Bukti

- `make test` (`-race`, PostgreSQL asli) dan `make lint` (0 issues). Test baru: filter tabel + fuzz 735 ribu input
  (idempoten, tidak menciptakan nomor HP), penerima near tepat di 160 px vs 161 px, avatar grace tidak menerima,
  zona lintas jarak dan keluar zona, `not_in_zone`, toggle Semua + `room_state`, Semua selalu aktif di campus,
  rate limit burst 5 lalu refill 3/detik, scope tak dikenal, welcome membawa `all_chat_enabled` dan zona saat reconnect,
  chat lewat gateway sungguhan dengan filter, simpan/tolak tanpa sesi/purge 30 hari dengan laporan tetap ada,
  purge berkala tetap jalan saat DB gagal, PATCH toggle (izin, nilai salah, persisten, hub diberi tahu).
- Frontend: lint, 72 unit test, build lulus.
- Chrome 3 akun (`/tmp/kv-e2e/slice6.mjs`), tanpa error runtime: chat dekat di campus; Semua diterima pengajar di ruang
  yang sama tapi tidak oleh siswa di campus; filter `wa **** [disembunyikan]`; dua akun di sudut berbeda Meja 1 saling
  terima, setelah pengajar pindah ke Meja 2 tidak lagi; tab zona otomatis; pengajar mematikan Semua dan input siswa
  langsung nonaktif; 7 pesan beruntun → 5 terkirim, 2 `rate_limited`; reload tidak memutar ulang chat; 60 fps.
  Screenshot: `/tmp/kv-e2e/s6-zone.png`, `s6-campus.png`.
- DB: hanya teks hasil filter tersimpan, `zone_key` terisi untuk chat zona, chat campus tidak tersimpan.
  Log server tidak memuat isi chat.

## Ditunda / keterbatasan

- Alamat rumah belum disaring otomatis (C6-06).
- Riwayat chat tidak ditampilkan saat masuk/reconnect (C6-04).


## Rujukan

- `docs/spec.md`: US-07, US-08, US-13 (chat juga di peta sekolah), Keamanan 2/4/5, NFR bandwidth dan rate limit chat.
- `docs/design.md`: tabel pesan WS (`chat`, `zone`), *Proximity*, skema `chat_messages`, `rooms.all_chat_enabled`,
  catatan purge 30 hari, PATCH `/api/rooms/{room_id}` (chat semua ruangan).
- `docs/visual.md`: chip zona, bubble chat, input chat, zona putus-putus Mangga / solid saat di dalam, radius Nila.
- Screenshot: `03-ruang-kelas.png` (tab Meja 3 / Dekat / Semua, zona dan radius).

## Hasil

Di peta sekolah, kelas, dan taman: peserta mengetik pesan ke orang **dekat** (radius 5 tile). Di kelas/taman,
masuk persegi zona (meja/bangku) otomatis bergabung ke chat **zona**; semua anggota zona menerima pesan apa pun
jaraknya. Tab **Semua** mengirim ke seluruh ruang, bisa dimatikan pengajar. Masuk/keluar zona terlihat di peta dan panel.
Isi chat disaring kata kasar dan pola data pribadi, disimpan per sesi, dihapus otomatis setelah 30 hari.

Tidak termasuk: broadcast pengajar, gather, mute/kick/lock, report dan UI moderasi (slice 7), mode HP tanpa movement
(slice 8), voice (slice 9), riwayat chat lintas sesi, reaksi/emote, angkat tangan.

## Keputusan yang disetujui

| ID | Celah | Usulan |
| --- | --- | --- |
| C6-01 | Penerima `near` | Jarak Euclid ≤ 5 × 32 = 160 px antar titik kaki, dihitung server saat pesan diterima. Pengirim selalu menerima salinannya. Berlaku juga di peta sekolah (US-13). Avatar dalam grace reconnect tidak menerima. |
| C6-02 | Zona | Layer objek `zones` pada classroom/garden (sudah ada: persegi + properti `key`, nama objek = label). Zona aktif = persegi yang memuat titik kaki; peta sekolah tidak punya zona. Dihitung server tiap tick; perubahan dikirim pesan `zone {key, name}` ke pemiliknya (kosong = keluar). Pesan `zone` lain tidak disiarkan. |
| C6-03 | Tab dan scope | Tab **Zona** (nama zona, tampil hanya saat di zona) / **Dekat** / **Semua**. Tab default: Zona jika di zona, selain itu Dekat. Pesan `zone` ditolak (`not_in_zone`) jika pengirim tidak di zona saat diterima server. Scope `broadcast` tetap untuk slice 7. |
| C6-04 | Payload | Client → `chat {text, scope}`. Server → `chat {id, from:{id,name,role}, text, scope, zone?, at}`; `id` dari DB bila tersimpan, kalau penyimpanan gagal tetap dikirim tanpa `id`. Riwayat tidak dikirim saat masuk/reconnect (hanya pesan baru selama terhubung). |
| C6-05 | Batas | 1–500 code point setelah NFC + trim; karakter kontrol ditolak kecuali baris baru (dipadatkan maks 2). Maks 3 pesan/detik per user dengan burst 5 (token bucket); kelebihan → `error rate_limited`. |
| C6-06 | Filter (Keamanan 4) | Disamarkan, bukan ditolak: kata kasar (daftar proyek yang sama dengan nama tampilan, cocok kata utuh) → `****`; nomor HP Indonesia (`08…`, `+62…`, `62…`, 9–13 digit dengan spasi/titik/strip) dan email → `[disembunyikan]`. Alamat tidak disaring otomatis di slice ini (terlalu banyak salah deteksi); dicatat sebagai keterbatasan. Yang disimpan dan disiarkan hanya teks hasil filter. |
| C6-07 | Penyimpanan | Setiap pesan non-campus disimpan ke `chat_messages` (session aktif room) lewat worker hub yang sudah ada, tidak memblokir room loop. Peta sekolah tidak punya sesi → chat campus tidak disimpan dan langsung hilang. Purge harian `created_at < now() - 30 hari` dari goroutine di `cmd/server` (ticker 24 jam + sekali saat start). Log tanpa isi chat. |
| C6-08 | Chat semua ruangan | PATCH `/api/rooms/{room_id}` menerima `all_chat_enabled` (teacher/admin). Room diberi tahu lewat hub; perubahan disiarkan `room_state {all_chat_enabled}` ke semua di ruang. Saat mati, scope `room` ditolak `all_chat_disabled`; tab Semua tetap tampil tapi input nonaktif dengan keterangan. Peta sekolah: Semua selalu aktif dan tidak bisa diubah. `welcome`/`traveled` membawa `all_chat_enabled`. |
| C6-09 | Tampilan peta | Zona digambar dari Tiled: border putus-putus `--color-accent-line` + isi `--color-accent-soft` tipis; zona sendiri solid lebih tebal + label "· kamu di sini". Lingkaran radius dekat putus-putus Nila tipis di sekitar avatar sendiri. Panel kanan berisi tab dan chat (mengganti teks "Chat dan zona hadir pada slice berikutnya"). |
| C6-10 | Aksesibilitas chat | Daftar pesan `role="log"` `aria-live="polite"`, input berlabel tersembunyi, Enter kirim / Shift+Enter baris baru, tombol kirim ikon 44×44 dengan `aria-label="Kirim pesan"`. Maks 200 pesan per tab di memori. Fokus di input tidak menggerakkan avatar (keyboard peta hanya saat canvas fokus, sudah dari slice 3). |

## Kontrak

**WS (`message.go` ↔ `messages.ts`)**: tambah `chat` (dua arah), `zone`, `room_state`; error `rate_limited`,
`not_in_zone`, `all_chat_disabled`, `bad_message` untuk teks tidak valid; `welcome`/`traveled` + `all_chat_enabled`.

**REST**: `UpdateRoomRequest` + `all_chat_enabled`; `Room` + `all_chat_enabled`. OpenAPI dan model Go/TS diregenerasi.

**DB**: tabel `chat_messages` dan kolom `all_chat_enabled` sudah ada di `00001`. Migration baru hanya jika butuh index
(`chat_messages (session_id, id)` untuk laporan slice 7 belum perlu sekarang). Query sqlc baru: `InsertChatMessage`,
`PurgeChatMessages`, kolom `all_chat_enabled` di admission dan update room.

## Urutan tugas

### 6.0 Keputusan dan kontrak
- [x] Setujui C6-01…C6-10; tulis ke `docs/design.md`; perbarui OpenAPI, `message.go`, `messages.ts`.

### 6.1 Filter dan validasi teks (backend murni)
- [x] Package `moderation` (sesuai struktur di design): `FilterChat(text) (string, error)` — normalisasi, batas, kontrol,
  kata kasar, nomor HP, email. Daftar kata dipindah dari `auth` ke sini; `auth` memakai interface sisi pemakai atau
  fungsi yang sama tanpa saling impor store.
- [x] Table-driven test + fuzz kecil (tidak panic, hasil tidak pernah memuat digit HP utuh).

### 6.2 Zona dan chat di room loop
- [x] Parser peta: layer `zones` (key unik, persegi di dalam peta).
- [x] Room: zona per member dihitung tiap tick, kirim `zone` saat berubah; `chat` di-route per scope;
  rate limit per member; `all_chat_enabled` di state room.
- [x] Test unit: penerima near di batas 160 px, zona lintas jarak, keluar zona, rate limit, room chat mati,
  campus tanpa zona, member grace tidak menerima.

### 6.3 Persistensi dan purge
- [x] Worker hub menulis `chat_messages` memakai session terbuka; gagal tulis tidak memblokir.
- [x] Purge 30 hari di `cmd/server` dengan clock injeksi; test PostgreSQL (pesan lama terhapus, laporan tidak ikut).

### 6.4 Toggle chat semua ruangan
- [x] PATCH room `all_chat_enabled` + notifikasi hub → `room_state`; test handler/service dan WS.

### 6.5 Frontend chat
- [x] Store chat per tab (maks 200), komponen `ChatPanel` (tab, log, input, error, rate limit), bubble sesuai visual.
- [x] Toggle "Chat semua ruangan" untuk pengajar di panel ruang.
- [x] Unit test store dan komponen.

### 6.6 Zona dan radius di Phaser
- [x] Gambar zona dari Tiled, highlight zona sendiri, lingkaran radius dekat; tidak mengubah collision.

### 6.7 Verifikasi
- [x] `make test && make lint`, frontend lint/test/build.
- [x] Chrome 3 akun: dekat vs jauh di campus; dua orang di meja sama (jauh) saling terima, meja lain tidak;
  Semua diterima semua lalu dimatikan pengajar; filter HP/kata kasar; rate limit; reconnect tidak mengulang pesan.
- [x] Laporan dan review pengguna sebelum commit.

## Pemeriksaan wajib

```sh
cd classverse-backend && make test && make lint
cd ../classverse-frontend && npm run lint && npm run test:unit -- --run && npm run build
```

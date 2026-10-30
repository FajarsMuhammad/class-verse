# Fase 4 — Implementasi fitur (vertical slice)

Setiap slice mengerjakan backend, frontend, dan test untuk satu kelompok fitur sampai bisa dicoba di browser.
Satu slice = satu branch (`slice-N-<nama>`) = satu file task (`docs/tasks/slice-N-*.md`). Jangan mulai slice berikutnya
sebelum checklist slice sebelumnya selesai dan sudah direview.

Kerangka repo dari Fase 3 sudah ada (`docs/tasks/phase-3.md`): monorepo dengan `classverse-backend/` (Go) dan
`classverse-frontend/` (Vue + Phaser), lihat ADR-010 dan bagian *Struktur repo* di `docs/design.md`.

| # | Slice | Stories | Hasil yang bisa dicoba | Prasyarat keputusan |
| --- | --- | --- | --- | --- |
| 1 | Auth | US-01 | Daftar email + OTP, login, login Google, refresh token, lengkapi profil (tanggal lahir) | — |
| 2 | Organisasi & undangan | US-02, US-03 | Buat sekolah, undang lewat kode, gabung; peta sekolah otomatis dibuat | Lokasi `openapi.yaml` |
| 3 | Real-time inti | US-05, US-13 (sebagian) | Dua browser saling melihat avatar bergerak di peta sekolah | Cara frontend mendapat file peta Tiled |
| 4 | Profil & avatar | US-04 | Pilih avatar dan nama tampilan (3–24 karakter, filter kata kasar) | — |
| 5 | Ruang & travel | US-06, US-13 | Buat ruang di gedung, masuk lewat pintu, kembali ke peta | — |
| 6 | Chat & zona | US-07, US-08 | Proximity chat, meja diskusi, chat semua ruangan | — |
| 7 | Kontrol pengajar & moderasi | US-10, US-11, US-12 | Broadcast, kumpulkan, mute/kick/kunci, laporan | — |
| 8 | Mode HP | — | Daftar gedung dan zona, chat tanpa movement | — |
| 9 | Voice (M2) | US-09 | Proximity voice lewat LiveKit | — |

Slice 1–8 = milestone M1 (siap pilot). Slice 9 = M2.

Prasyarat keputusan dicatat sebagai pertanyaan terbuka di `docs/design.md` (bagian *Berikutnya*) beserta usulannya;
putuskan dan tulis ke `design.md` sebelum slice itu dimulai.

Batas slice 1 dan 4: slice 1 hanya meminta data yang wajib sebelum bergabung ke organisasi (tanggal lahir; nama
tampilan diisi dari registrasi atau akun Google). Validasi nama tampilan, filter kata kasar, dan pemilihan avatar
dikerjakan di slice 4; sampai saat itu semua user memakai `avatar_key = 'default'`.

## Aturan untuk setiap slice

- Baca `CLAUDE.md`, bagian `docs/spec.md` dan `docs/design.md` yang dirujuk task, dan `docs/visual.md` + screenshot terkait untuk UI.
- Mulai dengan rencana singkat (file yang akan dibuat/diubah di backend dan frontend, migration/query baru, endpoint, pesan WebSocket) sebelum menulis kode.
- Keputusan yang tidak tertulis di dokumen: usulkan dulu, jangan langsung diterapkan.
- Backend dan frontend tidak saling mengimpor file. Perubahan kontrak (endpoint REST, tipe pesan di
  `classverse-backend/internal/realtime/message.go` dan cerminnya di `classverse-frontend/src/realtime/messages.ts`)
  dikerjakan di kedua sisi dalam slice yang sama.
- Skema database berubah hanya lewat migration baru (`make migrate-new name=...`); `00001_init.sql` tidak diedit lagi.
- Selesai berarti: semua checklist di task tercentang, pemeriksaan berikut lulus, dan alur bisa didemokan manual di browser
  lewat `npm run dev` (bukan hanya lewat curl).

  ```
  # classverse-backend/
  make test && make lint

  # classverse-frontend/
  npm run lint && npm run test:unit -- --run && npm run build
  ```

- Akhiri dengan laporan: ringkasan perubahan, perintah yang dijalankan beserta hasilnya, keputusan yang diambil sendiri,
  dan hal yang sengaja ditunda.

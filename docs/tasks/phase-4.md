# Fase 4 — Implementasi fitur (vertical slice)

Setiap slice mengerjakan backend, frontend, dan test untuk satu kelompok fitur sampai bisa dicoba di browser.
Satu slice = satu branch = satu file task (`docs/tasks/slice-N-*.md`). Jangan mulai slice berikutnya sebelum
checklist slice sebelumnya selesai dan sudah direview.

| # | Slice | Stories | Hasil yang bisa dicoba |
| --- | --- | --- | --- |
| 1 | Auth | US-01 | Daftar email + OTP, login, login Google, refresh token, lengkapi profil |
| 2 | Organisasi & undangan | US-02, US-03 | Buat sekolah, undang lewat kode, gabung; peta sekolah otomatis dibuat |
| 3 | Real-time inti | US-05, US-13 (sebagian) | Dua browser saling melihat avatar bergerak di peta sekolah |
| 4 | Profil & avatar | US-04 | Pilih avatar dan nama tampilan |
| 5 | Ruang & travel | US-06, US-13 | Buat ruang di gedung, masuk lewat pintu, kembali ke peta |
| 6 | Chat & zona | US-07, US-08 | Proximity chat, meja diskusi, chat semua ruangan |
| 7 | Kontrol pengajar & moderasi | US-10, US-11, US-12 | Broadcast, kumpulkan, mute/kick/kunci, laporan |
| 8 | Mode HP | — | Daftar gedung dan zona, chat tanpa movement |
| 9 | Voice (M2) | US-09 | Proximity voice lewat LiveKit |

Slice 1–8 = milestone M1 (siap pilot). Slice 9 = M2.

## Aturan untuk setiap slice

- Baca `CLAUDE.md`, bagian `docs/spec.md` dan `docs/design.md` yang dirujuk task, dan `docs/visual.md` untuk UI.
- Mulai dengan rencana singkat (file yang akan dibuat/diubah, query baru, endpoint) sebelum menulis kode.
- Keputusan yang tidak tertulis di dokumen: usulkan dulu, jangan langsung diterapkan.
- Selesai berarti: semua checklist di task tercentang, `make test` dan `make lint` lulus, dan alur bisa didemokan manual.
- Akhiri dengan laporan: ringkasan perubahan, perintah yang dijalankan beserta hasilnya, keputusan yang diambil sendiri,
  dan hal yang sengaja ditunda.

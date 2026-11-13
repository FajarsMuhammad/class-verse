# D1 — Menara Sains, Ruang Guru, dan jalur modul

Referensi pengguna: `kelasverse-assets/reference/m2/Kelasverse Revamp.html` (export bundler delapan halaman),
PNG interior Sains/Guru, jalur modul, pemutar modul. Branch `design-science-staff`, basis M2 `85d761c`.

## Batas desain

- Pakai komponen, token, avatar, chat/voice, reader dan progres existing; bukan menjalankan prototype HTML.
- Modul tetap katalog bebas, tanpa gembok urutan pada contoh; akses dari luar ruang tetap tersedia.
- Tidak menyalin nama penghuni, nilai, statistik, jadwal, skor, XP, atau modul contoh sebagai data nyata.
- Interior `science` dan `staff` ditambahkan sebagai pilihan baru; tidak mengganti peta/ID ruang lama.
  Pilih gedung Sains/Guru saat membuat ruang baru untuk default interior baru. `staff` hanya boleh dibuat
  di `ruang-guru`; otorisasi masuk tetap berdasarkan gedung khusus staf.

## Hasil

- [x] Layout Menara Sains: planetarium, teleskop, tiga meja lab, empat stasiun eksperimen, tiga pod VR;
  31 kursi. Ruang Guru: enam meja kerja, meja rapat, studio, periksa tugas, pantry; 18 kursi.
- [x] Geometri dari referensi disalin ke runtime `src/assets/world/rooms.json`; field penghuni contoh dibuang,
  label meja memakai nomor. Generator menghasilkan peta server, collision, kursi dan pintu; peta lama tetap identik.
- [x] Grid lantai Sains/checker Guru, stasiun dengan label, klik stasiun dan highlight pilihan. Tombol daftar
  stasiun menyediakan akses keyboard dan HP. Katalog IPA memakai data M1/M2 nyata; belum ada relasi modul-stasiun,
  jadi semua stasiun membuka katalog gedung yang sama dan batas ini tertulis di UI.
- [x] Panel Guru menampilkan daftar modul sekolah (draf/terbit) nyata dan link ke Kelola. Tidak memakai label
  "Modul saya" karena endpoint belum menyaring pembuat. Rekaman, pemeriksaan tugas, dan simulasi hanya scenery,
  bukan tombol fungsional palsu. Suara mengikuti aturan proximity existing, tanpa rekaman/notulen otomatis.
- [x] Jalur modul: header teal, latar mint bertitik, kartu krem beroutline tinta, node bernomor/centang dan
  jalur titik; responsif. Pemutar materi/video: daftar isi, segmen progres, Sebelumnya/Selesai/Lanjut;
  navigasi tidak otomatis menandai selesai. PDF/audio tetap memakai FileReader existing.
- [x] Browser Chrome: membuat/memasuki kedua interior, 31/18 kursi, peserta ditolak dari Ruang Guru,
  klik peta memilih stasiun, link Kelola, stasiun di HP 390 px tanpa overflow, modul video/teks/PDF dan progres.
- [x] Verifikasi: 43 file/160 test frontend, lint, type-check/build, world BFS/staleness check;
  seluruh test backend `-race` dengan Postgres + AIStor dan lint lulus. Chrome interior desktop/390 px,
  modul teks/video/PDF, progres dan regresi perpustakaan lulus. Fixture browser menangani reconnect ruang
  serta masuk eksplisit di mode HP (login baru tidak menjamin resume ruang sebelumnya).
- [ ] Tidak termasuk implementasi kuis/game/VR/AI/catatan/rapor/jadwal, redesign Arena/perpustakaan/kampus
  menyeluruh, Safari/iOS, migrasi otomatis ruang lama. Fitur belajar berikutnya tetap Q1.

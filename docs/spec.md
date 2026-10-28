# Kelasverse — Spec MVP (Fase 1)

Oct 28, 2026 · @Fajar

## Ringkasan produk

Kelasverse adalah ruang belajar virtual 2D tempat siswa dan mahasiswa hadir sebagai avatar, berjalan di dalam kelas, dan berdiskusi dengan orang di dekatnya lewat text dan voice. Pilot pertama menyasar **SMA dan kampus** di Indonesia.

**Masalah.** Kelas online lewat video call terasa pasif: peserta hanya menatap grid wajah, kamera mati, dan diskusi kelompok terasa kaku. Akibatnya fokus dan partisipasi turun.

**Solusi.** Ruang virtual yang terasa seperti bermain tapi tetap terstruktur. Ngobrol terjadi alami saat avatar saling mendekat, diskusi kelompok cukup dengan duduk di meja yang sama, dan pengajar tetap memegang kontrol kelas.

**Prinsip desain.**

- Lucu dan nyaman untuk semua usia, tapi tidak kekanak-kanakan untuk mahasiswa.
- Ruang selalu tertutup per organisasi; tidak ada ruang publik.
- Berjalan lancar di laptop spek menengah dan koneksi internet rumahan Indonesia.
- Arsitektur siap untuk segmen SD/SMP di fase berikutnya tanpa rewrite.

## Persona dan role

Ada tiga role dalam satu organisasi; satu akun bisa punya role berbeda di organisasi berbeda.

| Role | Contoh orang | Kebutuhan utama | Hak akses |
| --- | --- | --- | --- |
| Admin organisasi | Staf IT sekolah, pengurus himpunan | Membuat organisasi, mengundang anggota, mengatur role | Semua, termasuk hapus anggota dan lihat laporan |
| Pengajar / host | Guru SMA, dosen, asisten dosen | Memulai sesi kelas, mengumpulkan peserta, moderasi | Broadcast, mute, kick, pindahkan peserta, kunci ruangan |
| Peserta | Siswa SMA, mahasiswa | Ikut kelas, diskusi kelompok, ngobrol santai | Bergerak, chat, voice di zona yang diizinkan, report |

Persona utama pilot: **Bu Rina**, guru Biologi SMA yang mengajar 4 kelas online per minggu dan kesulitan membuat siswa aktif, dan **Dimas**, mahasiswa semester 3 yang malas menyalakan kamera di Zoom tapi aktif di Discord.

## Scope MVP

MVP dibagi dua milestone. M1 harus bisa dipakai kelas sungguhan sebelum M2 dimulai.

| Milestone | Isi | Kriteria selesai |
| --- | --- | --- |
| M1 — Ruang + text | Auth, organisasi, undangan, peta sekolah sebagai spawn utama, 2 map ruang bawaan (kelas + taman diskusi), avatar preset, movement real-time, proximity text chat, zona meja, kontrol pengajar, moderasi dasar, mode HP (pilih zona lewat daftar + chat, tanpa movement) | Satu kelas pilot (30 orang) menjalankan sesi 60 menit tanpa gangguan besar |
| M2 — Voice | Proximity voice (LiveKit), voice per zona, broadcast voice pengajar, mute oleh pengajar, indikator siapa yang bicara | Satu kelas pilot menjalankan sesi diskusi kelompok dengan voice |

**Out-of-scope MVP:** video kamera, screen share, map editor custom, gamifikasi (poin, badge, toko item), kuis interaktif, whiteboard, mobile app native, integrasi LMS, recording sesi, segmen SD/SMP.

Screen share kemungkinan jadi kandidat pertama setelah M2, karena pengajar biasanya butuh menampilkan materi.

## User stories dan acceptance criteria

Setiap story punya ID agar bisa dirujuk di ticket, API spec, dan test. Story M1 kecuali ditandai M2.

### Auth dan organisasi

**US-01** Sebagai pengguna baru, saya ingin daftar dengan email atau akun Google agar bisa masuk tanpa ribet.

- Registrasi email wajib verifikasi lewat OTP atau link sebelum bisa masuk ruang.
- Login Google tersedia; email yang sama tidak membuat akun ganda.
- Pengguna wajib mengisi tanggal lahir; di bawah 15 tahun hanya bisa bergabung lewat undangan sekolah selama pilot.

**US-02** Sebagai admin, saya ingin membuat organisasi agar sekolah atau kampus saya punya ruang sendiri.

- Organisasi punya nama, slug unik, dan logo opsional.
- Pembuat organisasi otomatis menjadi admin.

**US-03** Sebagai admin atau pengajar, saya ingin mengundang anggota lewat kode atau link agar mereka bisa bergabung.

- Kode undangan berlaku maksimal 7 hari dan bisa dicabut kapan saja.
- Undangan menentukan role (pengajar atau peserta).
- Tanpa undangan, orang tidak bisa melihat atau masuk organisasi.

### Ruang virtual

**US-04** Sebagai peserta, saya ingin memilih avatar dan nama tampilan agar saya punya identitas di ruang.

- Tersedia minimal 12 avatar preset dengan beberapa pilihan warna.
- Nama tampilan 3–24 karakter dan melewati filter kata kasar.

**US-05** Sebagai peserta, saya ingin berjalan dengan keyboard (WASD/panah) atau klik agar bisa berpindah tempat.

- Posisi peserta lain terlihat bergerak mulus, jeda di bawah 200 ms pada koneksi normal.
- Avatar tidak bisa menembus dinding atau objek.

**US-06** Sebagai pengajar, saya ingin membuat ruang dari map bawaan agar bisa menyiapkan kelas dalam satu menit.

- Pilihan map: ruang kelas (kapasitas 40) dan taman diskusi (kapasitas 40).
- Ruang bisa dijadwalkan atau dibuka langsung.

**US-13** Sebagai peserta, saya ingin muncul di peta sekolah setelah masuk agar bisa bertemu dan ngobrol dengan teman sebelum kelas dimulai.

- Setelah login dan memilih organisasi, peserta muncul di alun-alun peta sekolah.
- Setiap gedung menampilkan jumlah orang dan kelas yang sedang live.
- Berjalan ke pintu gedung lalu memilih ruang membawa peserta ke ruang itu tanpa reload halaman.
- Ruang Guru hanya bisa dimasuki pengajar dan admin.
- Keluar dari ruang mengembalikan peserta ke depan pintu gedung asalnya.
- Proximity chat dan voice (M2) berlaku juga di peta sekolah.

### Komunikasi

**US-07** Sebagai peserta, saya ingin chat dengan orang di sekitar avatar saya agar diskusi terasa alami.

- Pesan proximity diterima orang dalam radius tertentu (default 5 tile).
- Ada tab chat "semua ruangan" terpisah, default aktif; pengajar bisa mematikannya.

**US-08** Sebagai peserta, saya ingin duduk di meja diskusi agar langsung tergabung dalam grup.

- Semua orang di zona yang sama berbagi chat (dan voice di M2), apa pun jaraknya di dalam zona.
- Masuk dan keluar zona ditandai visual yang jelas.

**US-09 (M2)** Sebagai peserta, saya ingin bicara dengan orang di dekat saya agar diskusi lebih cepat dari mengetik.

- Volume mengecil sesuai jarak dan hilang di luar radius.
- Mikrofon default mati saat masuk ruang; ada tombol mute yang jelas.
- Ikon menunjukkan siapa yang sedang bicara.

### Kontrol pengajar

**US-10** Sebagai pengajar, saya ingin broadcast pesan (dan voice di M2) ke seluruh ruangan agar semua mendengar instruksi.

**US-11** Sebagai pengajar, saya ingin memanggil semua peserta ke satu titik agar kelas bisa dimulai serentak.

- Semua avatar berpindah ke titik kumpul dalam satu aksi.

**US-12** Sebagai pengajar, saya ingin mute, kick, dan mengunci ruangan agar kelas tetap tertib.

- Peserta yang di-kick tidak bisa masuk lagi ke sesi yang sama tanpa izin pengajar.
- Ruang terkunci menolak peserta baru.

## Keamanan dan moderasi

Karena peserta SMA sebagian masih di bawah 18 tahun, aturan ini berlaku sejak M1 dan tidak bisa dimatikan oleh organisasi.

1. **Ruang tertutup.** Semua ruang hanya bisa diakses anggota organisasi; tidak ada pencarian atau direktori publik.
2. **Tidak ada DM tersembunyi.** Chat privat antar peserta tidak tersedia di MVP; semua percakapan terjadi di ruang yang dimoderasi pengajar.
3. **Report.** Setiap peserta bisa melaporkan pesan atau pengguna; laporan masuk ke pengajar dan admin organisasi.
4. **Filter chat.** Kata kasar dan pola data pribadi (nomor HP, alamat) disaring atau disamarkan.
5. **Log moderasi.** Aksi mute, kick, dan report disimpan untuk audit admin; isi chat disimpan maksimal 30 hari.
6. **Voice tidak direkam.** Audio hanya di-relay secara real-time, tidak disimpan di server.
7. **Persetujuan.** Pengguna di bawah 18 tahun bergabung lewat undangan sekolah; sekolah bertanggung jawab memperoleh persetujuan orang tua/wali sesuai UU PDP Pasal 25. Mekanisme ini perlu dikonfirmasi dengan konsultan hukum sebelum peluncuran.
8. **Data minimal.** Hanya menyimpan nama tampilan, email, tanggal lahir, role, dan keanggotaan organisasi.

## Non-functional requirements

Target ini dipakai sebagai acuan desain teknis di Fase 2 dan load test di Fase 6.

| Area | Target pilot |
| --- | --- |
| Kapasitas per ruang | 40 orang aktif bersamaan per ruang; 200 orang di peta sekolah |
| Kapasitas total | 10 ruang aktif bersamaan (±400 koneksi) |
| Latency movement | p95 < 200 ms untuk user di Indonesia |
| Update posisi | 10–15 tick/detik dari server |
| Bandwidth peserta | < 50 kbps untuk movement + chat; voice ±30–50 kbps per pembicara yang terdengar |
| Perangkat | Laptop RAM 4 GB, Chrome/Edge/Firefox terbaru; HP lewat browser (Chrome Android, Safari iOS) untuk mode HP; tablet best effort |
| Ketersediaan | 99% selama jam sekolah (07.00–17.00 WIB) |
| Lokasi server | Jakarta atau Singapura |
| Keamanan | HTTPS/WSS wajib, JWT berumur pendek + refresh token, rate limit di endpoint auth dan chat |
| Observability | Structured log, metrik koneksi aktif per ruang, error tracking |
| Bahasa | UI Bahasa Indonesia; struktur siap i18n untuk English |

## Metrik keberhasilan dan validasi

Pilot dianggap berhasil jika minimal 3 kelas memakai Kelasverse secara rutin selama 4 minggu.

| Metrik | Target | Cara ukur |
| --- | --- | --- |
| Kelas yang kembali memakai | ≥ 3 kelas, 4 minggu berturut-turut | Jumlah sesi per organisasi per minggu |
| Partisipasi peserta | ≥ 60% peserta mengirim pesan atau bicara per sesi | Event chat dan voice per user |
| Durasi rata-rata di sesi | ≥ 80% dari durasi kelas | Waktu join–leave |
| Kepuasan pengajar | Skor ≥ 4 dari 5 | Survei singkat setelah sesi |
| Kepuasan peserta | ≥ 70% lebih suka dibanding Zoom/Meet | Survei setelah minggu ke-2 |

**Rencana validasi** (paralel dengan Fase 2–4):

- [ ] Wawancara 5–10 guru SMA dan dosen tentang kelas online mereka saat ini
- [ ] Wawancara 5–10 siswa atau mahasiswa
- [ ] Cari 2–3 pengajar yang bersedia menjadi pilot setelah M1
- [ ] Revisi spec ini berdasarkan hasil wawancara

## Pertanyaan terbuka dan asumsi

**Asumsi:**

- Pengajar pilot bersedia memakai alat terpisah (Google Slides, Drive) untuk materi sampai screen share tersedia.
- Kapasitas 40 orang per ruang cukup untuk satu kelas SMA atau satu kelas kuliah reguler.
- Sebagian besar peserta mengakses dari laptop; peserta dari HP memakai mode HP.

**Pertanyaan terbuka:**

- [x] Model bisnis: gratis untuk pilot, lalu langganan per organisasi (diputuskan).
- [x] Chat "semua ruangan" default aktif (diputuskan).
- [x] Mode HP (tampilan sederhana tanpa movement) masuk M1, karena banyak siswa hanya punya HP (diputuskan).
- [x] Aset visual (avatar, tile map): desain sendiri dengan bantuan Claude (style guide, konsep avatar, tile map) di Fase 2 (diputuskan).

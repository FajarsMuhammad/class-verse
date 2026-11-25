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
| M2 — Voice | Voice per ruangan/zona (LiveKit, tanpa attenuasi jarak), broadcast voice pengajar, mute oleh pengajar, indikator siapa yang bicara | Satu kelas pilot menjalankan sesi diskusi kelompok dengan voice |

**Out-of-scope MVP:** video kamera, screen share, map editor custom, gamifikasi (poin, badge, toko item), kuis interaktif, whiteboard, mobile app native, integrasi LMS, recording sesi, segmen SD/SMP.

Screen share kemungkinan jadi kandidat pertama setelah M2, karena pengajar biasanya butuh menampilkan materi.

### Tambahan scope — MVP 2 fitur belajar (disetujui, belum diimplementasikan)

MVP 2 fitur belajar berbeda dari milestone voice **M2** di atas. Batas out-of-scope sebelumnya tetap catatan
MVP awal; perluasan ini disetujui pengguna melalui rencana `docs/tasks/mvp2-learning-plan.md`.

| Urutan | Hasil peserta/pengajar | Batas disetujui |
| --- | --- | --- |
| F1 | Unggah materi dan baca di aplikasi | MinIO privat; PDF + audio dulu, EPUB menyusul; kuota 5 GB per sekolah dan maks 50 MB/file; tanpa RAG |
| L1 → L2 | Katalog, klik rak, baca digital, pinjam/kembali fisik | Satu contoh berlabel untuk pengujian; tanpa antre/denda; maks 5 pinjaman aktif, 14 hari |
| M1 → M2 | Modul per gedung dan progres nyata | Pengajar/admin mengisi; katalog terbuka bagi semua anggota sekolah, dibuka dari mana saja; langkah bebas dibuka, progres dicatat per siswa |
| Q1 → C1 → A1 → R1 | Kuis mandiri, kelas/rombel, tugaskan modul ke kelas dan sematkan di ruang, rapor | Penilaian server; pengajar/admin mengelola kelas/siswa, admin memilih tahun ajaran aktif; peserta hanya rapor sendiri |
| Q2 | Arena Kuis live dan klasemen sesi | Batas ruang 40 sampai kapasitas baru terbukti; nilai tersimpan, bukan angka prototipe |
| B1 → B2 | Leaderboard dan gamifikasi bertahap | Redis untuk ranking mingguan sekolah + filter gedung/mapel; progres modul di Rapor, bukan ranking modul |

Identitas leaderboard: tampil **inisial**, hover menampilkan **nama lengkap dan kelas**; detail yang sama wajib
tersedia lewat focus keyboard dan tap. Ini bukan anonimisasi: anggota organisasi dapat melihat nama lengkap.
Sumber kelas: fitur rombel tersendiri (C1), **maksimal satu kelas aktif per siswa per sekolah per tahun ajaran**.
Kelas berbeda dari ruang virtual. Kelas sekolah/tahun lain tidak saling menimpa; perpindahan mempertahankan riwayat.
Data existing belum punya rombel; tidak ada penempatan otomatis. Sumber nama lengkap masih perlu dikonfirmasi
karena data sekarang hanya punya nama tampilan. Pengajar/admin mengelola kelas dan penempatan siswa; admin
memilih satu tahun ajaran aktif per sekolah. Tahun lama tetap tersedia untuk Rapor, tanpa kenaikan kelas otomatis.
C1 dikerjakan setelah Q1 sebelum R1; belum ada perubahan schema atau kode.
Tidak ada direktori publik, tidak menebak kelas dari ruang, tidak menambah nama legal tanpa keputusan terpisah.

Kriteria lintas slice: data nyata, isolasi organisasi, otorisasi server, retry tidak menggandakan pinjaman/nilai/poin,
mode HP tetap bisa dipakai, dan akun/ruang lama tidak dihapus. Rumus poin/XP dan aturan retry kuis belum final;
lihat keputusan tersisa serta acceptance criteria di rencana.

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

**US-05** Sebagai peserta, saya ingin berjalan dengan keyboard (WASD/panah) atau klik dua kali tujuan agar bisa berpindah tempat.

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

**US-09 (M2)** Sebagai peserta, saya ingin bicara dengan orang di ruangan/zona yang sama agar diskusi lebih cepat dari mengetik.

- Dalam ruangan yang sama, peserta mendengar zona yang sama dengan volume penuh, tanpa perubahan volume berdasarkan jarak. Area di luar semua zona menjadi satu kanal area umum.
- Server audio wajib menolak akses suara dari zona lain, kecuali broadcast pengajar. Memodifikasi browser tidak boleh melewati pembatasan ini.
- Mikrofon default mati saat masuk ruang; ada tombol mute yang jelas.
- Ikon menunjukkan siapa yang sedang bicara.

### Kontrol pengajar

**US-10** Sebagai pengajar, saya ingin broadcast pesan (dan voice di M2) ke seluruh ruangan agar semua mendengar instruksi.

**US-11** Sebagai pengajar, saya ingin memanggil semua peserta ke satu titik agar kelas bisa dimulai serentak.

- Semua avatar berpindah ke titik kumpul dalam satu aksi.

**US-12** Sebagai pengajar, saya ingin mute, kick, dan mengunci ruangan agar kelas tetap tertib.

- Peserta yang di-kick tidak bisa masuk lagi ke sesi yang sama tanpa izin pengajar, termasuk koneksi voice dengan token lama.
- Kick mencakup pemutusan voice yang dikonfirmasi server audio. Bila belum terkonfirmasi, pengajar melihat status tertunda/gagal dan sistem mencoba ulang; UI tidak boleh mengklaim voice sudah terputus hanya karena peserta hilang dari peta/chat.
- Ruang terkunci menolak peserta baru.

## Keamanan dan moderasi

Karena peserta SMA sebagian masih di bawah 18 tahun, aturan ini berlaku sejak M1 dan tidak bisa dimatikan oleh organisasi.

1. **Ruang tertutup.** Semua ruang hanya bisa diakses anggota organisasi, kecuali tamu pertemuan G1 yang disetujui host untuk satu ruang tujuan; tidak ada pencarian atau direktori publik.
2. **Tidak ada DM tersembunyi.** Chat privat antar peserta tidak tersedia di MVP; semua percakapan terjadi di ruang yang dimoderasi pengajar.
3. **Report.** Setiap peserta bisa melaporkan pesan atau pengguna; laporan masuk ke pengajar dan admin organisasi.
4. **Filter chat.** Kata kasar dan pola data pribadi (nomor HP, alamat) disaring atau disamarkan.
5. **Log moderasi.** Aksi mute, kick, dan report disimpan untuk audit admin; isi chat disimpan maksimal 30 hari.
6. **Voice tidak direkam.** Audio hanya di-relay secara real-time, tidak disimpan di server.
7. **Persetujuan.** Pengguna di bawah 18 tahun bergabung lewat undangan sekolah; sekolah bertanggung jawab memperoleh persetujuan orang tua/wali sesuai UU PDP Pasal 25. Tamu pertemuan G1 tidak wajib mengisi usia; host/sekolah bertanggung jawab atas undangan tamu dan persetujuan yang diperlukan. Mekanisme ini perlu dikonfirmasi dengan konsultan hukum sebelum peluncuran.
8. **Data minimal.** MVP awal menyimpan nama tampilan, email, tanggal lahir, role, dan keanggotaan organisasi.
   Perluasan fitur belajar menyimpan berkas sekolah, pinjaman, progres, jawaban/nilai, dan event poin sesuai scope
   MVP 2 di atas, termasuk kelas dan riwayat penempatan per tahun ajaran. Sumber nama lengkap serta kebijakan
   retensi data belajar harus ditetapkan sebelum
   fitur terkait dirilis; jangan menganggap retensi chat 30 hari berlaku otomatis pada rapor.

## Non-functional requirements

Target ini dipakai sebagai acuan desain teknis di Fase 2 dan load test di Fase 6.

| Area | Target pilot |
| --- | --- |
| Kapasitas per ruang | 40 orang aktif bersamaan per ruang; 200 orang di peta sekolah |
| Kapasitas total | 10 ruang aktif bersamaan (±400 koneksi) |
| Latency movement | p95 < 200 ms untuk user di Indonesia |
| Update posisi | 10–15 tick/detik dari server |
| Bandwidth peserta | < 50 kbps untuk movement + chat; voice ±30–50 kbps per pembicara yang terdengar |
| Perangkat | Laptop RAM 4 GB, Chrome/Edge/Firefox terbaru; HP lewat browser (Chrome Android, Safari iOS) dengan peta yang sama; tablet best effort |
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
- Sebagian besar peserta mengakses dari laptop; peserta dari HP memakai peta yang sama dengan sentuhan.

**Pertanyaan terbuka:**

- [x] Model bisnis: gratis untuk pilot, lalu langganan per organisasi (diputuskan).
- [x] Chat "semua ruangan" default aktif (diputuskan).
- [x] ~~Mode HP (tampilan sederhana tanpa movement) masuk M1~~ **dicabut**: HP memakai peta yang sama dengan desktop (diputuskan saat revamp tugas/modul). Protokol `list`/`seat` tetap di server sebagai warisan.
- [x] Aset visual (avatar, tile map): desain sendiri dengan bantuan Claude (style guide, konsep avatar, tile map) di Fase 2 (diputuskan).

# Kelasverse — Desain Visual

Referensi visual untuk implementasi frontend. Mockup interaktif ada di kanvas desain
(https://claude.ai/artifact/91wACgcSRmyC7kD4Kqswve, hanya bisa dibuka pemilik); screenshot-nya ada di `docs/screens/`.
Mockup adalah acuan tampilan dan alur, bukan kode: bangun ulang dengan komponen Vue dan token di `tokens.css`.

## Arah visual: "kelas yang hangat"

Lucu dan ramah untuk semua usia, tapi tidak kekanak-kanakan untuk mahasiswa. Bentuk membulat, warna lembut,
tanpa gradien, tanpa emoji di UI. Satu warna utama (Nila) untuk aksi, satu aksen (Mangga) untuk zona dan sorotan.

| Token | Nilai | Dipakai untuk |
| --- | --- | --- |
| Nila `--color-primary` | `#4F46E5` | Tombol utama, bubble chat sendiri, nav aktif |
| Mangga `--color-accent` | `#F5A524` | Zona/meja, chip zona aktif, jadwal |
| Tinta `--color-ink` | `#23203A` | Teks utama, tombol mikrofon |
| Kabut `--color-bg` | `#F7F5FF` | Latar aplikasi |

Tipografi: **Fredoka 600** untuk judul dan nama ruang/gedung, **Nunito** 400/600/700 untuk semua teks lain.
Teks isi minimal 15 px. Font di-host sendiri (lisensi SIL OFL), tanpa Google Fonts.

## Alur layar

```
Login → (pilih organisasi) → Peta sekolah → jalan ke pintu gedung → pilih ruang → Ruang kelas
                                   ↑                                                   │
                                   └──────────── keluar ruang (kembali ke pintu gedung) ┘
Pengajar/admin juga punya: Peta sekolah ⇄ Kelola ruang
HP: Peta sekolah (daftar gedung) → Ruang (daftar zona + chat), tanpa movement
```

| Layar | Screenshot | Isi utama |
| --- | --- | --- |
| Style guide | `01-style-guide.png` | Warna, tipografi, avatar preset, komponen dasar |
| Peta sekolah (desktop) | `02-peta-sekolah.png` | Peta 2D dengan gedung, alun-alun spawn, avatar lain; panel daftar gedung |
| Ruang kelas (desktop) | `03-ruang-kelas.png` | Peta kelas: papan tulis, 5 meja sebagai zona, radius proximity; toolbar pengajar; panel chat bertab |
| Kelola ruang (pengajar) | `04-kelola-ruang.png` | Kartu ruang per gedung dengan status live/terjadwal, buat ruang, undang anggota |
| HP: peta sekolah | `05-hp-peta-sekolah.png` | "Di dekatmu", daftar gedung yang bisa dibuka, input chat dekat |
| HP: di ruang | `06-hp-di-ruang.png` | Broadcast pengajar, pilih zona, teman semeja, chat zona, mic |

## Revisi layout setelah slice 6 (disetujui)

- Desktop: workspace selebar viewport dengan margin ringkas; peta mengisi area kiri dan chat tetap di kanan
  (320–360 px). Tinggi mengikuti viewport, daftar pesan scroll sendiri. Overview peta tetap, tanpa kamera follow.
- Daftar gedung bukan sidebar permanen: tombol **Daftar gedung** membuka panel sementara di area peta;
  memilih gedung membuka preview ruang dan petunjuk tujuan, tanpa teleport.
- Dekat pintu: kartu nama gedung + **Pilih ruang**. Pemilihan ruang tidak menutup chat.
- Layar sempit: pilihan tampilan **Peta / Chat**; mode HP tanpa movement tetap slice 8.

## Revisi layout setelah slice 9 (disetujui)

Kontrol ruang ada di dalam bingkai peta, bukan di luar, agar jelas bahwa aksi itu milik ruang yang sedang dibuka.
Sidebar kanan hanya berisi chat sehingga input chat selalu terlihat. Menggantikan "kontrol pengajar di atas chat".

```
┌ bingkai peta ───────────────────────────────────────┐ ┌ chat ──────┐
│ [<] Biologi XI-2   [Kumpulkan] [Kunci ruang ·] [Peserta]│ │ tab zona   │
│            Papan tulis                               │ │ pesan…     │
│   Meja 1   Meja 2   Meja 3            (peta)         │ │            │
│            Pintu keluar                              │ │            │
│ (mic) Mendengar Meja 1 · mikrofon mati [Bicara semua] │ │ [input][>] │
└─────────────────────────────────────────────────────┘ └────────────┘
```

- Baris atas: tombol kembali (ikon + `aria-label`), nama ruang, jumlah online; pengajar menambah Kumpulkan peserta,
  Kunci ruang, Chat semua ruangan, dan Peserta (popover moderasi). Peta sekolah: nama sekolah + Daftar gedung.
- Baris bawah: mic bulat 52 px, status suara, Bicara ke semua ruang (pengajar), petunjuk gerak.
- Kedua rail melayang di atas peta, satu baris dengan tinggi tetap 60 px; kamera di-fit di pita antara keduanya.
  Chip kanal suara berlebar tetap; pesan/error melayang di atas rail, tidak menambah tinggi. Status sambungan
  memakai garis putus-putus dan teks, bukan warna saja. Kontrol pengajar memakai popover **Peserta & chat**.
- Jalur klik mengutamakan jalan di luar meja selain meja asal/tujuan untuk mengurangi perpindahan kanal suara;
  meja tetap dapat dilalui jika tidak ada jalan lain.
- Tombol kembali bernama **Keluar ruang**: dari mana saja, avatar berjalan sendiri ke Pintu keluar lalu keluar (tanpa
  teleport; server tetap memeriksa jarak pintu). Bila tidak sampai dalam 15 detik, batalkan proses keluar otomatis
  dan beri tahu pengguna; jangan mengirim travel belakangan tanpa klik baru.
- Toggle menyebut aksinya ("Kunci ruang"); statusnya pill kecil Aktif/Mati, aktif bergaris Nila. `role="switch"`.
- Peta kelas memberi label "Papan tulis" dan "Pintu keluar"; label zona di kiri bawah meja agar tidak tertutup
  avatar di kursi atas; name tag yang bertumpuk digeser ke bawah.
- Mode HP: kontrol pengajar sama (dalam kartu lipat), label tanpa huruf kapital, header satu baris akun
  + satu baris navigasi yang bisa digeser.

## D1 — Referensi Menara Sains dan Ruang Guru

Referensi lokal terbaru: `kelasverse-assets/reference/m2/` (export HTML dan PNG pengguna).
Implementasi dan batas: [`tasks/design-science-staff.md`](tasks/design-science-staff.md).
Interior baru dipilih eksplisit saat membuat ruang; ruang lama tidak diganti. Jalur modul mengikuti header
teal/latar mint/node bulat referensi dengan progres nyata, tanpa gembok urutan katalog. Pemutar materi/video
memakai daftar isi dan navigasi bebas. Jadwal, XP, simulasi, rekaman dan statistik contoh tidak ditampilkan
sebagai layanan/data nyata. Modul tetap dapat dibuka dari luar ruang.

## Peta (Phaser)

- Gaya top-down, flat, sudut membulat; palet `--map-*` dan `--roof-*` di `tokens.css`.
- Peta sekolah: jalan berbentuk salib dengan alun-alun bertiang bendera di tengah (titik spawn).
  Gedung: Kelas A, Kelas B (atap Nila), Ruang Guru (bergembok, khusus pengajar), Perpustakaan (zona hening, chat saja),
  Aula, Kantin, Taman diskusi, Lapangan.
- Setiap gedung punya label mengambang (nama + badge jumlah/status) dan satu pintu sebagai titik `travel`.
- Ruang kelas: lantai kayu dengan grid tile, papan tulis di atas, meja diskusi sebagai zona bergaris putus-putus Mangga;
  zona tempat pemain berada diberi garis solid dan isi lebih tebal.
- Radius proximity pemain: lingkaran putus-putus Nila mengikuti `near_tiles` peta, sama dengan jangkauan chat
  Dekat di server: sekolah 5 tile (160 px), kelas/taman 3 tile (96 px). Ini radius jarak, bukan batas privasi meja;
  suara dan chat Meja tetap dibatasi zona. Pembicara aktif (M2): cincin `--color-speaking`.
- Name tag: pill putih teks 12 px; pemain sendiri pill Tinta, pengajar pill Nila.

## Avatar

### Revisi sporty US-04 (disetujui setelah slice 9)

- Pixel art orisinal terinspirasi game sosial, bukan aset Gather: Pelari, Topi sport, Headphone, Varsity,
  Kuncir, Hoodie, Kacamata. Headphone tanpa boom mikrofon. Tujuh gaya × enam warna pakaian = 42 pilihan.
  Kacamata berbingkai Tinta, lensa terbuka; tampak samping menampilkan gagang, tampak belakang tanpa lensa.
- Palet pakaian Nila, Mangga, Zamrud, Biru laut, Koral, Grafit; warna dasar/panel terang dari
  `--avatar-kit-*` di `tokens.css`. Warna kulit tidak berganti saat warna pakaian diganti.
- Profil: preview dan kelompok radio gaya/warna terpisah, label nama warna, indikator native radio,
  fokus terlihat, target minimal 44 px. Simpan ke akun memakai `sporty-<gaya>-<warna>`.
- Phaser: gambar 32×48 px di layar, bukan diregangkan menjadi persegi. Titik kaki (16,46) mengikuti posisi
  resmi; label tetap di bawah. Empat arah, pose diam + empat frame jalan pada 8 fps, nearest-neighbor.
  Animasi hanya saat posisi render berubah; preferensi kurangi gerakan memakai pose diam sesuai arah.
- Aset di `assets/avatars/sporty/`, generator dan pemeriksaan reproducible di
  `scripts/generate_sporty_avatars.py`. Avatar SVG lama tetap didukung dan tidak diganti saat hanya mengedit nama.

### Avatar lama (kompatibilitas)

Enam preset di `assets/avatars/` (viewBox 64×64), satu bentuk badan yang sama dengan aksesori berbeda:

| File | Nama | Warna badan |
| --- | --- | --- |
| `round.svg` | Bulat | `#7C83FD` |
| `cap.svg` | Topi | `#FFB547` |
| `glasses.svg` | Kacamata | `#2FB58A` |
| `headset.svg` | Headset | `#5BB8F0` |
| `bun.svg` | Sanggul | `#F48FB1` |
| `beret.svg` | Baret | `#A78BFA` |

Spec meminta minimal 12 kombinasi: 6 aksesori × pilihan warna badan. Nilai `avatar_key` di database berbentuk
`<aksesori>-<warna>`. Slice 4: enam aksesori × `nila`/`mangga` = 12 preset,
misalnya `cap-mangga`. Warna badan memakai token `--avatar-nila` dan `--avatar-mangga`;
aksesori mengikuti aset asli. Key `default` memakai Bulat lama. Dialog Profil dari header memuat
preview, nama, radio grid 12 pilihan, Simpan/Batal; label pilihan menyebut aksesori dan warna,
status terpilih memakai radio/border selain warna. Phaser memakai aset yang sama dengan preview.

## Komponen dasar

| Komponen | Spesifikasi |
| --- | --- |
| Tombol utama | Tinggi 48, radius 16, latar Nila, teks putih Nunito 700 15–16 px |
| Tombol sekunder | Tinggi 44–48, border 2 px `--color-border`, latar putih, teks Tinta |
| Tombol ikon (mic, kirim) | 44×44 (52 di toolbar ruang), wajib `aria-label`; mic bulat Tinta, ikon garis |
| Chip zona | Pill tinggi 36–44, aktif: latar `--color-accent-soft`, border Mangga, teks `--color-accent-ink` |
| Badge | Pill kecil 12–13 px: live (hijau), jadwal (Mangga), info (Nila lembut) |
| Bubble chat | Orang lain: `--color-surface-alt`, sudut kiri-bawah 6 px; sendiri: Nila, teks putih, sudut kanan-bawah 6 px; nama pengirim 13 px di atasnya |
| Banner broadcast | Latar `--color-primary-soft`, teks `--color-primary-strong` 600, di atas chat |
| Kartu | Putih, radius 24 (20 di HP), padding 14–28 |
| Input chat | Tinggi 44, border 2 px `--color-input-border`, radius 14, label tersembunyi untuk screen reader |

## Aturan aksesibilitas

- Semua yang bisa diketuk minimal 44×44 px.
- Kontras teks minimal 4.5:1, termasuk label abu-abu kecil (`--color-ink-muted` sudah lolos di atas putih dan Kabut).
- Pakai elemen asli (`<button>`, `<a>`, `<input>` + `<label>`), bukan `div` yang diberi click handler.
- Warna yang harus dibedakan juga berbeda terang-gelapnya, bukan hanya hue.
- Gerak di peta memakai keyboard (WASD/panah) atau klik dua kali lantai (maksimal 400 ms, jarak 24 px layar).
  Gedung/rak/stasiun tetap sekali klik. Peta menerima fokus keyboard saat masuk; fokus chat/form tidak direbut kembali.
  Mode HP tidak butuh gerak sama sekali.

## Di luar MVP

Mode gelap belum didesain. Tile art final, sprite sheet animasi, dan ikon gedung detail dibuat saat implementasi peta di Fase 4.

## Menu samping ruang (2026-11, referensi `docs/screens/sidemenu.png` dan `reff-design-1..3.png`)

- Desktop/tablet: rel ikon tipis (60 px) selalu tampil di kiri peta + satu panel yang bisa diciutkan. Kolom
  kiri "Panel belajar" dan kolom kanan chat/katalog dihapus; peta mengisi sisa lebar. Menciutkan panel menambah
  lebar peta, posisi/koneksi ruang tidak berubah.
- Item untuk semua: **Belajar** (profil, tugas, lanjutkan belajar, kursi), **Chat** (tetap ter-mount saat
  disembunyikan agar draf tidak hilang; titik merah bila ada pesan baru), **Katalog** di Perpustakaan atau
  **Stasiun** di Menara Sains/Ruang Guru.
- Guru/admin desktop: **Peta, Chat, Modul, Kelola, Rapor**; Katalog tambahan hanya di Perpustakaan.
  Katalog guru adalah panel pendamping (peta tetap terlihat) dan tidak terbuka otomatis saat masuk; guru mulai di Peta.
  Peta sekolah: kamera tidak mengikuti avatar. Mulai di posisi avatar, lalu digeser dengan klik-tahan-geser
  (gerak >6 px = geser, bukan klik; dibatasi tepi peta, peta lebih kecil dari layar tetap di tengah). Zoom di
  kanan bawah. Kartu tempat (Menuju pintu/Lihat modul) dan daftar ruang (Masuk ruang) sama-sama ringkas 280 px
  di pojok kanan atas, agar alur pilih gedung → pintu → masuk tidak berpindah posisi.
  Klik dua kali (≤400 ms) berlaku di semua peta: gedung = pilih + jalan ke pintunya; stasiun/rak di ruangan =
  pilih + jalan ke petak bebas terdekat; lantai = jalan ke titik itu. Pilihan stasiun/rak menunggu 400 ms agar panel
  yang terbuka tidak menggeser peta di antara dua klik. Taman Diskusi dan Alun-alun punya chip judul seperti gedung.
  Kartu tempat mengikuti `reff-design-4.png`: ikon + judul, tutup ikon, satu tombol utama lebar 36 px dan tombol
  ikon persegi 36 px (Lihat modul); tombol Masuk ruang 32 px.
  Panel Peta di dalam gedung punya tombol **Keluar ruang** (berjalan ke pintu lalu keluar) sebagai alternatif
  aksesibel untuk keluar otomatis di pintu. Menu akun tertutup saat klik di luar; pesan simpan profil hilang 4 detik.
  Tidak ada item Belajar terpisah untuk guru, termasuk di Ruang Guru: isi pendamping berada pada menu **Peta**
  sesuai `reff-design-3.png`. Peta membuka panel ruang 320 px di kiri dan peta aktif di kanan; dapat diciutkan.
  **Peta dan Chat** bergantian memakai panel pendamping, bukan dua kolom. Chat menampilkan konteks ruang/zona, pesan bergulir dan input tetap
  di bawah; draf serta indikator pesan baru tetap terjaga.
  **Modul, Kelola, Rapor** dan katalog guru terbuka sebagai halaman penuh di samping rel.
  Tombol **Kembali ke peta** di atas halaman kembali ke Peta beserta panel ruang; item **Peta** pada rel
  membuka/menutup panel itu. Panel pendamping memakai tombol ciut. Escape dari halaman juga kembali ke Peta.
  Peta tetap ter-mount, berdimensi nonzero, tersembunyi dan inert saat halaman dibuka: koneksi/posisi tidak
  direset dan tidak ada fokus keyboard pada peta tersembunyi. Draf chat tetap terjaga.
  **Stasiun dihapus untuk guru** karena alatnya sudah di menu. Kelola memakai satu pemilih alat:
  Kuis, Kelas, Tugas, Ruang (termasuk undangan), Buku perpustakaan, Berkas. Kontrol sesi juga di Kelola.
  Modul berisi pengelola modul (dibatasi `perpus` di Perpustakaan), dan kartu **Sematkan di ruang ini**
  pada interior. Tidak ada bilah sematan di atas peta.
- Header aplikasi dan header peta guru desktop dihapus. Peta memenuhi tinggi viewport dengan sela 4 px;
  menu awal ciut jika belum ada preferensi. Akun di bawah rel menyimpan Profil, Ganti sekolah, Moderasi,
  Keluar akun. Identitas Kelasverse di atas rel kembali ke peta. Daftar gedung dan daftar orang ada di Belajar.
  Header halaman non-peta, peserta dan mode HP tetap.
- Bingkai hitam luar interior dihapus; warna dinding luar memakai `--kv-map-ground` agar menyatu dengan
  viewport. Lantai, furnitur, pintu dan collision tidak berubah.
- Avatar keluar otomatis saat kembali ke pintu interior: setelah berjarak >56 px dari pintu, masuk radius
  40 px memicu travel sekali; spawn di pintu tidak langsung keluar. Server tetap memeriksa radius 48 px.
  Disconnect, travel dan pergantian ruang mereset pemicu. Tombol keluar mode HP tetap tersedia.
- Peserta melihat sematan sebagai kartu "Dari pengajar" di atas panel Belajar; titik pada ikon Belajar bila ada
  sematan baru saat panel tertutup. Satu elemen tegas: kartu nila bergaya catatan yang ditempel (pushpin),
  sisanya tenang.
- Status ciut menu disimpan di `localStorage` (`kv-side-collapsed`) sehingga peta tetap lebar setelah muat ulang.
  Penempatan menu saja; server tetap memeriksa peran, bukan lokasi avatar.
- Fokus: membuka item memindah fokus ke judul panel; menciutkan/Escape mengembalikan fokus ke tombol rel.
  Escape di dalam `<dialog>` (pembaca, kuis) tidak menutup panel. Pembukaan otomatis saat masuk ruang tidak
  mencuri fokus dari peta.
- Layar sempit (≤900 px): rel mendatar di atas, panel di atas peta. Mode HP (daftar) tidak berubah; alat guru di
  HP lewat halaman **Kelola sekolah** (`/kelola-ruang?bagian=ruang|modul|kelas|perpustakaan`).
- Pustakawan non-guru belum menjadi peran sendiri; tambahkan peran khusus katalog bila sekolah memakai petugas
  perpustakaan yang tidak boleh melihat nilai.

# Handoff: revisi UI/UX menu Kelola

Dokumen ini untuk Claude Code. Tujuannya mengimplementasikan revisi menu **Kelola** (alat pengajar) di Kelasverse sesuai mockup di `html/kelola/`.

Stack: **backend Go, frontend Vue**. Ikuti konvensi yang sudah ada di repo: struktur folder, cara fetch API, state management, router, penamaan, dan library komponen. Jangan menambah dependency UI baru tanpa bertanya.

---

## 0. Aturan kerja

- **Baca dulu kode Kelola yang ada** sebelum mengubah apa pun. Petakan tiap menu ke komponen/route-nya, lalu tulis ringkasan pemetaan itu sebelum mulai Fase 1.
- **Mockup adalah acuan visual dan perilaku, bukan kode.** Jangan copy inline style dari HTML. Buat komponen Vue yang rapi dengan token dari `tokens/tokens.css`. Kalau frontend sudah punya token atau warna sendiri, petakan ke yang ada dan jangan membuat duplikat.
- **Jangan ubah aturan bisnis yang sudah ada.** Contohnya: tugas membekukan penerima dan isi langkah saat dibuat, rapor memakai nilai kuis tertinggi, tahun aktif dipilih admin, file yang dipakai tidak boleh dihapus. Revisi ini mengubah tampilan dan alur, bukan aturan.
- **Jangan mengarang data.** Semua angka di UI berasal dari API. Kalau sebuah tampilan butuh data yang belum ada endpoint-nya, tandai dengan `TODO(api)` dan tanyakan dulu (lihat bagian 4).
- **Kerjakan per fase** (bagian 5). Di akhir setiap fase, jalankan lint, type-check, dan test yang ada, lalu berhenti untuk review.
- Kode: sederhana, konsisten, komponen kecil dengan satu tanggung jawab. Hindari abstraksi yang belum dipakai minimal dua kali.

---

## 1. Kondisi sekarang (diamati di app, 18 Nov 2026)

Kelola dibuka dari rail kiri (khusus role Pengajar/Admin) dan tampil sebagai halaman penuh dengan 10 menu datar:

| Menu | Isi sekarang |
|---|---|
| Modul belajar | Daftar modul (judul, gedung · jenjang · n langkah, chip Terbit/Draf, tombol Buka), filter Jenjang, tombol **+ Modul baru**. Form modul baru: pilihan template (Materi bacaan, Praktikum, Mulai kosong), Gedung, Jenjang, Judul, Deskripsi. |
| Kuis | Daftar kuis (gedung · jenjang · n soal · lulus 60), filter Jenjang, **+ Kuis baru**. |
| Kelas | Pilih tahun ajaran, cari kelas, daftar 1A–2B (n siswa · wali kelas · n guru mapel), **+ Kelas baru**. |
| Siswa | Tahun ajaran, filter kelas (termasuk "Belum punya kelas"), cari, pilih semua, dropdown kelas per siswa, tombol Riwayat. |
| Tugas | Form "Tugaskan modul ke kelas" (tahun, kelas, modul terbit, tenggat opsional) **di atas** daftar "Tugas sekolah". |
| Ruang & undangan | Grid kartu ruang (nama, Siap/Live, gedung · kapasitas, Lihat di peta, Ubah, Arsipkan), kartu "Ruang baru", lalu bagian "Undang anggota" (Sebagai, Berlaku, Batas pemakaian) di bawah. |
| Buku perpustakaan | Kartu buku (judul, penulis, jenjang, jenis, status, ketersediaan, Ubah, Arsipkan), **+ Tambah buku**. |
| Berkas | Input file bawaan browser, kuota 0/5.000 MB, daftar file (Lanjutkan, Hapus). PDF/MP3/M4A ≤ 50 MB, upload bisa dilanjutkan 24 jam. |
| Tahun ajaran | Input tahun baru + Tambah, daftar tahun dengan chip Aktif. |
| Kontrol sesi | Tombol Undang tamu, Kumpulkan, Kunci ruang (Mati), Peserta & chat (0). |

Masalah UX utama:
1. 10 menu datar tanpa pengelompokan, tidak ada penanda menu yang butuh tindakan.
2. Pola header, filter, dan aksi utama beda-beda tiap halaman.
3. Form diletakkan di atas daftar (Tugas, Ruang), jadi halaman terasa seperti form, bukan daftar.
4. Masalah yang menghambat siswa (kuis 0 soal, kelas tanpa wali/siswa) tidak terlihat.
5. Aksi per siswa pakai dropdown satu-satu, tidak ada aksi massal.
6. Empty state hanya berisi teks "Belum ada…" tanpa langkah berikutnya.

Bug yang ditemukan saat pengecekan:

| # | Tempat | Masalah | Perbaikan |
|---|---|---|---|
| B1 | Peta, kartu Ruang Guru | Untuk role **Peserta**, tombol "Menuju pintu" aktif tapi tidak melakukan apa-apa. | Tombol disabled dengan teks "Hanya untuk pengajar", atau sembunyikan. |
| B2 | Kelola → Buku perpustakaan → Tambah buku | Saat Format = E-book/Audio, field **File** dan **Warna sampul** keluar dari grid label–isian (label terpisah dari isiannya). | Selesai otomatis dengan desain baru di 3.5b. Kalau form lama masih dipakai sementara, perbaiki grid-nya. |
| B3 | Tambah buku, Format = Audio | Pilihan File menampilkan file **PDF**. | Filter file Berkas per format: E-book → PDF, Audio → MP3/M4A. Lebih baik difilter di API (`?type=`), bukan hanya di UI. |

Catatan form Tambah buku sekarang (19 Nov 2026), untuk konteks 3.5b:
- Field: Judul, Penulis, Rak, Jenjang, Format, Eksemplar (Buku) atau File (E-book/Audio), Warna sampul, checkbox "Terbitkan ke katalog".
- Opsi Rak: Dongeng, Novel, Komik Edu, Sains, Matematika, Sejarah, Inggris, Referensi.
- Opsi Jenjang: SD, SMP, SMA, Umum. Opsi Format: Buku, E-book, Audio.
- Opsi Warna sampul: Hijau tua, Biru, Cokelat, Merah muda, Oranye, Hijau, Ungu, Merah.
- Form muncul di atas daftar buku dalam halaman yang sama.

---

## 2. Kerangka bersama (berlaku untuk semua menu)

Lihat layar mana pun di `html/kelola/`. Strukturnya:

```
AppRail (72px) | KelolaSidebar (248px) | Konten
                                        ├─ Topbar: breadcrumb · "Anda di <ruang>" · Kembali ke peta
                                        └─ main (max 1180px, padding 28/32)
                                           ├─ PageHeader: judul, deskripsi, 1 aksi utama (kanan)
                                           ├─ Toolbar: StatusTabs · filter · search
                                           └─ isi (tabel / grid kartu / layout 2 kolom + panel)
```

Responsif: semua baris memakai `flex-wrap`. Di layar sempit, sidebar dan panel samping turun ke bawah, dan tabel di-scroll horizontal di dalam kotaknya.

### Komponen yang dibuat (nama boleh disesuaikan dengan konvensi repo)

| Komponen | Props / perilaku |
|---|---|
| `KelolaLayout` | Slot `header`, `toolbar`, `default`, `aside` (opsional, lebar 340px). Merender Topbar + main. |
| `KelolaSidebar` | Item dikelompokkan (lihat di bawah). Tiap item: ikon, label, `badge` opsional (`count`, `warn`, `live`). Item aktif: `aria-current="page"`. |
| `PageHeader` | `title`, `description`, slot `actions` (maks 1 primary + 1 secondary). |
| `StatusTabs` | `items: {key, label, count}[]`, `v-model`. Pakai `role="tablist"`. |
| `FilterSelect`, `SearchInput` | Selalu dengan `<label>` (boleh visually-hidden). Tinggi 40px. |
| `DataTable` | Kolom terdefinisi, baris dengan aksi di kolom terakhir rata kanan. Dibungkus `overflow-x: auto`. |
| `StatusChip` | `kind: ok \| draft \| warn \| info \| neutral`. Warna **selalu** disertai teks. |
| `LiveChip` | Titik hijau + teks ("Live", "Live · 1"). |
| `InlineBanner` | `kind: warn \| info \| ok`, judul, teks, slot aksi. |
| `EmptyState` | Ikon, judul, penjelasan, 1 aksi. |
| `RowMenu` | Tombol ikon titik tiga (`aria-label` wajib) untuk aksi jarang: Ubah, Arsipkan, Hapus. |
| `SidePanel` | Panel di kolom `aside` untuk form/detail (Tugas baru, detail Kelas, Undang anggota). Ada tombol tutup. |
| `BulkActionBar` | Muncul saat ada baris terpilih: "n dipilih", aksi, Batal. |
| `ToggleSwitch` | `role="switch"`, `aria-checked`. |

### Pengelompokan sidebar

```
Beranda
KONTEN BELAJAR   Modul belajar · Kuis · Buku perpustakaan · Berkas
KELAS & SISWA    Kelas · Siswa · Tugas
RUANG            Ruang & undangan · Kontrol sesi
PENGATURAN       Tahun ajaran
```

Badge per item: jumlah item (abu-abu), jumlah masalah (merah, misalnya Kuis = jumlah kuis 0 soal, Kelas = jumlah kelas tanpa wali), atau `Live` (hijau) untuk Kontrol sesi kalau ada sesi berjalan.

### Aturan visual
- Token di `tokens/tokens.css`. Font: pakai font aplikasi yang sudah ada. Nunito hanya dipakai di mockup.
- Tombol & filter 40px, input form 44px, radius tombol 12px, kartu 18px.
- Satu tombol primary per area. Aksi berbahaya (Hapus, Arsipkan) tidak pernah primary dan selalu di `RowMenu`.
- Kontras teks minimal 4.5:1. Ikon tanpa label wajib punya `aria-label`.

---

## 3. Spesifikasi per layar

Format: **File mockup** → perubahan → data.

### 3.1 Beranda (baru) → `kelola/Kelola-Beranda.html`
- Jadi halaman default saat Kelola dibuka.
- 4 kartu angka (Modul terbit, Kuis terbit, Siswa, Tugas aktif). Tiap kartu adalah link ke menunya.
- **Perlu perhatian**: daftar masalah yang diurutkan dari yang paling menghambat siswa, dan tiap baris punya tombol aksi ke halaman terkait:
  1. Kuis dengan 0 soal
  2. Kelas tanpa wali kelas
  3. Kelas tanpa siswa
  4. Modul draf
  5. Belum ada tugas untuk kelas yang punya siswa
- **Buat cepat**: Modul baru, Kuis baru, Tugaskan modul, Undang anggota.
- **Sedang berlangsung**: ruang live + jumlah orang + tombol Kontrol.
- Data: idealnya satu endpoint ringkasan (lihat 4.1). Sementara bisa diturunkan di frontend dari endpoint list yang sudah ada.

### 3.2 Modul belajar → `kelola/Kelola-Modul.html`
- StatusTabs: Semua / Terbit / Draf / Arsip (dengan jumlah). Filter: Gedung, Jenjang. Search judul.
- Tabel kolom: Modul (judul + subteks), Gedung (chip), Jenjang, Isi (n langkah), Kuis akhir, Status, Aksi.
- Aksi per baris: modul terbit → **Tugaskan** (ke halaman Tugas dengan modul sudah terpilih). Draf → **Lanjutkan edit**. Lainnya di `RowMenu`.
- Kolom "Kuis akhir" dan banner "tautkan kuis" bergantung pada 4.2. Kalau belum ada, sembunyikan dulu.
- Toggle tampilan daftar/kartu opsional, boleh ditunda.

### 3.3 Modul baru → `kelola/Kelola-ModulBaru.html`
- Pertahankan field yang sudah ada (template, Gedung, Jenjang, Judul, Deskripsi).
- Stepper 4 langkah di atas: Mulai & info dasar → Isi konten → Kuis akhir → Terbitkan. Fase ini hanya langkah 1 yang interaktif; sisanya penanda posisi.
- Pilihan "Mulai dari": Pakai template / Upload file / Mulai kosong. "Upload file" boleh disabled dengan keterangan "Segera" kalau belum ada backend parsing.
- Jenjang dan Gedung memakai tombol pilihan (chip), bukan `<select>`, karena opsinya sedikit dan sering dipakai.
- Teks bantuan: "Disimpan otomatis sebagai draf…" hanya ditampilkan kalau memang ada autosave. Kalau tidak ada, pakai teks yang sekarang ("Disimpan sebagai draf. Tambahkan isi, lalu terbitkan.").

### 3.4 Kuis → `kelola/Kelola-Kuis.html`
- Banner peringatan di atas kalau ada kuis 0 soal.
- Tabel: Kuis, Soal (merah + ikon kalau 0), Nilai lulus, Modul (lihat 4.2), Status, Aksi.
- Aksi: 0 soal → **Tambah soal** (primary). Draf → **Edit soal**. Terbit → **Lihat hasil** (kalau halaman hasil belum ada, pakai Buka).
- Tombol Terbitkan diblokir di backend dan di UI kalau soal = 0. Cek apakah validasi backend sudah ada.

### 3.5 Buku perpustakaan → `kelola/Kelola-Buku.html`
- StatusTabs + filter Jenis & Jenjang + search judul/penulis.
- Kartu: sampul (warna placeholder kalau tidak ada gambar), judul, penulis, chip, ketersediaan, aksi. Draf mendapat tombol **Terbitkan**.
- Kartu terakhir "Tambah buku" (dashed) sebagai empty/add state.
- Tombol "Tambah buku" dan kartu dashed membuka halaman **Tambah buku** (3.5b), bukan form di atas daftar.

### 3.5b Tambah buku → `kelola/Kelola-TambahBuku.html` (interaktif, coba di browser)
Halaman/route sendiri: `Kelola / Buku perpustakaan / Tambah buku`. Dua kolom: form di kiri, pratinjau + simpan di kanan (turun ke bawah di layar sempit). Formulir **Ubah buku** memakai komponen yang sama dengan data terisi.

Urutan form (bernomor 1–4):
1. **Jenis buku** di paling atas: 3 kartu pilihan (Buku fisik / E-book / Buku audio). Nilainya sama dengan field Format yang ada (`Buku`, `E-book`, `Audio`); hanya labelnya yang lebih jelas. Mengganti jenis akan mengosongkan pilihan file.
2. **Info buku**: Judul (wajib, error inline "Judul belum diisi."), Penulis (opsional), Rak dan Jenjang sebagai pilihan chip (`role="radiogroup"`), bukan `<select>`. Default Rak & Jenjang bisa mengikuti pilihan terakhir guru.
3. **Isian sesuai jenis**:
   - Buku fisik → stepper Eksemplar (− / +, min 1), dengan penjelasan cara stok bekerja.
   - E-book → daftar PDF dari Berkas sebagai radio list (nama, jenis, ukuran) + strip "Upload file baru" yang mengunggah ke Berkas lalu langsung memilihnya.
   - Audio → hanya MP3/M4A (lihat B3). Kalau kosong, empty state "Belum ada file audio di Berkas" + tombol upload.
   - Error inline kalau file belum dipilih saat disimpan.
4. **Warna sampul**: 8 bulatan warna (`role="radio"`, `aria-label` = nama warna), nama warna terpilih tampil di bawah. Nilai yang disimpan tetap kunci warna yang sudah ada di backend; petakan ke hex di frontend.

Panel kanan:
- **Pratinjau di rak**: sampul dengan warna terpilih + judul + penulis, chip (jenjang, jenis, rak), ketersediaan ("n dari n eksemplar tersedia" atau "Selalu tersedia"), chip status Draf/Terbit. Pratinjau berubah langsung saat mengetik.
- **Siap disimpan?**: checklist (judul, eksemplar/file, rak · jenjang).
- Tombol **Simpan & terbitkan** (primary) dan **Simpan sebagai draf** menggantikan checkbox "Terbitkan ke katalog". Keduanya memanggil endpoint simpan yang sama, beda nilai `published`.
- Kalau data belum lengkap: banner ringkasan error di atas + fokus ke field pertama yang salah. Kalau berhasil: banner sukses dengan "Lihat katalog" dan "Tambah buku lain" (reset form).

Tidak ada field baru, jadi tidak perlu perubahan skema. Yang perlu dicek di backend hanya filter file Berkas per jenis (B3) dan upload dari form ini (pakai endpoint upload Berkas yang sudah ada).

### 3.6 Berkas → `kelola/Kelola-Berkas.html`
- Meteran kuota (dipakai / total).
- Area drag-and-drop + tombol "Pilih file" yang menggantikan input file bawaan. Pertahankan resumable upload yang sudah ada.
- Tabel: File (ikon jenis + nama), Jenis, Ukuran, **Dipakai di** (lihat 4.3), Status, Aksi.
- Hapus dinonaktifkan kalau file masih dipakai, disertai tooltip alasannya.

### 3.7 Kelas → `kelola/Kelola-Kelas.html`
- Kartu dikelompokkan per tingkat (diturunkan dari nama kelas atau field tingkat kalau ada).
- Peringatan "Belum ada wali kelas" di kartu, dengan tombol **Atur** inline. Kelas kosong mendapat aksi **Tambah siswa**.
- Klik kartu membuka `SidePanel` detail: pilih wali kelas, daftar siswa, tugas aktif.
- Banner "Atur sekaligus" kalau semua/banyak kelas tanpa wali (boleh ditunda).

### 3.8 Siswa → `kelola/Kelola-Siswa.html`
- Filter kelas pindah ke kolom kiri (daftar dengan jumlah per kelas, termasuk "Belum punya kelas").
- Tabel dengan checkbox. Saat ada yang dipilih, `BulkActionBar` muncul dengan isi: "Pindahkan ke [kelas] → Pindahkan".
- Dropdown kelas per baris diganti chip kelas. Pindah satu siswa memakai checkbox + bar yang sama.
- Tombol header: Impor CSV (boleh disabled kalau belum ada), Tambah siswa.
- Data: lihat 4.4 untuk pindah massal.

### 3.9 Tugas → `kelola/Kelola-Tugas.html`
- **Daftar dulu, form di panel samping.** Tombol "Tugas baru" membuka `SidePanel`.
- StatusTabs: Aktif / Lewat tenggat / Selesai. Filter kelas, search.
- Form 3 langkah di panel: (1) Kelas, dengan kelas 0 siswa disabled dan diberi keterangan; (2) Modul terbit; (3) Tenggat opsional.
- Banner info aturan pembekuan ditampilkan **sebelum** tombol submit, bukan di atas halaman.
- Tombol submit menyebut targetnya ("Tugaskan ke 1A") dan tetap disabled sampai kelas & modul dipilih.
- Kartu tugas (setelah ada data): nama modul · kelas, tenggat, progress bar, hitungan selesai/sedang/belum mulai. Di mockup bagian ini memakai placeholder `[...]` karena belum ada data; jangan dirender seperti itu di app.
- Empty state menjelaskan apa yang terjadi setelah tugas dibuat.

### 3.10 Ruang & undangan → `kelola/Kelola-Ruang.html`
- Ruang dikelompokkan per gedung. StatusTabs: Semua / Live / Arsip. Filter gedung, search.
- Kartu: mini-peta, nama, Siap/Live, gedung · kapasitas, aksi **Masuk** (menggantikan "Lihat di peta"), **Kontrol** kalau live, `RowMenu` untuk Ubah/Arsipkan.
- "Undang anggota" pindah ke `SidePanel` kanan, ditambah field **Ke kelas** dan daftar **Undangan aktif** (lihat 4.5).

### 3.11 Kontrol sesi → `kelola/Kelola-Sesi.html`
- Kartu ruang aktif di atas: nama, LiveChip, n dari kapasitas, pilih ruang lain.
- 4 kartu aksi: Kunci ruang (toggle), Kumpulkan, Bisukan semua, Undang tamu. Masing-masing diberi satu kalimat penjelasan.
- Daftar peserta dengan tab Peserta / Chat.
- **"Bisukan semua" adalah fitur baru**, tanyakan dulu sebelum membuat endpoint-nya (lihat 4.6).

### 3.12 Tahun ajaran → `kelola/Kelola-Tahun.html`
- Daftar tahun sebagai kartu. Tahun aktif ditandai border + chip "Aktif", dengan ringkasan isinya (n kelas, n siswa).
- Form "Siapkan tahun ajaran baru" dipisah ke kartu sendiri.
- Banner panduan naik kelas.
- Aksi "Jadikan aktif" (kalau sudah ada) wajib memakai dialog konfirmasi yang menjelaskan dampaknya.

---

## 4. Kebutuhan API yang perlu dicek

Sebelum membuat endpoint baru, cek dulu apakah datanya sudah tersedia. Kalau belum, **tanyakan ke saya** dan usulkan bentuk endpoint-nya.

| # | Kebutuhan | Dipakai di | Catatan |
|---|---|---|---|
| 4.1 | Ringkasan Kelola: hitungan per status + daftar "perlu perhatian" | Beranda, badge sidebar | Contoh: `GET /api/kelola/summary`. Hitung di backend supaya satu request. |
| 4.2 | Relasi kuis ↔ modul (kuis akhir modul) | Modul, Kuis | Perubahan skema. Tunda kalau belum diputuskan; sembunyikan kolomnya. |
| 4.3 | Pemakaian berkas (dipakai di modul/buku mana) | Berkas | Diperlukan untuk menonaktifkan Hapus dengan alasan yang jelas. |
| 4.4 | Pindah kelas massal | Siswa | Contoh: `POST /api/students/move` `{studentIds[], classId, academicYearId}`. Satu transaksi, kembalikan hasil per siswa. |
| 4.5 | Undangan: target kelas + daftar undangan aktif + cabut | Ruang & undangan | Cek field yang sudah ada di endpoint undangan. |
| 4.6 | Bisukan semua peserta | Kontrol sesi | Fitur baru, butuh event realtime. Konfirmasi dulu. |
| 4.7 | Hitungan per filter (tab counts) | Semua list | Bisa dari response list (`meta.counts`) supaya tidak ada request tambahan. |

Gaya backend Go: handler tipis → service → repository, error dibungkus dengan konteks, validasi di service, query di repository dengan parameter (tanpa string concat). Ikuti pola yang sudah ada di repo kalau berbeda.

---

## 5. Urutan fase

| Fase | Isi | Selesai jika |
|---|---|---|
| 1 | Kerangka: `KelolaLayout`, `KelolaSidebar` (dengan grup, badge masih statis/dari list), `PageHeader`, `StatusTabs`, `StatusChip`, `InlineBanner`, `EmptyState`, `RowMenu`. Pasang ke semua 10 menu tanpa mengubah isinya. | Semua menu memakai kerangka baru, perilaku lama tetap jalan. |
| 2 | Modul belajar, Modul baru, Kuis | Sesuai 3.2–3.4; aksi per baris jalan. |
| 3 | Kelas, Siswa (+ bulk move), Tugas (panel samping) | Sesuai 3.7–3.9. |
| 4 | Ruang & undangan, Kontrol sesi, Buku + **Tambah/Ubah buku**, Berkas, Tahun ajaran | Sesuai 3.5, 3.5b, 3.6, 3.10–3.12; bug B2 & B3 tertutup. |
| 5 | Beranda + endpoint ringkasan + badge sidebar dari data asli | Sesuai 3.1 dan 4.1. |
| 6 | Perbaikan tombol "Menuju pintu" untuk Peserta (bug B1) dan rapikan sisa `TODO(api)` | Tidak ada tombol mati untuk role yang tidak berhak. |

Catatan: Beranda dan pengelompokan sidebar sudah ada di app per 19 Nov 2026. Cek dulu apa yang sudah selesai dan lanjutkan dari situ; jangan kerjakan ulang.

Di setiap fase:
- Tambah/ubah test komponen untuk perilaku penting: tab filter, bulk select, tombol disabled saat syarat belum terpenuhi.
- Cek tampilan di lebar 1440px dan 390px.
- Laporkan file yang diubah dan hal yang sengaja ditunda.

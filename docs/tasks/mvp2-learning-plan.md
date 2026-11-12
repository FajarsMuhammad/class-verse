# MVP 2 — Rencana fitur belajar

Status: **cakupan dan urutan disetujui; rincian teknis untuk ditinjau**. Belum ada kode fitur belajar.
Keputusan pengguna dicatat di bawah dan diringkas di `docs/spec.md` serta `docs/design.md`.
Usulan teknis bukan persetujuan diam-diam atas aturan produk baru. Checklist pelaksanaan ada di dokumen ini,
memakai konvensi repo `docs/tasks/`, bukan direktori task kedua.

## Keputusan

| Tanggal | Keputusan | Akibat |
| --- | --- | --- |
| 2026-11-11 | Modul dan e-book **dibaca di dalam aplikasi**, bukan tautan keluar | File diunggah dan disimpan di Kelasverse (slice F1), lalu dirender oleh pembaca di aplikasi |
| 2026-11-11 | **Belum perlu RAG** | Tidak ada embedding, vector store, atau LLM; "Tanya pustakawan" membuka chat ke pengajar |
| 2026-11-11 | **Leaderboard memakai Redis** | ADR-011 di `docs/design.md`; Redis ditambahkan ke compose; Postgres tetap sumber kebenaran poin |
| 2026-11-11 | Penyimpanan file: **MinIO** di VPS pilot | MinIO di `compose.yaml` dev dan VPS; client S3-compatible sehingga bisa pindah ke S3/R2 |
| 2026-11-11 | Pembaca: **PDF + audio dulu**, EPUB menyusul | `pdfjs-dist` + `<audio>`; tidak ada `epubjs` di F1 |
| 2026-11-11 | **Antre buku ditunda** | L2 hanya Pinjam/Kembalikan dan "Sedang dipinjam" |
| Persetujuan lanjutan | **Kuota B: 5 GB per sekolah** | Batas total file sekolah; batas 50 MB per file tetap terpisah |
| Persetujuan lanjutan | **Klasemen per sesi kuis + leaderboard mingguan sekolah, filter mapel/gedung** | Tidak ada leaderboard per modul atau per rombel pada tahap ini |
| Persetujuan lanjutan | **Urutan F1 → L1 → L2 → M1 → M2 → Q1 → R1 → Q2 → B1 → B2** | Tiap slice diverifikasi sebelum lanjut |
| Persetujuan lanjutan | **Inisial; hover menampilkan nama lengkap dan kelas** | Sediakan juga focus keyboard dan tap; kelas dari rombel, bukan ruang virtual |
| Persetujuan kelas | **Satu kelas aktif per siswa per sekolah per tahun ajaran** | Kelas/rombel menjadi data tersendiri; bukan label bebas profil atau kelas mata kuliah jamak |
| Persetujuan C1 | **Pengajar/admin mengelola kelas dan siswa; admin memilih tahun ajaran aktif** | C1 setelah Q1 sebelum R1; perpindahan menyimpan riwayat, tidak ada kenaikan kelas otomatis |

Sumber: `kelasverse-assets/README.md`, `reference/Main.dc.html` (header, panel belajar, leaderboard), `Room.dc.html`
(modul berjenjang di gedung), `Quiz.dc.html` (Arena Kuis live), `Library.dc.html` + gambar
`Perpustakaan — rak, katalog & pinjam@2x.png` (katalog dan pinjam).

## Prinsip

- **Data nyata saja.** Angka prototipe (XP 320/500, 64 pemain, 2.110 poin) tidak pernah tampil sebagai data. UI kosong
  diberi teks kosong yang jujur ("Belum ada modul").
- **Per organisasi.** Semua data baru terikat `org_id` secara langsung atau lewat induk yang diverifikasi;
  query tetap memfilter organisasi dan memeriksa membership. FK komposit mencegah relasi lintas organisasi.
- **Pengajar/admin mengisi, peserta memakai.** Peserta tidak bisa membuat konten.
- **Vertical slice.** Setiap slice = migration + sqlc + service + handler + OpenAPI + UI + test + uji browser.
- **Poin dihitung server.** Client tidak pernah mengirim skor; server menilai jawaban.
- **MVP 1 dan MVP 2 terpisah.** Bagian tambahan MVP 2 di `docs/spec.md` memperluas scope tanpa menghapus batas
  historis MVP 1. MVP 2 bukan milestone voice M2 lama.

## Urutan dan ketergantungan

Urutan kerja **disetujui: F1 → L1 → L2 → M1 → M2 → Q1 → C1 → R1 → Q2 → B1 → B2**.

| Slice | Dependensi | Batas agar urutan tetap bisa dikerjakan |
| --- | --- | --- |
| F1 | Auth, organisasi, membership yang sudah ada | Upload dan baca satu file sebelum katalog/modul |
| L1 | F1 | Katalog dan pengelolaan, belum peminjaman |
| L2 | L1 | Stok dan pinjaman nyata, tanpa antre |
| M1 | F1 | Materi PDF/audio/teks; langkah kuis belum tersedia |
| M2 | M1 | Progres materi; aturan lulus kuis baru dihubungkan setelah Q1 |
| Q1 | M1, M2 | Bank soal, kuis mandiri, lalu sambungkan langkah kuis ke modul |
| C1 | Q1 dalam urutan pengerjaan; auth/org untuk data kelas | Tahun ajaran, kelas, roster dan riwayat penempatan |
| R1 | M2, Q1, C1 | Progres, nilai mandiri dan filter kelas; riwayat Arena baru muncul setelah Q2 |
| Q2 | Q1, R1 | Sesi live memakai penilaian Q1, hasil masuk Rapor |
| B1 | M2, Q1, Q2 | Redis dibangun dari hasil yang sudah tersimpan; rincian poin perlu disetujui |
| B2 | B1 | Formula XP/misi/lencana ditetapkan sebelum pengerjaan |

Tambahan **C1 — Kelas/rombel** disetujui setelah Q1, sebelum R1; menjadi sumber filter kelas R1 dan tooltip
kelas B1. Q1 adalah urutan pengerjaan, bukan dependensi FK kelas terhadap kuis. Belum mulai implementasi.

Tidak ada migration yang merujuk tabel kuis sebelum tabel itu ada. Spike pembaca PDF/range dan rekoneksi Arena
boleh dilakukan lebih awal untuk menguji risiko, tetapi bukan mengirim fitur yang belum lengkap.

## Berkas: dibaca di aplikasi

Diputuskan: e-book dan materi modul dibuka di dalam Kelasverse. RAG tidak dipakai sekarang; bila nanti diperlukan,
RAG memakai berkas yang sama (dokumen tetap harus tersimpan dulu), jadi F1 tidak sia-sia.

## Slices

### F1 — Penyimpanan file dan pembaca di aplikasi

Fondasi untuk L1 (e-book/audio) dan M1 (materi berkas).

**Blocker verifikasi upstream:** saat persiapan implementasi, API GitHub `minio/minio` mengembalikan
`archived: true`; README resmi menyatakan repository tidak lagi dipelihara dan binary community lama tidak
menerima pembaruan. Pengguna menyetujui **evaluasi AIStor Free**, bukan otomatis penerimaan lisensi atau
penggantian deployment. Hasil evaluasi di bawah. Keputusan MinIO sebelumnya tidak diubah diam-diam: jangan pin image legacy untuk pilot
production atau mengganti penyedia sebelum pengguna memilih. Sumber:
[repository resmi](https://github.com/minio/minio), [metadata repository](https://api.github.com/repos/minio/minio).
Catatan historis: saat blocker ditemukan belum ada kode. Setelah evaluasi dan izin lanjut, fondasi storage
lokal sudah dibuat (lihat progres di bawah); MinIO OSS legacy tidak dipakai.

#### Progres F1 — fondasi storage lokal (sebagian F1-01)

- [x] AIStor RELEASE.2026-09-19T17-05-25Z dipin dengan digest di profile Compose opsional `storage`;
  endpoint/console hanya loopback 9100/9101, license mount read-only. License command melaporkan plan FREE;
  nilai secret tidak ditampilkan atau disimpan dalam dokumen.
- [x] `make storage-up` membuat bucket privat dan akun aplikasi terbatas `materials`, kredensial acak file lokal
  mode 600; `.config/.gitignore` dan `.dockerignore` mencegah secret masuk Git/build context.
- [x] Adapter `internal/file/storage.go` memakai `minio-go/v7` v7.4.1, tanpa route HTTP publik baru.
  Test real AIStor: PUT, full GET, range, invalid range, DELETE dan anonymous GET 403; unit error path juga diuji.
- [x] Fondasi metadata F1-02: migration baru `00006_learning_files.sql`, query sqlc dan service internal.
  Reservasi kuota atomik, limit 50 MB/file dan 5 GB/org, role teacher/admin, pembatas organisasi, finalisasi digest
  idempotent, status pending/ready/deleting/deleted. Kuota baru dilepas setelah konfirmasi objek terhapus.
  Diuji dengan Postgres disposable termasuk dua reservasi serentak dan rollback metadata gagal.
- [x] Migration 6 Up/Down/Up di DB disposable; regression rename migration 5 kini menargetkan versinya secara
  eksplisit agar migration berikutnya tidak mematahkan test. Migration lama tidak diubah.
- [x] Upload resumable chunk 5 MiB (S3 multipart): migration `00007_resumable_uploads.sql`, endpoint
  `POST/GET/DELETE /api/orgs/{org_id}/uploads[/{id}]`, `PUT .../parts/{n}`, `POST .../complete`. ID sesi dibuat
  klien dan disimpan sebelum POST, jadi retry tidak menggandakan reservasi; status part dibaca dari storage; hanya
  chunk yang belum ada dikirim ulang. SHA-256 file utuh diverifikasi saat assemble; sesi 24 jam, cleanup kedaluwarsa
  saat startup dan tiap menit melepas kuota. Panel uji di Kelola ruang (guru/admin). File tetap `pending` sampai
  validasi format ada.
- [x] Validasi isi saat assemble: header/trailer PDF, tolak PDF berpassword (`pdf_encrypted`), MP3 (ID3/frame)
  dan M4A (`ftyp`); file palsu dibuang dan kuota langsung dilepas. File valid → `ready` dengan `content_type`.
- [x] Baca: `GET /files` (daftar + kuota + posisi), `GET /files/{id}/content` lewat origin aplikasi dengan Bearer,
  satu Range `bytes=`, membership dicek tiap request; `PUT /files/{id}/position`; `DELETE /files/{id}` oleh
  pengunggah/admin. Storage tidak pernah terbuka ke browser.
- [x] **Keputusan transport (F1-01):** klien mengambil isi file utuh dengan Bearer dan memakai Blob URL untuk
  `<audio>` (native tak bisa kirim header) dan data PDF.js. Tanpa presigned URL, tanpa token di query, tanpa
  perubahan auth. Batas: file ≤50 MB diunduh penuh sebelum diputar; ganti ke range transport PDF.js bila PDF besar
  terasa lambat di HP.
- [x] Reader: `pdfjs-dist` 6.5.438 (Apache-2.0, dipin persis, worker lokal versi sama, dimuat lazy hanya saat PDF
  dibuka); navigasi halaman, lompat halaman, zoom, panah kiri/kanan, text layer untuk seleksi/screen reader, render
  dibatalkan saat pindah/tutup. Audio `<audio>` native. Posisi (halaman/detik) disimpan ter-debounce dan saat tutup.
- [x] Verifikasi browser Chrome desktop 1440 px dan 390 px (`/tmp/kv-e2e/reader.mjs`, fixture CC0 buatan sendiri):
  chunk 2 diputus lalu dilanjutkan tanpa mengirim ulang chunk 1, satu reservasi; PDF berpassword, WAV, WAV
  diganti nama `.mp3` ditolak; kuota hanya file diterima; PDF 2 halaman + text layer; seek audio; posisi pulih
  setelah reload; peserta bisa baca dan Range 206 tetapi upload/hapus 403, tanpa token 401.
- [ ] Belum: Safari/iOS dan HP fisik, backup/restore objek+metadata. Aturan file draf/terbit kini lewat L1.

#### Evaluasi AIStor Free — hasil sumber resmi

**Kesimpulan evaluasi:** layak sebagai pilot standalone satu VPS, **bersyarat**, bukan pengganti OSS dengan
lisensi yang sama. Pengguna kemudian menyediakan license lokal dan meminta melanjutkan implementasi; runtime
lokal sudah diuji. Tidak membuat akun SUBNET atau menerima ketentuan lewat formulir atas nama pengguna.

| Aspek | Temuan dan dampak untuk Kelasverse |
| --- | --- |
| Lisensi | Proprietary, royalty-free, standalone; agreement Free §1 mengizinkan commercial production. Bukan open source dan bukan trial 60 hari. |
| Batas distribusi | Free §2 melarang redistribusi/sublicense/resell software. Operasikan sendiri; jangan bundel binary/image ke paket instalasi sekolah. Model managed/on-prem/resale perlu review kontrak terpisah. |
| Kapasitas/HA | Single compute resource, satu atau lebih drive; tanpa distributed clustering/HA. Batas kapasitas dan masa berlaku harus diperiksa pada license aktual, bukan diasumsikan unlimited/perpetual. Kuota sekolah 5 GB tetap aturan aplikasi. |
| Fitur Free | Dokumentasi lisensi mengecualikan replication, object tiering, encryption at rest, serta fitur enterprise lain. Pricing menyebut full feature set single-node, tetapi daftar pembatasan docs lebih spesifik; validasi build/license sebelum menjanjikan fitur. |
| Keamanan | TLS tetap dapat dikonfigurasi. Tidak mengandalkan server-side encryption MinIO pada Free; rencanakan enkripsi disk/backup di lapisan VPS jika dipilih dan verifikasi terpisah. Backup di luar VPS wajib, bukan replication bawaan. |
| Operasional | Dukungan self-service/komunitas, tanpa SLA. Monitor expiry/capacity selain health endpoint: S3 bisa ditolak walau proses/admin/health masih hidup. |
| Deployment | Dokumentasi container meminta license file Free dari SUBNET, dipasang read-only terpisah dari volume data. Image resmi keluarga `quay.io/minio/aistor/minio`; tag/digest belum dipin. Tidak perlu Kubernetes untuk standalone. |
| Ketergantungan lisensi | Tanpa license, server baru dapat hidup dalam offline mode tetapi semua operasi S3 ditolak. Default expiry berjenjang grace/read-only/offline bergantung claim token; perlu periksa `mc license info` dan uji nyata, bukan memakai lisensi trial diam-diam. |

**Gate sebelum lanjut deployment F1:** operator menerima lisensi Free dan memilih AIStor; operator memperoleh
license melalui jalur resmi dan menyimpannya di secret file lokal (bukan git, chat atau memex). Setelah itu
pin image/digest, validasi fitur/expiry/capacity dengan license aktual, uji S3 put/get/range/deny-anonymous dan
backup/restore. Akses aplikasi pakai kredensial least-privilege, bukan akun root. Lisensi proprietary dan
pembatasan redistribusi perlu ditinjau sebelum distribusi on-prem; evaluasi ini bukan opini hukum.

Sumber resmi yang dibaca:
- [AIStor Free Agreement, updated January 30, 2026](https://www.min.io/legal/aistor-free-agreement) — §1 production/standalone, §2 larangan distribusi/modifikasi.
- [Pricing](https://www.min.io/pricing) — Free single-node, dukungan komunitas, berbeda dari Enterprise trial.
- [Container deployment](https://docs.min.io/aistor/installation/container/install/) — license Free dan volume data/license.
- [License operations](https://docs.min.io/aistor/operations/licenses/) — fitur Free, expiry, capacity dan mode offline; limit aktual mengikuti license.

- **Penyimpanan:** object storage S3-compatible. Dev dan pilot: **MinIO** di compose/VPS yang sama; nanti S3/R2 tanpa
  ubah kode. Tidak disimpan di Postgres atau disk aplikasi.
- **DB usulan:** `files(id, org_id, kind{pdf,audio}, size_bytes, sha256, object_key, uploaded_by, status,
  created_at, deleted_at)`; `status` membedakan pending/ready/deleting/failed. Posisi baca:
  `reader_positions(org_id, user_id, file_id, page, updated_at)`, unique per user/file, diperbarui ter-debounce
  dan dicek terhadap akses file. `library_books.file_id` dan `module_steps.file_id` ditambahkan saat slice
  pemakainya dibuat. EPUB dan upload gambar ditunda; sampul memakai token warna.
- **Upload:** hanya pengajar/admin; PDF, MP3/M4A; maks **50.000.000 byte** per file; total
  **5.000.000.000 byte** per sekolah. UI memakai MB/GB desimal agar cocok dengan batas. Cek tipe dari isi file
  dan validasi format, bukan ekstensi saja. PDF rusak/berpassword ditolak dengan pesan jelas.
- **Kuota aman saat upload serentak:** reservasi ukuran di transaksi Postgres sebelum streaming ke MinIO;
  pending ikut menghabiskan kuota. Setelah selesai, cocokkan byte aktual dan SHA-256; kegagalan/batal melepaskan
  reservasi setelah objek dibersihkan. Jangan tahan transaksi DB selama transfer. Reconciler menangani proses
  mati di tengah upload; kuota tidak bergantung pada hitungan UI atau daftar objek MinIO.
- **Akses baca usulan:** `GET /api/orgs/{org}/files/{id}/content` memeriksa membership dan hak akses konten,
  lalu stream dari MinIO lewat origin aplikasi, termasuk `Range` untuk PDF/audio. Autentikasi mengikuti client
  yang sudah ada; token tidak masuk query URL. Untuk `<audio>`, mekanisme pengiriman autentikasi/range wajib
  dibuktikan di F1 sebelum kontrak dibekukan (elemen native tidak mengatur header Bearer sendiri).
  MinIO tetap privat. Presigned redirect 5 menit dari draf awal belum menjadi kontrak: URL itu bearer access,
  bisa dibagikan selama berlaku dan tidak mengecek ulang membership tiap range request. Alternatif presigned
  hanya dipakai bila trade-off tersebut disetujui.
- **Lifecycle:** hapus file yang masih dirujuk konten aktif ditolak. Pengarsipan buku tidak otomatis menghapus file.
  File terhapus tetap dihitung sampai penghapusan objek berhasil, agar kuota mencerminkan ruang yang masih dipakai.
  Kelola menampilkan pemakaian, upload berjalan, dan kegagalan yang bisa dicoba ulang.
- **Pembaca di aplikasi:**
  - **PDF:** kandidat `pdfjs-dist` (Mozilla PDF.js, dependency baru) untuk `Reader.vue`; navigasi halaman, zoom,
    text layer untuk seleksi/screen reader, dan simpan halaman terakhir per user/file di server. Render halaman
    terlihat saja, batalkan render saat tutup. Worker dan library harus versi sama, di-host lokal dan dimuat lazy.
    Tidak mengandalkan viewer native yang perilaku inline/posisinya berbeda antarbrowser; Safari/iOS tetap harus
    diuji, bukan diasumsikan berfungsi. Dokumen pindai tanpa teks tidak otomatis aksesibel; OCR di luar scope.
  - **EPUB:** menyusul setelah F1 (diputuskan).
  - **Audio:** elemen `<audio>` bawaan.
  - **Materi teks:** markdown pendek dirender aman (tanpa HTML mentah).
- **Unduh:** pembaca tidak menyediakan tombol unduh, tetapi file yang bisa dibaca di browser tetap bisa disalin;
  ini bukan DRM. Hak cipta materi menjadi tanggung jawab sekolah yang mengunggah (dicatat di syarat penggunaan).
- **Operasional:** bucket privat; kredensial aplikasi bukan root MinIO; TLS lewat Caddy, console tidak publik;
  versi image/dependency, lisensi dan pemeliharaan upstream diperiksa sebelum dipin. Backup objek dan metadata
  Postgres ke lokasi di luar VPS yang sama, lalu uji restore. Volume MinIO di VPS bukan backup.
- **Test:** file palsu/rusak/berpassword, batas byte tepat/melebihi, dua upload bersamaan dekat kuota, cancel/crash,
  akses lintas organisasi dan konten draf, range valid/invalid, PDF/audio desktop dan HP. Rate limit upload/baca,
  timeout dan pembatas concurrency. Log hanya ID/ukuran/error, bukan isi file atau URL bertanda tangan.

### L1 — Katalog perpustakaan (baca saja)

Siswa di ruang `library` melihat katalog: cari, tab kategori (sama dengan 8 rak), kartu buku. Klik rak di peta
memilih kategori. Pengajar/admin menambah, mengubah, menghapus buku dari halaman Kelola.

- **DB:** `library_books(id, org_id, title, author, shelf, level, format{buku,ebook,audio}, copies, file_id,
  cover_color, created_by, created_at, published_at, archived_at)`. `shelf` ∈ kunci rak di `rooms.json` (dongeng … referensi).
  `file_id` (F1) wajib untuk ebook/audio, kosong untuk buku fisik.
- **API:** `GET /api/orgs/{org}/library/books?shelf=&q=` (anggota), `POST/PATCH/DELETE` (pengajar/admin).
  Pencarian `ILIKE` judul/penulis cukup untuk ratusan judul; full-text menyusul bila perlu.
- **UI:** panel Katalog menggantikan panel Chat di perpustakaan, dengan tab "Katalog | Chat" agar chat tetap bisa.
  Rak di Phaser jadi bisa diklik (`selectShelf`). Kartu: sampul warna, judul, penulis, chip level + format, status.
  Ebook/Audio → "Baca sekarang"/"Dengarkan" membuka pembaca F1 di dalam aplikasi (overlay di atas peta).
- **Contoh awal:** satu PDF contoh lewat perintah seed eksplisit untuk organisasi uji (lihat bagian akhir),
  idempotent dan berlabel Contoh. Migration hanya skema, tidak memanggil MinIO atau mengisi koleksi sekolah nyata.
- **Test:** service table-driven (validasi, otorisasi lintas org), handler, komponen katalog, uji browser klik rak.

#### Progres L1

- [x] Migration `00008_library_books.sql`: `library_books` dengan `shelf`/`level`/`format`/`cover` ber-CHECK, FK
  komposit `(org_id, file_id)` ke `files` (file wajib milik sekolah yang sama), buku fisik tanpa file, digital
  `copies = 0`, `sample`, draf = `published_at IS NULL`, arsip = `archived_at`. `cover` adalah kunci token
  `--kv-subject-*`, bukan hex.
- [x] `internal/library`: CRUD pengajar/admin (`PUT` ganti semua field, `DELETE` = arsip), daftar urut judul
  dengan cursor, cari `ILIKE` judul/penulis dengan escape `%`/`_`, filter rak, posisi baca pemanggil.
  File harus `ready`, milik sekolah, dan sesuai format (e-book → PDF, audio → MP3/M4A).
- [x] Akses file diperketat: peserta hanya membaca file dari buku **terbit** non-arsip (selain itu 404, tidak
  membocorkan draf); daftar file `/files` khusus staf; file yang dipakai buku aktif tidak bisa dihapus (409
  `file_in_use`).
- [x] Seed eksplisit `make seed-library org=<id>`: PDF 2 halaman CC0 buatan sendiri (`db/seed/`) diunggah lewat
  jalur upload yang sama (kuota, checksum, format) sebagai admin pertama; ID turunan org sehingga idempoten;
  label "Contoh". Bukan migration.
- [x] UI: Kelola punya "Katalog perpustakaan" (tambah/ubah/terbit/draf/arsip, pratinjau). Ruang `library`
  membuka panel kanan "Perpustakaan" dengan tab Katalog | Chat; klik rak atau labelnya di peta memilih rak dan
  menyorotnya; cari ter-debounce, respons usang dibuang, "Muat lagi". Mode HP menampilkan kartu Katalog.
- [x] Verifikasi: integration Postgres+AIStor (peran, lintas org, draf, escape, paginasi, akses file, seed dua
  kali), komponen, browser Chrome (guru membuat draf → peserta tidak melihat dan file 404 → terbit → klik rak
  Sains → cari → baca di overlay; hapus file terpakai 409).
- [ ] Belum: status "Sedang dipinjam"/Pinjam (L2), Safari/iOS, ruang `perpus` lama (map `baca`) tidak mendapat
  katalog sampai dimigrasi ke `library`.

### L2 — Pinjam dan kembalikan

- **DB:** `library_loans(id, org_id, book_id, user_id, borrowed_at, due_at, returned_at)`; indeks unik parsial
  `(book_id, user_id) WHERE returned_at IS NULL`.
- **Aturan:** hanya format `buku`; maks **5** pinjaman aktif per anggota; 14 hari; stok = `copies` − pinjaman aktif,
  dicek dalam transaksi dengan urutan lock tetap: membership peminjam lalu buku. Lock peminjam menjaga batas
  lima saat meminjam buku berbeda bersamaan; lock buku menjaga eksemplar terakhir. Retry pinjam/kembali tidak
  membuat duplikat. Mengurangi `copies` di bawah jumlah pinjaman aktif ditolak. Jatuh tempo ditampilkan WIB.
- **UI:** tombol Pinjam / Kembalikan / "Sedang dipinjam"; bagian **Pinjamanku** (maks 5) dengan jatuh tempo.
- **Ditunda (diputuskan):** antre (reservasi) dan denda.

### M1 — Modul

Modul = jalur belajar per gedung dan level (SD/SMP/SMA/Umum), berisi langkah berurutan.

- **DB:** `modules(id, org_id, building_key, level, title, description, position, published_at, archived_at)`,
  `module_steps(id, org_id, module_id, position, kind{materi,berkas,video}, title, body_md, file_id, url,
  minutes)`. `kind=kuis` dan FK `quiz_id` ditambahkan di Q1. Draf hanya terlihat pengajar/admin; publikasi dan
  arsip mengatur akses peserta. Usul: revisi terbit immutable; edit menghasilkan revisi baru agar progres dan
  hasil lama tidak berubah diam-diam.
- **Konten:** `materi` = markdown pendek; `berkas` = PDF/audio dari F1, dibaca di aplikasi; `video` = URL YouTube
  yang di-embed di aplikasi (`youtube-nocookie.com`, bukan diunggah: video besar di luar kuota F1); `kuis` = merujuk
  kuis Q1.
- **UI peserta:** detail gedung (PlaceCard → "Lihat modul") menampilkan jalur seperti `Room.dc.html`.
- **UI pengajar:** halaman Kelola → Modul (buat, urutkan langkah, terbitkan).

### M2 — Progres modul

- **DB:** `module_progress(org_id, user_id, step_id, completed_at)` PK `(org_id, user_id, step_id)`;
  progres terikat revisi langkah. Endpoint selesai idempotent, urutan divalidasi server.
- **Aturan usulan:** langkah dibuka berurutan; langkah materi/video selesai saat siswa menekan "Selesai";
  langkah kuis selesai saat lulus (nilai ≥ ambang, usul default 60), baru dihubungkan setelah Q1.
- **UI:** "Lanjutkan belajar" di panel belajar kiri memakai progres nyata.

### Q1 — Bank soal dan kuis mandiri

- **DB:** `quizzes(id, org_id, building_key, title, level, pass_score)`,
  `quiz_questions(id, quiz_id, position, text, options text[], answer_index)`,
  `quiz_attempts(id, quiz_id, user_id, started_at, finished_at, score, correct)`,
  `quiz_answers(attempt_id, question_id, chosen, correct, answered_at)`.
- **Keamanan:** `answer_index` tidak pernah dikirim ke client sebelum soal dijawab; server menilai.
- **Nilai vs poin:** nilai Rapor **0–100** (`benar / jumlah soal × 100`); poin lomba terpisah. Rumus
  `+100 + bonus streak` dari prototipe **belum disetujui**, tidak dijadikan aturan otomatis. Soal wajib tepat
  empat opsi dan satu jawaban; attempt menyimpan snapshot versi soal. Satu jawaban per soal, pengiriman ulang
  idempotent, hasil selesai immutable. Pembukaan kunci jawaban saat retry/live perlu aturan eksplisit.
- **UI:** tampilan soal 4 pilihan bentuk-warna (lingkaran/segitiga/kotak/belah ketupat, tidak bergantung warna saja).

### Q2 — Arena Kuis live

- Pengajar membuka sesi kuis di ruang Arena; peserta di ruang yang sama ikut serentak.
- **Realtime:** memakai room actor yang ada: `cmd quiz_start/quiz_next`, server menyiarkan soal tanpa jawaban,
  peserta mengirim `quiz_answer {q, choice}`, server menilai, menyiarkan klasemen tiap soal. Batas waktu per soal
  dari server (bukan jam client).
- **Kapasitas:** batas ruang 40 tetap berlaku; "N/200 pemain" baru tampil setelah load test kapasitas Arena.
- **State usulan:** lobby → question → reveal → finished/canceled. Start/next hanya pengajar/admin di ruang
  tersebut; jawaban terikat sesi, soal, dan peserta aktif; deadline dari clock server. Jawaban terlambat/duplikat,
  reconnect, pengajar pergi, travel/kick, dan restart server punya test. I/O DB di luar goroutine ticker room;
  hasil baru diumumkan tersimpan setelah commit. Restart tidak menghasilkan sesi sukses palsu.
- Hasil disimpan sebagai `quiz_attempts` dengan mode/sesi Arena, jadi masuk Rapor dan leaderboard.
  Identitas sesi masuk kunci idempotensi; leaderboard live per sesi terpisah dari agregat mingguan Redis.

### C1 — Kelas/rombel (cakupan dan pengelolaan disetujui; schema usulan)

**Tujuan:** daftar siswa kelas menjadi sumber kelas pada tooltip dan filter Rapor, bukan mengikuti lokasi avatar.

- **Disetujui:** siswa maksimal satu kelas aktif dalam sekolah dan tahun ajaran yang sama. Akun bisa punya kelas
  berbeda di sekolah lain atau tahun ajaran lain. Perubahan kelas tidak menghapus akun, hasil belajar, atau riwayat.
- **Pengelolaan disetujui:** pengajar/admin membuat kelas (mis. XI IPA 2) pada tahun ajaran (2026/2027), dan
  menempatkan/memindahkan peserta dari anggota sekolah. **Admin** memilih satu tahun ajaran aktif per sekolah;
  tahun lama tetap bisa dipilih di Rapor. Siswa tidak mengubah kelas sendiri. Anggota lama tidak
  diberi kelas otomatis; statusnya "Belum ditempatkan" sampai pengajar/admin menempatkan.
- **Usulan model:** `academic_years` milik organisasi, `classes` terikat organisasi/tahun ajaran, dan
  `class_enrollments` berisi siswa, kelas, waktu masuk/keluar. Constraint unik parsial pada
  `(org_id, academic_year_id, user_id) WHERE ended_at IS NULL` menjamin maksimal satu penempatan aktif, termasuk
  dua permintaan bersamaan. FK komposit memastikan tahun/kelas/anggota berasal dari organisasi yang sama.
- **Pindah kelas:** tutup penempatan lama dan buat penempatan baru dalam satu transaksi; pertahankan riwayat.
  Tahun berikutnya dibuat eksplisit, tidak memindahkan atau menaikkan siswa otomatis. Tooltip memakai tahun
  ajaran aktif pilihan admin, bukan menebak dari kalender sistem. Tanpa penempatan tampil "Belum ditempatkan".
- **Pemakai:** tooltip menampilkan kelas pada tahun ajaran yang dipilih/aktif, bukan kelas dari tahun acak.
  Rapor bisa memfilter anggota kelas; usul awal filter roster saat ini pada tahun terpilih, bukan mengatribusikan
  ulang nilai lama sebagai nilai yang diperoleh dalam kelas baru. Ranking per kelas tetap di luar scope B1.
- **Uji wajib:** penempatan ganda serentak ditolak; siswa yang sama di tahun/sekolah berbeda diizinkan; pindah
  kelas atomik dan riwayat tetap; akses lintas organisasi ditolak; peserta tidak bisa mengubah roster sendiri.

### R1 — Rapor

- **Peserta:** ringkasan per gedung/mapel: modul selesai, nilai kuis terakhir dan terbaik, riwayat Arena.
- **Pengajar/admin:** tabel anggota organisasi, filter kelas/tahun ajaran dari C1 serta gedung/modul/kuis;
  siapa sudah/belum menyelesaikan, rata-rata nilai dan ekspor CSV. Definisi filter historis mengikuti keputusan C1.
- Query agregasi atas M2/Q1; nilai tidak dicampur poin kompetisi. Peserta hanya melihat rapor sendiri;
  pengajar/admin melihat anggota organisasinya. CSV di-escape terhadap formula injection, termasuk nama siswa.

### B1 — Poin dan leaderboard mingguan (Redis)

- **Sumber kebenaran:** Postgres `point_events(id, org_id, user_id, building_key, source, ref_id, points,
  created_at)`; unique `(org_id, source, ref_id, user_id)`. Hasil belajar, event poin dan pekerjaan proyeksi
  dicatat dalam satu transaksi. Aturan penghargaan retry harus mencegah farming (lihat keputusan tersisa).
- **Leaderboard disetujui:** mingguan sekolah dan filter gedung/mapel; per sesi kuis tampil di Q2, progres per
  modul di Rapor. Redis hanya menyimpan user ID/skor, bukan nama lengkap/kelas.
- **Kunci:** `lb:{org_id}:{ISO-year-week-WIB}:all` dan `lb:{org_id}:{ISO-year-week-WIB}:building:{building_key}`.
  Semua timestamp disimpan UTC; batas minggu dihitung `Asia/Jakarta`, termasuk pergantian tahun ISO.
- **Proyeksi usulan (mengoreksi draf `ZINCRBY`):** worker membaca pekerjaan durable dari Postgres, menghitung total
  user pada scope/minggu terkait, lalu `ZADD` **skor absolut**. Satu jalur worker terserialisasi menangani update
  dan rebuild per scope agar write lama tidak menimpa skor baru. Retry setelah timeout/crash menghasilkan skor
  sama; pipeline `ZINCRBY` saja tidak mencegah duplikasi atau beda total sekolah/mapel.
- **Rebuild dan outage:** proyeksi dipulihkan dari Postgres saat Redis restart/eviction atau setelah outage;
  bukan hanya startup/kunci hilang. Rebuild menggunakan kunci sementara dan swap atomik, tidak menampilkan hasil
  setengah jadi. Worker melanjutkan pekerjaan tertunda setelah snapshot selesai. TTL 35 hari dari akhir minggu
  membatasi histori cache; DB tetap sumber riwayat. Kuis tetap jalan saat Redis mati; leaderboard menunjukkan
  status belum tersedia/memperbarui, bukan nol palsu. Detail locking dan recovery diuji sebelum kontrak dibekukan.
- **Baca:** top 10 dan posisi saya; detail identitas diperiksa ulang terhadap membership organisasi.
  Pengguna yang keluar tidak tampil dari cache lama. Usul tie-break deterministik tanpa bonus kecepatan;
  aturan seri masih perlu disepakati.
- **Infrastruktur:** Redis 7 di `compose.yaml` (dev) dan VPS (pilot), tanpa persistensi wajib (bisa dibangun ulang),
  hanya bisa diakses dari jaringan privat, dengan password. Client Go: `github.com/redis/go-redis/v9`.
- **Identitas disetujui:** inisial; tooltip hover berisi nama lengkap dan kelas. Konten yang sama tersedia lewat
  focus keyboard dan tap (bukan `title` saja), bisa ditutup Esc/tap luar. Inisial tidak unik: gunakan user ID
  untuk identitas, nama tetap di-escape. Akses hanya anggota organisasi.
- **Batas privasi:** ini tampilan ringkas, **bukan anonimisasi**; anggota organisasi tetap dapat melihat nama
  lengkap. Jangan mengklaim inisial menyembunyikan identitas. Kelas berasal dari C1; sumber nama lengkap masih
  perlu dikonfirmasi (lihat keputusan tersisa). Jangan menebak kelas dari ruang avatar atau menambah nama legal.

### B2 — Bintang, beruntun, misi, lencana

- **Usulan, belum aturan final:** bintang dari poin; beruntun dari hari aktivitas belajar valid (WIB), bukan
  sekadar login. Misi/lencana memakai aturan statis dan klaim unik; tidak memberi hadiah dari event hadiah lain.
- Panel kiri memakai data nyata; formula level/XP, reset streak dan penghargaan misi disepakati sebelum B2.
  Tanpa formula disetujui, tampilkan progres belajar saja, bukan angka contoh.

## Di luar rencana ini

Kamera/video, editor peta, toko item, antre buku, denda, RAG, upload video, integrasi LMS.

## Kuota yang disetujui

**B: 5 GB per sekolah, maks 50 MB per file.** Ini batas penyimpanan total, bukan jatah per siswa atau jumlah
unduhan. Satu file yang dipakai dua modul tetap satu objek, dihitung sekali. Tidak ada deduplikasi lintas sekolah.
Kelola menampilkan pemakaian dan pesan kuota penuh. Batas tidak menggantikan pemantauan kapasitas disk VPS.

Usulan implementasi: simpan limit byte di pengaturan organisasi dengan default 5 GB, bukan environment variable
sebagai pengaturan dinamis palsu. Pengubahan limit tanpa deploy butuh jalur operator; UI paket langganan ditunda.
Menurunkan limit di bawah pemakaian tidak menghapus file: upload baru ditolak sampai pemakaian turun.

## Keputusan tersisa (tidak menghalangi perencanaan F1)

1. **Nama tooltip.** C1, pengelolaan dan tahun ajaran aktif sudah disetujui. Nama tooltip masih diusulkan
   memakai nama profil yang ada, bukan meminta nama legal baru. Detail schema ditinjau sebelum migration C1.
2. **Aturan belajar.** Usul batas pinjaman 5/14 hari, ambang lulus 60, revisi modul immutable, dan video YouTube
   embedded masih usulan. Putuskan sebelum L2/M1/Q1; persetujuan urutan tidak otomatis menyetujui semuanya.
3. **Poin dan seri.** Usul: nilai akademik terpisah dari poin, tanpa bonus cepat; hanya hasil terbaik per kuis
   per minggu memberi kontribusi (peningkatan menambah selisih), modul mendapat hadiah sekali per revisi.
   Tie-break stabil berdasarkan user ID dengan skor sama ditampilkan sebagai seri. Nilai poin dan hadiah B2
   ditentukan sebelum B1/B2; jangan memakai streak prototipe sebagai keputusan.
4. **Jalur baca file.** Pilih kontrak streaming berautentikasi atau presigned setelah spike audio/range;
   pertimbangkan pencabutan akses, dukungan browser, dan bandwidth Go. Tidak mengubah JWT/cookie auth lama
   diam-diam untuk kebutuhan pembaca.
5. **Retensi data belajar.** Sekolah perlu menetapkan masa simpan hasil/rapor, pinjaman dan file serta penanganan
   keluar organisasi/penghapusan akun sebelum pilot fitur terkait. Backup juga harus mengikuti kebijakan;
   retensi chat 30 hari tidak otomatis dipakai untuk semua data.

## Tugas pelaksanaan dan acceptance criteria

Checklist task utuh masih **belum selesai**; subtask fondasi F1-01 di atas sudah diverifikasi. Target file di bawah adalah permukaan perubahan, bukan file wajib
baru; ikuti pola handler/service/store yang ada. Tiap task dipecah menjadi patch terfokus bila menyentuh >5 file
handwritten; generated OpenAPI/sqlc dicatat terpisah. Setiap slice tetap berakhir sebagai alur vertikal yang bisa
dicoba, bukan menyerahkan backend kosong tanpa UI.

| ID | Task dan acceptance criteria | Dependensi | Permukaan file | Verifikasi |
| --- | --- | --- | --- | --- |
| F1-01 | [ ] Spike PDF/audio privat: PDF 2 halaman dan seek audio bekerja dengan auth/range di desktop dan HP; dokumentasikan keputusan transport | Auth/org existing | `internal/file/`, `src/components/Reader.vue`, `compose.yaml` | Browser + unauthorized/range checks; belum seed sekolah |
| F1-02 | [ ] Upload tervalidasi dan kuota atomik; tepat batas diterima, melebihi/serentak/batal/crash tidak melewati limit atau bocor reservasi | F1-01 | migration `files`/quota, `db/queries/`, `internal/file/` | Unit validasi, integration Postgres + MinIO, failpoint cleanup |
| F1-03 | [ ] Kelola unggah/hapus file, baca PDF/audio, lanjut halaman terakhir; error/retry jelas dan kontrol keyboard/tap bekerja | F1-02 | OpenAPI, API client, Kelola, `Reader.vue`, reader positions | Component + E2E upload/baca/restore posisi/lintas org |
| L1-01 | [x] Pengajar CRUD katalog draf/terbit/arsip; peserta hanya konten terbit, file wajib sesuai format dan org | F1 | migration/books queries, `internal/library/`, Kelola | Unit role, integration FK/org, UI authoring |
| L1-02 | [x] Cari/paginasi/filter kategori dan klik rak membuka katalog; Chat tetap tersedia; contoh hanya dari seed eksplisit | L1-01 | library UI, `CampusScene.ts`, game events, seed | Component + browser klik rak/baca desktop dan HP |
| L2-01 | [ ] Pinjam/kembali dan Pinjamanku; transaksi menahan pinjaman ke-6 dan dua peminjam eksemplar terakhir; retry aman | L1, aturan pinjam disetujui | migration/loans queries, `internal/library/`, library UI | Integration concurrent, UI stok/jatuh tempo/empty |
| M1-01 | [ ] Pengajar membuat draf modul/urutan langkah dan menerbitkan; peserta tidak bisa membaca draf atau file yang hanya terikat draf | F1, aturan revisi disetujui | migration/modules queries, `internal/module/`, Kelola | Unit validasi + role/org/draft integration + UI authoring |
| M1-02 | [ ] Lihat modul per gedung/level dan buka materi di aplikasi tanpa meninggalkan ruang; kosong/error/focus tertangani | M1-01 | PlaceCard, module UI, Reader, tokens | Component + browser materi PDF/audio/teks |
| M2-01 | [ ] Selesai langkah idempotent dan urut di server; reload melanjutkan langkah benar; tombol Lanjutkan memakai data nyata | M1 | migration/progress queries, `internal/module/`, LearningPanel | Unit urutan, integration duplikat/org, E2E reload |
| Q1-01 | [ ] Bank soal draf/terbit tepat 4 opsi; kunci hanya tersedia pengajar, snapshot tidak berubah setelah edit | M1/M2 | migration/quizzes queries, `internal/quiz/`, Kelola | Unit soal invalid + integration version/role + component |
| Q1-02 | [ ] Kuis mandiri menilai di server, submit ulang aman, nilai 0–100 tersimpan; langkah kuis lulus membuka langkah lanjut | Q1-01, aturan nilai disetujui | `internal/quiz/`, module linkage migration, quiz UI | Unit skor, integration submit/crash, E2E nilai/progres |
| C1-01 | [ ] Pengajar/admin membuat kelas pada tahun ajaran; hanya admin memilih tahun aktif; sekolah lain tidak terpengaruh | Q1 dalam urutan, auth/org existing | migrations/year/class queries, fitur kelas, Kelola | Unit role, integration org/tahun aktif serentak, component Kelola |
| C1-02 | [ ] Penempatan/pindah siswa atomik; maksimal satu aktif per sekolah/tahun meski request serentak; riwayat tetap, tahun/sekolah lain boleh berbeda | C1-01 | enrollment queries/service, roster UI | Integration concurrency/transfer/role/org, E2E pindah dan ganti tahun |
| R1-01 | [ ] Peserta melihat rapor sendiri; pengajar memfilter kelas/tahun serta konten dengan CSV aman; hasil lama tidak ditimpa saat pindah kelas | M2/Q1/C1 | report queries, fitur rapor baru, Rapor UI | Integration own/other/org/class/year, CSV injection + browser |
| Q2-01 | [ ] Pengajar menjalankan sesi; deadline/jawaban/reveal authoritative, tanpa DB blocking ticker; state dan recovery tercatat | Q1/R1 | `internal/quiz/`, `internal/realtime/`, protocol mirrors | State-machine + fake clock, reconnect/restart, `go test -race` |
| Q2-02 | [ ] Dua browser ikut kuis sampai hasil; travel/kick/reconnect tidak menggandakan nilai; Arena history muncul di Rapor | Q2-01 | Arena UI, WS client, Rapor query/UI | E2E teacher + 2 peserta, beban 40 dengan target NFR lama |
| B1-01 | [ ] Event/outbox atomik, proyeksi Redis idempotent; retry/crash/rebuild serentak tidak mengubah total; minggu WIB/ISO benar | Q2, aturan poin disetujui | migrations, `internal/leaderboard/`, Redis compose/config | Integration PG+Redis: timeout, eviction, outage, pergantian tahun |
| B1-02 | [ ] Top 10 + posisi saya, filter gedung, inisial dengan hover/focus/tap nama/kelas; nonanggota tidak tampil | B1-01, C1, sumber nama dikonfirmasi | identity source, leaderboard API/UI, LearningPanel | Component a11y + E2E scopes/org/member removed/ties |
| B2-01 | [ ] XP/streak/misi/lencana memakai aturan disetujui dan klaim unik; refresh/retry tak menggandakan hadiah | B1, formula B2 disetujui | achievement queries/service, LearningPanel | Unit WIB/streak, integration reward replay + browser |

### Gate verifikasi per slice

- [ ] RED → GREEN untuk aturan baru; test unit dan integration dengan dependency nyata, bukan mock seluruh sistem.
- [ ] Backend: `make test`, `make lint`; realtime: `go test -race ./internal/realtime`.
- [ ] Frontend: `npm run test:unit -- --run`, `npm run lint`, `npm run build`; coverage logika berubah target ≥80%.
- [ ] `npm run test:world` bila menyentuh map/rak; API/sqlc regenerate dan tidak ada drift.
- [ ] Chrome desktop + mobile viewport; keyboard/screen reader untuk reader/tooltip; Safari iOS nyata dicatat
  terpisah bila perangkat tidak tersedia. PDF/audio/kuis tidak boleh bergantung hanya pada test DOM.
- [ ] Review auth/org boundaries, secrets, dependency audit, migration Up/Down/Up di DB disposable; akun/ruang lama
  tetap berfungsi. Test tidak bergantung pada `kelasverse-assets/` atau kredensial lokal.

### Checkpoint dan rollout

- [ ] **Setelah F1/L1:** upload → katalog → baca bekerja; restore backup, kuota, HP dan chat diverifikasi.
- [ ] **Setelah L2/M1/M2:** alur pinjam dan lanjut belajar nyata; konten draf/arsip aman; review pengguna.
- [ ] **Setelah Q1/C1/R1:** nilai benar, satu kelas aktif terjamin, riwayat pindah/tahun ajaran dan akses rapor
  terisolasi; tak ada angka prototipe.
- [ ] **Setelah Q2/B1:** dua browser sampai hasil, uji 40 peserta, Redis outage/rebuild dan retry lulus.
- [ ] **Setelah B2:** audit hadiah ganda, aksesibilitas, rollback aplikasi dan pilot satu organisasi.

Migration baru saja; nama domain tanpa prefix tahap rilis. Jangan ubah migration yang telah diterapkan.
Pisahkan rollout schema dan deploy aplikasi; rollback aplikasi tidak otomatis menghapus hasil/pinjaman/file.
Existing ruang `baca` tetap didukung; migrasi `baca` ke `library` perlu persetujuan terpisah.
Perubahan `.gitignore`, skills, atau konfigurasi agent yang bukan bagian fitur ini tidak diikutkan.

## Rujukan teknis untuk spike

- [Mozilla PDF.js FAQ](https://github.com/mozilla/pdf.js/wiki/Frequently-Asked-Questions): versi worker/library
  harus sama, render halaman terlihat saja, range bergantung respons server; dukungan Safari tetap perlu diuji.
- [Redis sorted sets](https://redis.io/docs/latest/develop/data-types/sorted-sets/): `ZADD` memperbarui skor
  member, `ZINCRBY` menambah skor; pengiriman ulang increment bukan operasi idempotent.
- [S3 presigned URL](https://docs.aws.amazon.com/AmazonS3/latest/userguide/using-presigned-url.html): URL adalah
  bearer access dan bisa dipakai berulang sampai kedaluwarsa; bukan tautan privat yang terikat pengguna.
- MinIO dan dependency reader/Go: periksa rilis, lisensi, advisori, dan dokumentasi resmi saat F1; belum dipin.

## Contoh katalog (L1)

Satu buku contoh per organisasi, ditandai sebagai contoh agar tidak dikira koleksi asli dan bisa dihapus pengajar:

| Field | Nilai |
| --- | --- |
| title | Contoh: Petualangan di Tata Surya |
| author | Tim Kelasverse |
| shelf | sains |
| level | SD |
| format | ebook |
| copies | 0 (digital) |
| file | PDF contoh 1–2 halaman karya sendiri dengan dedikasi CC0, salinan produksi di `classverse-backend/db/seed/`; perintah seed eksplisit mengunggah ke MinIO, bukan migration atau folder referensi |
| cover_color | `--kv-subject-ipa` |

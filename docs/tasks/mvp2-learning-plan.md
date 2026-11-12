# MVP 2 — Rencana fitur belajar

Status: **draf; sebagian keputusan sudah diambil** (lihat *Keputusan*). Belum ada kode. Keputusan bertanda
**(putuskan)** harus dijawab sebelum slice terkait dimulai, lalu dicatat ke `docs/design.md` (aturan `CLAUDE.md`).

## Keputusan

| Tanggal | Keputusan | Akibat |
| --- | --- | --- |
| 2026-11-11 | Modul dan e-book **dibaca di dalam aplikasi**, bukan tautan keluar | File diunggah dan disimpan di Kelasverse (slice F1), lalu dirender oleh pembaca di aplikasi |
| 2026-11-11 | **Belum perlu RAG** | Tidak ada embedding, vector store, atau LLM; "Tanya pustakawan" membuka chat ke pengajar |
| 2026-11-11 | **Leaderboard memakai Redis** | ADR-011 di `docs/design.md`; Redis ditambahkan ke compose; Postgres tetap sumber kebenaran poin |

Sumber: `kelasverse-assets/README.md`, `reference/Main.dc.html` (header, panel belajar, leaderboard), `Room.dc.html`
(modul berjenjang di gedung), `Quiz.dc.html` (Arena Kuis live), `Library.dc.html` + gambar
`Perpustakaan — rak, katalog & pinjam@2x.png` (katalog dan pinjam).

## Prinsip

- **Data nyata saja.** Angka prototipe (XP 320/500, 64 pemain, 2.110 poin) tidak pernah tampil sebagai data. UI kosong
  diberi teks kosong yang jujur ("Belum ada modul").
- **Per organisasi.** Semua tabel baru punya `org_id`; setiap query memfilter `org_id` dan memeriksa membership
  (aturan keamanan yang sudah ada).
- **Pengajar/admin mengisi, peserta memakai.** Peserta tidak bisa membuat konten.
- **Vertical slice.** Setiap slice = migration + sqlc + service + handler + OpenAPI + UI + test + uji browser.
- **Poin dihitung server.** Client tidak pernah mengirim skor; server menilai jawaban.
- **Spec MVP 1 menyebut gamifikasi dan kuis "out of scope".** Revamp MVP 2 sudah disetujui melampaui itu; `docs/spec.md`
  perlu bagian MVP 2 baru sebelum slice L1 dikerjakan.

## Urutan dan ketergantungan

```
F1 Penyimpanan file + pembaca di aplikasi
 │
 ├──► L1 Katalog perpustakaan ──► L2 Pinjam/kembalikan
 │
 └──► M1 Modul + langkah (materi/berkas/video) ──► M2 Progres modul
                                              │
Q1 Bank soal + kuis mandiri ──► Q2 Arena Kuis live (realtime) ──┐
                                              │                 │
                                              ▼                 ▼
                                     R1 Rapor (progres + nilai)  B1 Poin + leaderboard mingguan
                                                                 │
                                                                 ▼
                                                       B2 Bintang, beruntun, misi, lencana (panel belajar)
```

Urutan kerja yang diusulkan: **F1 → L1 → L2 → M1 → M2 → Q1 → R1 → Q2 → B1 → B2.** Penyimpanan file lebih dulu
karena katalog e-book dan modul sama-sama dibaca di aplikasi. Perpustakaan menyusul karena ruangnya sudah ada. Arena live (Q2) paling berisiko (realtime + kapasitas) sehingga
didahului kuis mandiri (Q1) yang memakai bank soal yang sama.

## Berkas: dibaca di aplikasi

Diputuskan: e-book dan materi modul dibuka di dalam Kelasverse. RAG tidak dipakai sekarang; bila nanti diperlukan,
RAG memakai berkas yang sama (dokumen tetap harus tersimpan dulu), jadi F1 tidak sia-sia.

## Slices

### F1 — Penyimpanan file dan pembaca di aplikasi

Fondasi untuk L1 (e-book/audio) dan M1 (materi berkas).

- **Penyimpanan:** object storage S3-compatible. Dev dan pilot: **MinIO** di compose/VPS yang sama; nanti S3/R2 tanpa
  ubah kode. Tidak disimpan di Postgres atau disk aplikasi. **(putuskan)** MinIO di VPS pilot oke, atau langsung
  layanan terkelola (Cloudflare R2)?
- **DB:** `files(id, org_id, kind{pdf,epub,audio,image}, size_bytes, sha256, object_key, uploaded_by, created_at,
  deleted_at)`. Objek dirujuk dari `library_books.file_id` dan `module_steps.file_id`.
- **Upload:** hanya pengajar/admin; PDF, EPUB, MP3/M4A, PNG/JPEG; maks 50 MB (gambar 5 MB); tipe dicek dari
  *magic bytes*, bukan nama file; kuota per organisasi (usul 2 GB pilot). Unggah lewat server Go (streaming ke
  storage, hitung SHA-256), sehingga validasi tidak bisa dilewati. URL presigned PUT langsung ke storage bisa
  menyusul bila server jadi bottleneck.
- **Baca:** `GET /api/files/{id}` cek membership organisasi, lalu redirect ke URL presigned berumur 5 menit dengan
  `Content-Disposition: inline`. Tidak ada URL publik permanen.
- **Pembaca di aplikasi:**
  - **PDF:** `pdfjs-dist` (Mozilla pdf.js, dependency baru) dirender ke canvas di komponen `Reader.vue`;
    navigasi halaman, zoom, simpan halaman terakhir. Iframe viewer bawaan browser tidak dipakai karena tidak jalan
    di Safari iOS dan tidak bisa menyimpan posisi.
  - **EPUB:** **(putuskan)** dukung EPUB sekarang (butuh `epubjs`) atau PDF saja dulu? Usul: PDF saja dulu.
  - **Audio:** elemen `<audio>` bawaan.
  - **Materi teks:** markdown pendek dirender aman (tanpa HTML mentah).
- **Unduh:** pembaca tidak menyediakan tombol unduh, tetapi file yang bisa dibaca di browser tetap bisa disalin;
  ini bukan DRM. Hak cipta materi menjadi tanggung jawab sekolah yang mengunggah (dicatat di syarat penggunaan).
- **Backup:** bucket MinIO ikut jadwal backup VPS; penghapusan file = soft delete lalu objek dihapus oleh job.
- **Test:** validasi tipe/ukuran (file palsu ber-ekstensi `.pdf` ditolak), akses lintas organisasi ditolak, URL
  kedaluwarsa, uji browser membuka PDF contoh.

### L1 — Katalog perpustakaan (baca saja)

Siswa di ruang `library` melihat katalog: cari, tab kategori (sama dengan 8 rak), kartu buku. Klik rak di peta
memilih kategori. Pengajar/admin menambah, mengubah, menghapus buku dari halaman Kelola.

- **DB:** `library_books(id, org_id, title, author, shelf, level, format{buku,ebook,audio}, copies, file_id,
  cover_color, created_by, created_at, archived_at)`. `shelf` ∈ kunci rak di `rooms.json` (dongeng … referensi).
  `file_id` (F1) wajib untuk ebook/audio, kosong untuk buku fisik.
- **API:** `GET /api/orgs/{org}/library/books?shelf=&q=` (anggota), `POST/PATCH/DELETE` (pengajar/admin).
  Pencarian `ILIKE` judul/penulis cukup untuk ratusan judul; full-text menyusul bila perlu.
- **UI:** panel Katalog menggantikan panel Chat di perpustakaan, dengan tab "Katalog | Chat" agar chat tetap bisa.
  Rak di Phaser jadi bisa diklik (`selectShelf`). Kartu: sampul warna, judul, penulis, chip level + format, status.
  Ebook/Audio → "Baca sekarang"/"Dengarkan" membuka pembaca F1 di dalam aplikasi (overlay di atas peta).
- **Contoh awal:** satu buku contoh disisipkan per organisasi oleh migration (lihat bagian akhir).
- **Test:** service table-driven (validasi, otorisasi lintas org), handler, komponen katalog, uji browser klik rak.

### L2 — Pinjam dan kembalikan

- **DB:** `library_loans(id, org_id, book_id, user_id, borrowed_at, due_at, returned_at)`; indeks unik parsial
  `(book_id, user_id) WHERE returned_at IS NULL`.
- **Aturan:** hanya format `buku`; maks **5** pinjaman aktif per anggota; 14 hari; stok = `copies` − pinjaman aktif,
  dicek dalam transaksi `SELECT … FOR UPDATE` agar dua siswa tidak meminjam eksemplar terakhir bersamaan.
- **UI:** tombol Pinjam / Kembalikan / "Sedang dipinjam"; bagian **Pinjamanku** (maks 5) dengan jatuh tempo.
- **Ditunda:** antre (reservasi) dan denda. **(putuskan)** antre perlu di pilot?

### M1 — Modul

Modul = jalur belajar per gedung dan level (SD/SMP/SMA/Umum), berisi langkah berurutan.

- **DB:** `modules(id, org_id, building_key, level, title, description, position, published_at, archived_at)`,
  `module_steps(id, module_id, position, kind{materi,berkas,video,kuis}, title, body_md, file_id, url, quiz_id,
  minutes)`.
- **Konten:** `materi` = markdown pendek; `berkas` = PDF/audio dari F1, dibaca di aplikasi; `video` = URL YouTube
  yang di-embed di aplikasi (`youtube-nocookie.com`, bukan diunggah: video besar di luar kuota F1); `kuis` = merujuk
  kuis Q1.
- **UI peserta:** detail gedung (PlaceCard → "Lihat modul") menampilkan jalur seperti `Room.dc.html`.
- **UI pengajar:** halaman Kelola → Modul (buat, urutkan langkah, terbitkan).

### M2 — Progres modul

- **DB:** `module_progress(user_id, step_id, completed_at)` PK `(user_id, step_id)`.
- **Aturan:** langkah dibuka berurutan; langkah materi/video selesai saat siswa menekan "Selesai"; langkah kuis
  selesai saat lulus (nilai ≥ ambang, default 60).
- **UI:** "Lanjutkan belajar" di panel belajar kiri memakai progres nyata.

### Q1 — Bank soal dan kuis mandiri

- **DB:** `quizzes(id, org_id, building_key, title, level, pass_score)`,
  `quiz_questions(id, quiz_id, position, text, options text[], answer_index)`,
  `quiz_attempts(id, quiz_id, user_id, started_at, finished_at, score, correct)`,
  `quiz_answers(attempt_id, question_id, chosen, correct, answered_at)`.
- **Keamanan:** `answer_index` tidak pernah dikirim ke client sebelum soal dijawab; server menilai.
- **Skor:** benar +100, bonus beruntun +20 × streak (sesuai `Quiz.dc.html`).
- **UI:** tampilan soal 4 pilihan bentuk-warna (lingkaran/segitiga/kotak/belah ketupat, tidak bergantung warna saja).

### Q2 — Arena Kuis live

- Pengajar membuka sesi kuis di ruang Arena; peserta di ruang yang sama ikut serentak.
- **Realtime:** memakai room actor yang ada: `cmd quiz_start/quiz_next`, server menyiarkan soal tanpa jawaban,
  peserta mengirim `quiz_answer {q, choice}`, server menilai, menyiarkan klasemen tiap soal. Batas waktu per soal
  dari server (bukan jam client).
- **Kapasitas:** batas ruang 40 tetap berlaku; "N/200 pemain" baru tampil setelah load test kapasitas Arena.
- Hasil disimpan sebagai `quiz_attempts` biasa, jadi masuk Rapor dan leaderboard.

### R1 — Rapor

- **Peserta:** ringkasan per gedung/mapel: modul selesai, nilai kuis terakhir dan terbaik, riwayat Arena.
- **Pengajar/admin:** tabel per kelas/ruang: siapa sudah/belum menyelesaikan modul, rata-rata nilai, ekspor CSV.
- Tidak ada tabel baru; query agregasi atas M2/Q1. Hanya pengajar/admin organisasi yang melihat data siswa lain.

### B1 — Poin dan leaderboard mingguan (Redis)

- **Sumber kebenaran:** Postgres `point_events(id, org_id, user_id, source{quiz,module,mission}, ref_id, points,
  created_at)`, append-only, indeks unik `(source, ref_id, user_id)` sehingga poin tidak bisa dobel.
- **Leaderboard:** Redis sorted set per organisasi per minggu: `lb:{org_id}:{tahun-minggu ISO, WIB}`.
  - Tulis: setelah `point_events` ter-commit, `ZINCRBY` skor user; `EXPIRE` 35 hari supaya minggu lama terhapus
    sendiri. Reset Senin 00:00 WIB terjadi karena kunci minggu berganti, tanpa job.
  - Baca: `ZREVRANGE … 0 9 WITHSCORES` (top 10) + `ZREVRANK`/`ZSCORE` untuk posisi saya; nama diambil dari
    Postgres untuk 10 id itu.
  - **Konsistensi:** bila `ZINCRBY` gagal setelah commit, poin tidak hilang (Postgres). Job rebuild membangun ulang
    sorted set minggu berjalan dari `point_events` saat startup dan bila kunci tidak ada; perintah admin
    `make leaderboard-rebuild`.
  - **Redis mati:** leaderboard menampilkan "Peringkat sementara tidak tersedia"; kuis dan poin tetap jalan.
- **Infrastruktur:** Redis 7 di `compose.yaml` (dev) dan VPS (pilot), tanpa persistensi wajib (bisa dibangun ulang),
  hanya bisa diakses dari jaringan privat, dengan password. Client Go: `github.com/redis/go-redis/v9`.
- **Privasi (putuskan):** peserta < 18 tahun: nama tampilan lengkap atau inisial? Opsi sekolah mematikan leaderboard?

### B2 — Bintang, beruntun, misi, lencana

- Bintang = poin yang tampil di header; beruntun = hari berturut-turut ada `point_events` (WIB).
- Misi harian dan lencana: aturan statis di kode dulu (mis. "selesaikan 1 kuis"), bukan editor.
- Panel belajar kiri menampilkan Level/XP, bintang, beruntun, lencana, Misi hari ini dari data ini.

## Di luar rencana ini

Kamera/video, editor peta, toko item, antre buku, denda, RAG, upload video, integrasi LMS.

## Pertanyaan untuk disetujui

1. Urutan **F1 → L1 → L2 → M1 → M2 → Q1 → R1 → Q2 → B1 → B2**?
2. Penyimpanan file: **MinIO** di VPS pilot, atau layanan terkelola (Cloudflare R2)?
3. Format yang dibaca di aplikasi: **PDF + audio dulu**, EPUB menyusul?
4. Kuota per sekolah **2 GB** dan batas **50 MB** per file cukup?
5. Antre buku ditunda?
6. Leaderboard untuk siswa < 18 tahun: nama lengkap atau inisial, dan bisa dimatikan per sekolah?

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
| file | PDF contoh 1–2 halaman buatan sendiri (bebas hak cipta) di `classverse-backend/db/seed/`, diunggah ke storage saat seed; dibuka di pembaca aplikasi |
| cover_color | `--kv-subject-ipa` |

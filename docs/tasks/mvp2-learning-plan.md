# MVP 2 — Rencana fitur belajar

Status: **draf untuk disetujui.** Belum ada kode. Keputusan bertanda **(putuskan)** harus dijawab sebelum slice
terkait dimulai, lalu dicatat ke `docs/design.md` (aturan `CLAUDE.md`).

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
L0 Penyimpanan file (opsional, lihat keputusan berkas)
 │
L1 Katalog perpustakaan (baca saja) ──► L2 Pinjam/kembalikan
 │
M1 Modul + langkah (materi/video/tautan) ──► M2 Progres modul
                                              │
Q1 Bank soal + kuis mandiri ──► Q2 Arena Kuis live (realtime) ──┐
                                              │                 │
                                              ▼                 ▼
                                     R1 Rapor (progres + nilai)  B1 Poin + leaderboard mingguan
                                                                 │
                                                                 ▼
                                                       B2 Bintang, beruntun, misi, lencana (panel belajar)
```

Urutan kerja yang diusulkan: **L1 → L2 → M1 → M2 → Q1 → R1 → Q2 → B1 → B2.** Perpustakaan lebih dulu karena
ruangnya sudah ada dan paling kecil risikonya. Arena live (Q2) paling berisiko (realtime + kapasitas) sehingga
didahului kuis mandiri (Q1) yang memakai bank soal yang sama.

## Keputusan berkas: upload ke server atau RAG?

Keduanya menjawab kebutuhan yang berbeda, jadi bukan pilihan salah satu:

| | Menyimpan file (upload) | RAG |
| --- | --- | --- |
| Gunanya | Siswa **membuka** e-book/audio/PDF | **Menjawab pertanyaan** dari isi dokumen ("Tanya pustakawan") |
| Butuh | Penyimpanan file | Tetap butuh file aslinya, plus ekstraksi teks, embedding, vector store (pgvector), dan LLM |
| Biaya/risiko | Disk + bandwidth; validasi tipe & ukuran; malware | Biaya API LLM per pertanyaan; jawaban bisa salah; data siswa ke pihak ketiga (UU PDP, peserta < 18 th) |

RAG tidak menggantikan penyimpanan: untuk dijawab, dokumen tetap harus diunggah dan disimpan. Jadi urutan wajarnya
**simpan dulu, RAG belakangan**.

**Usulan bertahap:**

1. **Sekarang (L1/M1): tautan saja.** Buku/modul digital menyimpan `url` (Google Drive, YouTube, situs penerbit).
   Tanpa upload, tanpa biaya penyimpanan. Validasi: hanya `https://`, dibuka di tab baru dengan `rel="noopener"`.
2. **Berikutnya (L0): upload sendiri** bila sekolah tidak mau bergantung pada Drive. Disimpan di **object storage
   S3-compatible** (MinIO di VPS pilot, nanti S3/R2), bukan di kolom DB atau disk aplikasi. Unduh lewat URL bertanda
   tangan berumur pendek setelah cek membership. Batas: PDF/EPUB/MP3/M4A, maks 50 MB, cek magic bytes, kuota per
   organisasi. Butuh migration + service `storage` + backup.
3. **Nanti (bila diminta): RAG "Tanya pustakawan"**, hanya atas dokumen yang sudah diunggah sekolah. Butuh keputusan
   penyedia LLM, persetujuan data, filter jawaban, dan batas biaya. Sampai saat itu tombol "Tanya pustakawan"
   cukup membuka chat ke pengajar/pustakawan yang sedang di ruang.

**(putuskan)** Setuju mulai dari tautan saja (langkah 1)?

## Slices

### L1 — Katalog perpustakaan (baca saja)

Siswa di ruang `library` melihat katalog: cari, tab kategori (sama dengan 8 rak), kartu buku. Klik rak di peta
memilih kategori. Pengajar/admin menambah, mengubah, menghapus buku dari halaman Kelola.

- **DB:** `library_books(id, org_id, title, author, shelf, level, format{buku,ebook,audio}, copies, url, cover_color,
  created_by, created_at, archived_at)`. `shelf` ∈ kunci rak di `rooms.json` (dongeng … referensi). `url` wajib untuk
  ebook/audio, kosong untuk buku fisik.
- **API:** `GET /api/orgs/{org}/library/books?shelf=&q=` (anggota), `POST/PATCH/DELETE` (pengajar/admin).
  Pencarian `ILIKE` judul/penulis cukup untuk ratusan judul; full-text menyusul bila perlu.
- **UI:** panel Katalog menggantikan panel Chat di perpustakaan, dengan tab "Katalog | Chat" agar chat tetap bisa.
  Rak di Phaser jadi bisa diklik (`selectShelf`). Kartu: sampul warna, judul, penulis, chip level + format, status.
  Ebook/Audio → tombol "Baca sekarang"/"Dengarkan" membuka `url`.
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
  `module_steps(id, module_id, position, kind{materi,video,tautan,kuis}, title, body_md, url, quiz_id, minutes)`.
- **Konten:** `materi` = markdown pendek (dirender aman, tanpa HTML mentah); `video`/`tautan` = `url`;
  `kuis` = merujuk kuis Q1 (diisi setelah Q1).
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

### B1 — Poin dan leaderboard mingguan

- **DB:** `point_events(id, org_id, user_id, source{quiz,module,mission}, ref_id, points, created_at)`; append-only,
  satu event per sumber (indeks unik `(source, ref_id, user_id)`) agar tidak bisa dobel.
- **Leaderboard:** `SUM(points)` sejak Senin 00:00 **WIB** per organisasi, top 10 + posisi saya. Reset otomatis
  karena hanya rentang waktu.
- **Privasi (putuskan):** peserta < 18 tahun: tampilkan nama tampilan lengkap atau inisial saja? Opsi
  organisasi untuk menyembunyikan leaderboard?

### B2 — Bintang, beruntun, misi, lencana

- Bintang = poin yang tampil di header; beruntun = hari berturut-turut ada `point_events` (WIB).
- Misi harian dan lencana: aturan statis di kode dulu (mis. "selesaikan 1 kuis"), bukan editor.
- Panel belajar kiri menampilkan Level/XP, bintang, beruntun, lencana, Misi hari ini dari data ini.

## Di luar rencana ini

Kamera/video, editor peta, toko item, antre buku, denda, RAG (sampai diputuskan), integrasi LMS.

## Pertanyaan untuk disetujui

1. Urutan **L1 → L2 → M1 → M2 → Q1 → R1 → Q2 → B1 → B2**?
2. E-book/audio/modul digital mulai dengan **tautan saja**; upload (object storage) menyusul; RAG hanya bila nanti
   diminta?
3. Antre buku ditunda?
4. Leaderboard untuk siswa di bawah 18 tahun: nama lengkap atau inisial, dan bisa dimatikan per sekolah?

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
| url | kosong sampai pengajar mengisi tautan; tombol "Baca sekarang" nonaktif dengan teks "Tautan belum diisi" |
| cover_color | `--kv-subject-ipa` |

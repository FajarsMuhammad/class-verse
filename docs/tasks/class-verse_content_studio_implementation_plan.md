# CS — Content Studio dan Quiz Builder Kelasverse

Status: CS0–CS4 terimplementasi lokal di branch `content-studio` 2026-11-18; belum commit/push/deploy. Upgrade resmi: `make migrate` (`scripts/migrate.sh`: goose ≤20, preflight, lalu 21).

## Keputusan dan sumber kebenaran

Dokumen ini menggantikan rancangan generik Open Learning World versi 1.0 pada path ini.
Repository Kelasverse memakai modul/langkah, kuis mandiri, tugas, membership organisasi, dan Rapor;
bukan course/lesson/enrollment baru. Tidak perlu baseline PRD/SAD/ERD dari proyek lain.

Persetujuan pengguna: editor rich text tetap dipakai; modul terbit beralih dari edit langsung ke
working draft + revisi terbit immutable. Hasil belajar lama tidak boleh dihapus atau dihitung ulang.
Keputusan ini menggantikan aturan edit-in-place M1/M2 tanggal 2026-11-13, bukan aturan Q1 lainnya.

Sumber kebenaran lain: `docs/spec.md`, `docs/design.md`, `docs/visual.md`,
`docs/tasks/mvp2-learning-plan.md`. Endpoint/field/error baru tetap English dan mengikuti kontrak
`/api` + `snake_case` existing; bukan `/api/v1` dan bukan response envelope kedua.

## Tujuan dan acceptance observabel

1. Guru/admin membuat atau mengedit modul melalui editor rich text terstruktur; siswa tetap memakai
   modul → langkah → isi. Template Artikel, Langkah demi langkah, Tutorial kode mengisi struktur awal,
   bukan tiga backend/editor terpisah.
2. Menyimpan draft, retry, dan konflik dua tab tidak menimpa data diam-diam. Autosave memiliki status
   belum tersimpan/menyimpan/tersimpan/gagal; tombol Simpan eksplisit dan guard navigasi tetap ada.
3. Preview materi memakai renderer siswa; simulasi kuis tidak membuat attempt/progres/nilai nyata.
4. Publish tervalidasi menghasilkan snapshot immutable. Mengedit draft tidak mengubah konten siswa.
5. Siswa melanjutkan revisi yang sudah dipin; siswa baru mendapat revisi terbit terbaru. Progres selesai,
   hasil kuis, tugas, Rapor, file, dan sematan lama tetap sah.
6. Kuis tetap bisa mandiri atau dipakai ulang dalam beberapa modul. Publikasi modul mengikat revisi kuis
   terbit tertentu; tidak boleh otomatis menerbitkan draft kuis bersama milik modul lain.
7. Otorisasi organisasi, status, peran, dan akses file tetap berlaku pada setiap request termasuk versi lama.
   Tamu G1 tidak mendapat akses belajar melalui endpoint baru.
8. Tes unit/komponen, PostgreSQL concurrency/migrasi, dan E2E author–learner desktop/HP membuktikan alur.

## Pertahankan dan ubah

| Existing | Keputusan |
| --- | --- |
| `modules`, `module_steps` | Parent dan ID langkah stabil dipertahankan; tambahkan working version dan revisi |
| Langkah materi/berkas/video/kuis | Semua tetap ada; rich text menambah isi materi, bukan menghapus format lama |
| Modul per gedung/jenjang | Tetap; jangan tambah course/enrollment |
| `quiz_attempts.questions` snapshot | Tetap sumber grading attempt; tidak membaca draft saat submit |
| Start/resume, answer, finish idempotent | Pakai mesin existing dengan binding revisi yang benar |
| Tugas dengan snapshot langkah | Pertahankan konteks dan snapshot tugas; jangan rebinding ke katalog terbaru |
| Progres dan Rapor | Tidak reset, tidak regrade; referensi historis dipertahankan |
| Sematan materi ruang | Tetap bekerja; sematan bukan bypass versi/otorisasi |
| File privat dan YouTube reader | Reuse; revisi wajib mempertahankan referensi file yang diperlukan |
| UI authoring daftar → item → form | Reuse shell, token, kontrol `--touch-min`; tidak wajib wizard lima layar |

## Kebijakan kuis yang tetap berlaku

- Nilai lulus default **60**, rentang 1–100; percobaan **tanpa batas**; Rapor nilai tertinggi.
- Kunci/penjelasan baru tampil setelah seluruh attempt selesai; tidak berubah menjadi selalu tersembunyi.
- Nilai lama tetap `floor(benar × 100 / jumlah soal)`, bobot semua soal sama, kosong salah.
- Pilihan tunggal empat opsi existing tetap sah. Tambah benar/salah dengan tepat dua opsi; satu jawaban benar.
- Batas soal existing 50 tetap sampai ada kebutuhan terukur. Tidak otomatis menjadi 100.
- Tidak ada timer, bobot berbeda, pembatasan attempt, atau aturan Arena live baru dalam CS.
- DTO author mengandung kunci; DTO learner sebelum finish dan konten modul tidak mengandung kunci.

## Bentuk konten dan editor

- Modul berisi langkah; langkah materi berisi dokumen rich text/schema berversi dengan node ID stabil.
- Mendukung paragraf, bold/italic, heading 2–4, ordered/bullet list, link aman, code block berbahasa,
  callout dan checklist latihan manual. Struktur step tetap pada `module_steps`, bukan nested STEP tak terbatas.
- Template Artikel menyiapkan langkah artikel; Langkah demi langkah menyiapkan beberapa langkah;
  Tutorial kode menyiapkan penjelasan, kode, dan latihan. Konten awal tidak berpura-pura menjadi materi nyata.
- Tambah/hapus/duplikasi/urut blok melalui editor; keyboard dan tombol naik/turun tersedia, drag opsional.
- Reader dan preview memakai renderer yang sama. Jangan simpan/render HTML bebas atau menjalankan kode.
- Backend memvalidasi allowlist node/mark/attribute, nesting, jumlah node, URL scheme dan ukuran payload.
  Frontend render secara aman; link `javascript:`, event handler, iframe bebas ditolak.
- Gunakan dependency editor terpelihara bila tersedia: browser `contenteditable` mentah bukan editor rich text
  yang andal untuk paste/history/selection. Evaluasi integrasi Vue, lisensi, schema, dan bundle sebelum memilih.

## Revisi, draft dan publikasi

- Satu working draft per modul/kuis dengan optimistic counter yang diperiksa atomik setiap write.
  Pakai satu kontrak `expected_version` konsisten (atau If-Match bila sudah menjadi seam existing), jangan keduanya.
- `draft_version` berbeda dari nomor revisi publik. Konflik memberi error terstruktur dan opsi muat versi server
  atau salin perubahan lokal; tidak last-write-wins. Respons sukses saja yang menandai tersimpan.
- Publish memakai versi draft yang diharapkan, validasi lengkap dan transaksi. Retry publish versi yang sama
  tidak membuat revisi akademik ganda. Snapshot dan binding tidak dapat diubah setelah terbit.
- Revisi modul berisi metadata/langkah/dokumen dan binding quiz ID → published quiz revision ID.
  Revisi kuis berisi metadata/soal/kunci internal; learner serializer tidak menyalin snapshot rahasia.
- Publish modul tidak memublikasikan draft kuis. Kuis belum terbit/terarsip/referensi salah ditolak dengan field error.
- Daftar riwayat revisi author read-only; restore/branching bukan cakupan awal.
- Pin dibuat hanya oleh aktivitas belajar (tandai selesai, mulai/lulus kuis), bukan saat membuka daftar. Siswa yang belum mulai selalu membaca revisi terbit terbaru; progres in-progress dan completed mempertahankan pin.
  Publish baru tidak menghapus completion. Revisi tidak boleh dibaca lewat ID bebas lintas parent/org.
- Quiz di modul memakai binding dari pin modul server-side, bukan revision ID bebas dari browser.
  Kuis mandiri baru memakai revisi kuis terbit terkini; attempt terbuka tetap snapshot awal.
- Tugas memakai snapshot/revisi issuance sendiri; percobaan tugas tidak dihitung sebagai katalog dan sebaliknya.

## Kompatibilitas dan migrasi

- Migration berikutnya mulai `00020`; migration 00001–00019 yang sudah applied tidak diedit.
- Expand additive: tabel snapshot/pointer, working version, rich content nullable, revision references dan index.
- Backfill konten lama sebagai rich-text paragraf plain text, dengan line breaks tetap terbaca; tanpa parsing HTML lama.
- Backfill modul/kuis terbit menjadi revisi awal. Pin progres lama ke revisi awal dengan mapping ID langkah stabil.
  Hasil attempt lama tetap menggunakan snapshot dan nilai yang sudah ada. Data tanpa aktivitas tidak diberi progres palsu.
- Audit seluruh reader/writer: module detail/list/progress/resume, quiz start/finish, assignment issuance/reader,
  gradebook, file authorization/retention, learning pin, author preview. Tidak membuat API kedua yang melewati invariant.
- FK komposit parent/org dan revision binding harus benar; snapshot historis tidak ikut terhapus saat draft diarsip.
- Rollback harus fail-safe bila data versi baru tidak dapat direpresentasikan versi lama; tidak menjatuhkan tabel
  berisi hasil belajar. Uji Up/Down/Up pada DB disposable dengan dan tanpa data; production rollback memakai backup
  dan runbook kompatibilitas, bukan penghapusan diam-diam.

## UI

- Shell authoring dengan judul, status draft/save, Preview, Review & Publish. Outline langkah dan editor utama;
  pengaturan kontekstual tidak harus panel permanen tiga kolom. Responsif HP.
- Publish menampilkan error spesifik dan fokus field; konfirmasi revisi, tunggu respons server sebelum mengklaim terbit.
- Preview banner **Pratinjau — belum diterbitkan**. Simulasi kuis berlabel **Hasil tidak disimpan**.
- Reader semantik, heading hierarkis, navigasi langkah, tombol salin kode, kontrol progres dengan label teks.
- Semua warna/spacing/radius dari token. Focus visible, keyboard-only, label field, live status dan error announcement.

## Slice implementasi dan exit gate

### CS0 — Kontrak dan pemetaan (sebelum kode fitur)
- [x] Adaptasi rencana ke domain/aturan Kelasverse dan persetujuan immutable revisions + rich text.
- [x] Catat ADR pengganti edit-in-place, model versi serta mapping akademik/file.
- [x] Pilih editor rich text berdasarkan dependency/official docs, bukan implementasi DOM ad hoc.

### CS1 — Draft/revisi end-to-end
- [x] Migration/backfill + optimistic write + publish immutable untuk modul dan kuis.
- [x] Reader/progres/start attempt mengikuti pin dan binding revisi; tugas/Rapor/sematan tetap kompatibel.
- [x] Author UI edit draft/publish/conflict dan reader tidak berubah saat draft disimpan.
- [x] Tes real-Postgres publish bersamaan/edit stale, snapshot/history/data lama dan isolasi akses.

### CS2 — Rich text dan autosave
- [x] Structured rich text schema, validator/render aman dan legacy compatibility.
- [x] Tiga template awal, blok/urutan/duplikasi, renderer siswa bersama preview.
- [x] Autosave/retry/conflict/guard dan keyboard/HP.
- [x] Tes malicious paste/link/node/payload, autosave network failure dan dua tab.

### CS3 — Builder dan simulasi kuis
- [x] Pilihan tunggal existing + benar/salah, validasi server, preview interaktif tanpa attempt nyata.
- [x] Reuse QuizPlayer presentation tanpa memakai start/answer/finish akademik pada preview.
- [x] Uji answer-key isolation sebelum finish, nilai lama, resume/idempotent finish serta konteks tugas/modul/mandiri.

### CS4 — Verifikasi akhir
- [x] Backend race tests/Postgres/migration/lint; frontend unit/typecheck/lint/build.
- [x] E2E author membuat → rich text → kuis → preview → publish → siswa belajar → hasil.
- [x] E2E edit draft saat siswa belajar, publish revisi baru, siswa lama tetap pin, siswa baru mendapat revisi baru.
- [x] Desktop/HP, keyboard, error/retry/conflict, file lama, tugas/Rapor dan sematan.
- [x] Dokumen/API/status sesuai bukti; commit/push tidak dilakukan.

### Bukti dan celah

- Migration `00020` dipulihkan persis ke isi saat applied (SHA256 `e33ce63d…1334`). Perbaikan forward `00021_content_studio_hardening.sql`; preflight `db/upgrades/content_studio_preflight.sql` membuat revisi kuis historis untuk semua referensi kuis di modul dengan progres legacy, tanpa publish/regrade/delete. `make migrate` adalah jalur resmi; `goose up` langsung tidak didukung untuk 20→21.
- Tes dbtest: backfill literal, pin, attempt lama, rollback fail-closed, fresh19→20→preflight→21, existing20→21 dan retry preflight setelah 21. Lokal runtime: 20→21 sukses; preflight retry no-op.
- E2E `/tmp/kv-e2e/content-studio.mjs`: rich text, preview, publish, isolasi draft, pin lama/baru, HP, autosave offline retry, konflik dua tab, simulasi benar/salah tanpa attempt, binding kuis, reveal setelah finish. Regresi `g1-guest.mjs` lulus.
- Celah: belum ada outline heading; autosave hanya isi langkah (metadata modul/kuis memakai save eksplisit + konflik); riwayat menampilkan metadata, bukan isi revisi lama; tombol jadikan draf diganti terbitkan revisi baru; bundle LibraryManager >500 kB. Rapor/tugas UI tidak diuji E2E browser khusus, hanya tes backend.

## Bukan cakupan

Course/enrollment baru, perubahan auth, akses materi tamu, migrasi route lama massal, eksekusi kode,
AI, sandbox, bank soal lintas modul, real-time collaborative editing, SCORM, timer Arena,
regrade hasil lama, restore revisi, dan perubahan kebijakan nilai/attempt/reveal tanpa persetujuan.

## Catatan pelaksanaan

GitHub CLI `gh` tidak tersedia saat pemeriksaan awal; pencarian GitHub tidak berhasil, bukan bukti
ketiadaan solusi. Lanjutkan melalui dokumentasi resmi/API publik dan registry yang tersedia.

# Kelasverse — Tugas kelas & Modul jadi popup (prompt untuk Claude Code)

Referensi visual: canvas "Kelasverse · Redesign Tugas Kelas" (5 artboard: Sidebar, Popup tugas, Semua tugas, Popup modul gedung, Popup kerjakan modul).

## Masalah sekarang
- `AssignmentPanel.vue` menampilkan daftar langkah + isi materi di sidebar ±240px: teks kecil, harus scroll panjang, peta di sebelahnya tidak terpakai.
- `BuildingModules.vue` membuka popup katalog (jalur zig-zag, boros tempat), lalu langkah modul dibuka sebagai popup kedua yang bertumpuk dengan area baca sempit.

## Batasan
- **Tidak ada perubahan API/backend.** Pakai `assignmentApi.list/steps/complete` dan API modul yang sudah ada apa adanya.
- Field yang dipakai tetap: assignment `id, title, class_name, year_label, cancelled, done, total, due_at`; step `id, title, kind (materi|video|berkas|kuis), unlocked, done, rich_content|body, video_id, file_id, file_kind, quiz_id`.
- Aturan bisnis tetap: tugas = urutan wajib (`s.unlocked`), langkah kuis selesai otomatis saat lulus kuis lewat tugas (hasil kuis katalog tidak menyelesaikan tugas); modul = semua langkah terbuka.
- Token dari app: font Plus Jakarta Sans, `--kv-ink #1d2140`, `--kv-primary #5b4ee0`, `--kv-line #e9ebf3`, muted teks `#5d6280`, `--kv-star #f59e0b`. Warna mapel = `uiColor` gedung di `world.json`.

## 1. Komponen bersama baru: `LearnPopup.vue` (shell)
Dipakai oleh tugas DAN modul supaya pola belajarnya satu.
- Overlay `rgba(29,33,64,.58)` di atas peta, kotak putih `max-width: 1160px`, `min-height: min(790px, 100vh - 96px)`, radius 24, bayangan besar. Tutup dengan tombol ✕ (44×44), `Esc`, dan klik overlay. Fokus terkunci di dalam popup; kembalikan fokus ke pemicu saat ditutup. Pakai `<Teleport to="body">`.
- Slot: `header`, `default` (body). Prop `wide` opsional.
- Saat popup terbuka, input gerak avatar (WASD) di `CampusScene` dimatikan.

## 2. `StepPlayer.vue` (isi popup kerjakan langkah)
Props: `steps`, `currentId`, `mode: 'tugas' | 'modul'`. Emits: `select(id)`, `complete(id)`, `start-quiz(quizId)`.
- **Header**: chip mapel (warna lembut dari `uiColor`), label kecil ("Tugas kelas" / "Modul · {jenjang}"), judul 30px/800, baris meta (kelas · tahun, tenggat via `deadline()`, "Urutan wajib" atau "Boleh loncat langkah"), bilah progres "{done} dari {total} selesai", tombol "Semua tugas/Semua modul".
- **Kolom kiri** (`nav`, flex `1 1 280px`, bg `#f7f7fc`): tombol per langkah 2 baris (judul 15px/700; "{Jenis} · {Status}" 13px). Badge 36px: selesai = hijau + centang, aktif = ungu + nomor, belum = outline + nomor, terkunci = abu + gembok (`disabled`, opacity .55). `aria-current="step"` di langkah aktif. Catatan kecil di bawah: tugas → "Langkah berikutnya terbuka setelah langkah sebelumnya selesai. Progres tugas terpisah dari katalog; nilai dan progres lama tetap tersimpan."; modul → "Semua langkah terbuka…".
- **Area utama** (flex `999 1 560px`, isi `max-width: 820px`, padding 30/44): tag jenis + "Langkah n dari N · durasi", judul langkah 32px/800, lalu per `kind`:
  - `materi`: `rich_content` (fallback `body`) 17px / line-height 1.7, gambar boleh di kanan.
  - `video`: player 16:9 radius 18 (iframe sandbox yang sudah ada), keterangan di bawah.
  - `berkas`: kartu file (ikon jenis, nama, ukuran, "Buka di tab baru") + area pratinjau.
  - `kuis`: kartu oranye `#fff3e8`, judul + jumlah soal, teks aturan, tombol "Mulai kuis" / "Kuis lulus". Tidak ada "Tandai selesai".
- **Footer** sticky di bawah area utama: "Sebelumnya" | spacer | badge "Selesai" atau tombol utama "Tandai selesai" | "Lanjut" (disabled jika langkah berikut terkunci/terakhir). Semua tombol tinggi ≥44px.
- Responsif: di bawah ±860px kolom langkah pindah ke atas (flex-wrap), footer tetap di bawah.

## 3. Tugas kelas
- **`AssignmentPanel.vue` (sidebar)** jadi peluncur: judul "Tugas kelas" + pill "{n} aktif" + tautan "Lihat semua". Maks 2–3 kartu aktif: lingkaran progres 44px ("done/total"), judul, "Mapel · Kelas", tenggat (oranye `#fff1e6/#9a3d07` jika ≤ 3 hari), tombol "Lanjutkan"/"Mulai". Klik kartu → buka `LearnPopup` + `StepPlayer` mode `tugas` di langkah pertama yang belum selesai. Isi langkah TIDAK lagi dirender di sidebar.
- **`AssignmentList.vue` (baru, isi popup "Semua tugas")**: header judul + ringkasan hitungan + kolom cari (debounce ke `assignmentApi.list(org, 0, search)`), tab saring Semua / Belum selesai / Selesai / Dibatalkan, grid kartu `repeat(auto-fill, minmax(320px, 1fr))`. Kartu: chip mapel, status, judul 20px, "{total} langkah · jenis…", progres, tenggat, CTA. Dibatalkan = opacity .7 + "Lihat" (riwayat tetap bisa dibuka). Pagination `limit=20` dengan "Muat lebih banyak".

## 4. Modul
- **`BuildingModules.vue`** → isi `LearnPopup`: header berwarna gedung (`uiColor` lembut) + ikon gedung + "Gedung · {mapel}" + "Modul {nama gedung}", tab jenjang SD/SMP/SMA/Umum di header. Body: "Modul jenjang {x}" + grid kartu modul (baris titik per langkah: M/V/B/? atau centang hijau jika selesai, status, judul, deskripsi 15px, "{n} langkah · {menit} menit · {done} dari {n} selesai", CTA Mulai/Lanjutkan/Ulangi). Ganti jalur zig-zag. Bagian "Kuis di gedung ini" di bawah (kartu oranye, "{soal} soal · nilai terbaik {x}", "Main lagi"). Kosong → "Belum ada modul untuk jenjang ini."
- Klik kartu modul → **ganti isi popup yang sama** ke `StepPlayer` mode `modul` (tombol "Semua modul" kembali ke katalog). Jangan buka popup kedua di atasnya.
- `ResumeLearning.vue` "Lanjutkan belajar" di sidebar ikut membuka `StepPlayer` modul di langkah terakhir.

## Urutan kerja
1. `LearnPopup.vue` (overlay, fokus, Esc, matikan input gerak).
2. `StepPlayer.vue` dengan 4 jenis langkah + aturan kunci per mode.
3. Sidebar `AssignmentPanel.vue` jadi peluncur + `AssignmentList.vue`.
4. `BuildingModules.vue` jadi katalog kartu di dalam popup, langkah modul pakai `StepPlayer`.
5. Cek: tab keyboard, Esc, layar 390px, tugas dibatalkan, tugas tanpa tenggat, langkah kuis.

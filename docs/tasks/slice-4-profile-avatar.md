# Slice 4 — Profil & avatar (US-04)

Status: **draft rencana; belum diimplementasikan**. Branch implementasi: `slice-4-profile-avatar`.
Prasyarat: checklist slice 3 selesai dan direview, lalu branch dibuat dari hasil slice 3 tersebut.
Slice 5 belum boleh dimulai sebelum slice ini selesai dan direview.

## Konteks dan sumber kebenaran

- `docs/spec.md`: US-04, Keamanan dan moderasi, NFR.
- `docs/design.md`: Auth, API contract (`GET/PATCH /api/me`), protokol real-time, kolom `users`.
- `docs/visual.md`: Avatar, komponen dasar, aksesibilitas.
- Screenshot: `docs/screens/01-style-guide.png`, `02-peta-sekolah.png`, `03-ruang-kelas.png`.
- `docs/tasks/phase-4.md`: batas slice 1/4 dan aturan satu branch per slice.

Saat rencana ditulis, checkout aktif `phase-3-setup`, working tree bersih, hanya kerangka aplikasi dan
`docs/tasks/slice-1-auth.md` terlihat. Implementasi slice 1–3 yang disebut pengguna belum tersedia di checkout ini.
Path di bawah adalah target awal, bukan klaim tentang struktur hasil slice 3. Cocokkan dahulu; jangan menimpa
handler, store, route, atau tipe yang sudah ada di branch tersebut. Push bukan bagian tugas perencanaan ini.

## Hasil dan batas scope

Pengguna login bisa mengubah nama tampilan dan memilih minimal 12 kombinasi avatar preset.
Perubahan tersimpan, bertahan setelah reload/login ulang, dan dipakai renderer Phaser saat masuk peta.
Nama 3–24 karakter serta lolos filter kata kasar; validasi wajib di server, bukan hanya frontend.

Tidak mencakup unggah avatar, avatar kustom, DM, filter chat slice 6, moderasi slice 7, atau voice.
Tanggal lahir dan kontrak profil slice 1 tetap berfungsi; perubahan parsial tidak menghapus field lain.

## Keputusan yang perlu disetujui sebelum kode

Usulan berikut **belum menjadi keputusan desain**. Setelah disetujui, catat di `docs/design.md`
(dan `docs/visual.md` untuk katalog/tampilan), baru implementasikan.

| ID | Pertanyaan | Usulan untuk direview |
| --- | --- | --- |
| P4-01 | Cara menghitung karakter dan normalisasi nama? | Trim, normalisasi Unicode NFC, hitung Unicode code point; 3–24. Tolak karakter kontrol. Tetapkan juga perlakuan spasi berulang dan nama lama yang tidak valid. |
| P4-02 | Sumber dan aturan filter kata kasar? | Daftar lokal berlisensi jelas, Bahasa Indonesia dan istilah umum English, dikelola server. Tetapkan normalisasi pencocokan, batas kata, dan kasus false positive; nama ditolak, bukan diam-diam disensor. |
| P4-03 | Katalog key dan warna yang sah? | Enam aksesori `round`, `cap`, `glasses`, `headset`, `bun`, `beret`, masing-masing minimal dua warna; key `<aksesori>-<warna>`. Daftar key eksplisit di kontrak, palet lewat token. `default` lama tetap dapat dirender tanpa migrasi massal. |
| P4-04 | Kapan perubahan terlihat oleh peserta lain yang sudah online? | Usulan perubahan langsung: setelah commit DB, kirim pembaruan identitas lewat inbox room; detail event/payload baru harus disetujui dan dicatat. Jangan memasukkan identitas ke `snapshot` posisi tanpa keputusan kontrak. |
| P4-05 | Lokasi UI profil belum punya mockup khusus? | Panel/form profil dari header shell yang sudah ada, preview avatar dan pilihan native yang bisa dioperasikan keyboard; ikuti style guide. Setujui akses masuk dan layout sebelum UI dibuat. |

P4-04 perlu menetapkan perilaku saat commit DB berhasil tetapi koneksi putus atau publish gagal:
DB tetap sumber kebenaran, profil dimuat ulang saat reconnect; jangan melaporkan seolah DB gagal.
Kontrak OpenAPI memakai lokasi yang telah disepakati pada slice 2; jangan menetapkan lokasi baru dari draft ini.

## Peta perubahan

| Area | File/kontrak target awal |
| --- | --- |
| REST | Perluas `PATCH /api/me` dengan `display_name?`, `avatar_key?`; pertahankan `birth_date?`. `GET /api/me` memuat identitas tersimpan. |
| Backend | Perluas `internal/auth/{handler,service,store}.go` yang menangani profil; validator/katalog/filter dalam file domain kecil di package yang sama. Jangan membuat endpoint profil kedua. |
| SQL | Perluas `db/queries/users.sql`, jalankan `make sqlc`; jangan edit `internal/platform/db/query/` manual. |
| Migration | Tidak perlu jika skema hasil slice 3 sudah memiliki `display_name` dan `avatar_key`. Constraint baru hanya setelah kebijakan legacy disetujui, lewat migration baru; jangan ubah `00001_init.sql`. |
| Realtime | Jika P4-04 diterima: ubah `internal/realtime/message.go`, jalur inbox room, serta cermin `src/realtime/messages.ts` dalam slice yang sama. |
| Frontend | API profil pada `src/api/`, store auth/profil yang sudah ada, form profil, selector/preview avatar, renderer Phaser hasil slice 3. |
| Aset | `classverse-frontend/src/assets/avatars/`, sprite/texture pipeline hasil slice 3, `src/styles/tokens.css` bila token katalog belum ada. |
| Kontrak | OpenAPI sumber hasil slice 2 dan generated model Go/TypeScript melalui generator yang sudah dipakai. Tidak ada impor source backend oleh frontend. |

Jangan log nama pengguna, tanggal lahir, isi input filter, atau token. Error field memakai bentuk
`validation_failed` + `error.fields` dari `httpx`. Pengguna hanya mengubah profil dirinya dari identitas auth.

## Urutan tugas

### 4.0 — Pastikan baseline dan setujui kontrak

- [ ] Temukan branch/commit lokal slice 3; cek checklist dan hasil review slice 1–3. Jangan checkout atau memindahkan perubahan pengguna tanpa konfirmasi.
- [ ] Petakan handler profil, generated types, registry avatar, dan renderer aktual; perbarui path rencana ini.
- [ ] Selesaikan P4-01 sampai P4-05 dan dokumentasikan; perluas OpenAPI/payload real-time sesuai keputusan.

Verifikasi: review diff kontrak, contoh PATCH parsial, contoh key valid/tidak valid, dan kompatibilitas `default`.
Dependensi: hasil slice 3. Ukuran: kecil, dokumentasi/kontrak.

### 4.1 — Nama tampilan tersimpan lewat API

- [ ] Tambahkan validasi panjang/normalisasi dan filter server sesuai P4-01/P4-02, termasuk jalur lain yang menerima nama (registrasi/profil) tanpa merusak OAuth/onboarding.
- [ ] Perluas query update parsial tanpa menghapus avatar/tanggal lahir; update profil tetap atomik jika satu field gagal.
- [ ] Tambahkan test service table-driven dan handler; update tipe hasil generate.

File: handler/service profil, validator/filter, query users, test terkait; pecah perubahan query dan handler menjadi subtask bila melampaui satu sesi.
Verifikasi: test batas 2/3/24/25 karakter, Unicode, whitespace, kata terlarang, false positive,
PATCH field dihilangkan/null sesuai kontrak, user tanpa auth, serta regresi tanggal lahir.
Dependensi: 4.0. Ukuran: sedang per subtask.

### 4.2 — Pilihan avatar melalui API dan aset

- [ ] Tetapkan minimal 12 key sah beserta texture/preview yang cocok; seluruh kombinasi dapat dirender.
- [ ] Validasi allowlist di server; key/path/URL bebas ditolak, `default` lama tetap aman dirender.
- [ ] Integration test PostgreSQL membuktikan nama/avatar bertahan dan field lain tidak berubah.

File: katalog/validator, query/store, aset avatar dan test. Verifikasi: seluruh key dalam kontrak punya aset,
key tidak dikenal ditolak, reload profil menghasilkan nilai tersimpan. Dependensi: 4.1. Ukuran: sedang per kelompok aset/API.

### Checkpoint — Data dan kontrak

- [ ] Test backend profil dan integration test PostgreSQL lulus; generated types sinkron.
- [ ] Review katalog, lisensi filter, normalisasi, serta kompatibilitas akun lama sebelum UI.

### 4.3 — Form profil dan pemilihan avatar

- [ ] Buat form sesuai P4-05: preview, nama, pilihan aksesori/warna, simpan/batal, status loading/sukses/error.
- [ ] Pilihan minimal 44×44 px; label, fokus keyboard, status terpilih selain warna, error per field; tombol simpan mencegah submit ganda.
- [ ] Hasil simpan memperbarui state dari respons server; batal/gagal tidak mengubah profil tersimpan.

File: komponen form/selector, client API, store yang ada, unit test. Verifikasi: Vitest untuk pilihan,
validasi, simpan/gagal/batal; browser untuk fokus, layout, dan reload. Dependensi: 4.2. Ukuran: sedang per komponen/integrasi.

### 4.4 — Identitas di peta dan sinkronisasi online

- [ ] Renderer memakai `avatar_key` dan nama server, dengan fallback `default`/key tidak dikenal.
- [ ] Terapkan perilaku online P4-04 yang disetujui melalui owner room, tanpa state room lintas goroutine.
- [ ] Uji dua browser, reconnect, dan perpindahan scene; tidak ada identitas lama atau sprite hilang.

File: renderer hasil slice 3, registry texture, pesan real-time kedua sisi, jalur inbox, test.
Verifikasi: test payload/state, `go test -race ./...`, demo dua akun. Dependensi: 4.3. Ukuran: sedang per backend/frontend.

### 4.5 — Review dan penutupan

- [ ] Jalankan seluruh pemeriksaan di bawah, catat hasil aktual; jangan centang berdasarkan rencana.
- [ ] Demo: buka profil → pilih avatar/nama → simpan → terlihat di peta → reload/login ulang → tetap sama.
- [ ] Demo penolakan nama/key tidak valid, pembaruan peserta lain sesuai P4-04, dan akun legacy `default`.
- [ ] Review manusia selesai sebelum mulai slice 5; commit implementasi menyebut `US-04`.

## Pemeriksaan wajib saat implementasi

```sh
cd classverse-backend
make test && make lint
go test -race ./...

cd ../classverse-frontend
npm run lint && npm run test:unit -- --run && npm run build
npm run dev
```

Store test memakai PostgreSQL asli. Demo browser memakai backend berjalan dan minimal dua akun.
Laporan akhir memuat perubahan, perintah/hasil, keputusan yang disetujui, dan penundaan.
Saat rencana ini dibuat, pemeriksaan aplikasi belum dijalankan karena tidak ada perubahan kode.

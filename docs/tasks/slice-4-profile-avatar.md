# Slice 4 — Profil & avatar (US-04)

Status: **implementasi selesai, menunggu review manusia**. Branch implementasi: `slice-4-profile-avatar`.
Prasyarat: checklist slice 3 selesai dan direview, lalu branch dibuat dari hasil slice 3 tersebut.
Slice 5 belum boleh dimulai sebelum slice ini selesai dan direview.

## Konteks dan sumber kebenaran

- `docs/spec.md`: US-04, Keamanan dan moderasi, NFR.
- `docs/design.md`: Auth, API contract (`GET/PATCH /api/me`), protokol real-time, kolom `users`.
- `docs/visual.md`: Avatar, komponen dasar, aksesibilitas.
- Screenshot: `docs/screens/01-style-guide.png`, `02-peta-sekolah.png`, `03-ruang-kelas.png`.
- `docs/tasks/phase-4.md`: batas slice 1/4 dan aturan satu branch per slice.

Baseline implementasi: commit slice 3 `47a920e`, sudah direview dan di-push pengguna.
Branch `slice-4-profile-avatar` dibuat dari working tree bersih. Auth profil dan query update sudah ada;
implementasi memperluasnya tanpa migration. Commit/push tidak dilakukan oleh agent.

## Hasil dan batas scope

Pengguna login bisa mengubah nama tampilan dan memilih minimal 12 kombinasi avatar preset.
Perubahan tersimpan, bertahan setelah reload/login ulang, dan dipakai renderer Phaser saat masuk peta.
Nama 3–24 karakter serta lolos filter kata kasar; validasi wajib di server, bukan hanya frontend.

Tidak mencakup unggah avatar, avatar kustom, DM, filter chat slice 6, moderasi slice 7, atau voice.
Tanggal lahir dan kontrak profil slice 1 tetap berfungsi; perubahan parsial tidak menghapus field lain.

## Keputusan disetujui

P4-01…P4-05 disetujui pengguna sebelum kode. Keputusan final di `docs/design.md` dan `docs/visual.md`:
NFC, spasi digabung, kontrol/format ditolak, code point 3–24; PATCH omitted tidak berubah dan null ditolak;
filter kecil milik proyek kata utuh Indonesia/English; enam aksesori × nila/mangga; `default` kompatibel;
event `profile_updated {id,name,avatar}` sesudah DB commit; dialog native Profil dari header.
Tabel di bawah menyimpan usulan awal; ketentuan final di atas dan desain yang berlaku.

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

- [x] Baseline slice 3 `47a920e`, review pengguna selesai, working tree bersih sebelum branch baru.
- [x] Handler auth, query users, generated models, CampusScene dan World dipetakan dan diperluas.
- [x] P4-01 sampai P4-05 disetujui, desain/OpenAPI/WS kedua sisi diperbarui.

Verifikasi: review diff kontrak, contoh PATCH parsial, contoh key valid/tidak valid, dan kompatibilitas `default`.
Dependensi: hasil slice 3. Ukuran: kecil, dokumentasi/kontrak.

### 4.1 — Nama tampilan tersimpan lewat API

- [x] Validator NFC, panjang, kontrol/format dan filter dipakai profil/registrasi; OAuth memakai fallback aman.
- [x] Query update parsial menjaga field lain; invalid field membatalkan seluruh update.
- [x] Test validator table-driven, service/store/handler, generated Go/TS sinkron.

File: handler/service profil, validator/filter, query users, test terkait; pecah perubahan query dan handler menjadi subtask bila melampaui satu sesi.
Verifikasi: test batas 2/3/24/25 karakter, Unicode, whitespace, kata terlarang, false positive,
PATCH field dihilangkan/null sesuai kontrak, user tanpa auth, serta regresi tanggal lahir.
Dependensi: 4.0. Ukuran: sedang per subtask.

### 4.2 — Pilihan avatar melalui API dan aset

- [x] 12 key allowlist dan SVG preview/texture dari enam aset asli.
- [x] Allowlist server menolak URL/path; `default`/unknown fallback aman di renderer.
- [x] Integration test PostgreSQL membuktikan persistensi/atomisitas dan birth_date tidak berubah.

File: katalog/validator, query/store, aset avatar dan test. Verifikasi: seluruh key dalam kontrak punya aset,
key tidak dikenal ditolak, reload profil menghasilkan nilai tersimpan. Dependensi: 4.1. Ukuran: sedang per kelompok aset/API.

### Checkpoint — Data dan kontrak

- [x] Test backend profil/integrasi lulus; generated types sinkron.
- [x] Katalog/normalisasi disetujui, filter authored proyek, kompatibilitas default dites sebelum demo UI.

### 4.3 — Form profil dan pemilihan avatar

- [x] Dialog native: preview, nama, radio preset, simpan/batal, loading/sukses/error.
- [x] Target pilihan >44 px, native radio, label/fokus, error field, submit ganda dicegah.
- [x] Store hanya diperbarui respons sukses; batal/gagal tidak mengubah data tersimpan.

File: komponen form/selector, client API, store yang ada, unit test. Verifikasi: Vitest untuk pilihan,
validasi, simpan/gagal/batal; browser untuk fokus, layout, dan reload. Dependensi: 4.2. Ukuran: sedang per komponen/integrasi.

### 4.4 — Identitas di peta dan sinkronisasi online

- [x] Renderer memakai avatar/nama server, fallback default/unknown.
- [x] `profile_updated` dikirim melalui owner room; state tidak diakses lintas goroutine.
- [ ] Uji dua browser, reconnect, dan perpindahan scene; tidak ada identitas lama atau sprite hilang.

File: renderer hasil slice 3, registry texture, pesan real-time kedua sisi, jalur inbox, test.
Verifikasi: test payload/state, `go test -race ./...`, demo dua akun. Dependensi: 4.3. Ukuran: sedang per backend/frontend.

### 4.5 — Review dan penutupan

- [x] Pemeriksaan wajib dijalankan; hasil aktual tercatat di bawah.
- [x] Demo profil, simpan, peta, reload/login ulang tetap sama.
- [x] Browser membuktikan penolakan nama, update remote dan legacy default; key ilegal dites handler/server.
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
## Hasil aktual

- `make test` dengan `-race` dan PostgreSQL asli lulus; `make lint` 0 issues.
- Frontend lint, 55 unit test (9 file), type-check dan build lulus.
  Peringatan ukuran chunk Phaser tetap ada seperti slice 3.
- Backend test: batas nama 2/3/24/25 code point, NFC, spasi, kontrol/format, kata utuh,
  false positive substring, 12 key, URL/path ditolak, null per field, tanpa auth,
  update parsial menjaga birth_date, atomisitas invalid avatar. Publisher gagal tetap sukses simpan;
  gateway/room mengirim identitas ke pengirim dan peserta lain tanpa mengubah posisi.
- Frontend test: katalog 12 SVG dengan warna, fallback default, dialog simpan/gagal/batal,
  World memperbarui identitas tanpa memindahkan atau menampilkan avatar di luar AOI.
- Chrome + Vite `npm run dev`, dua akun: Dimas mengganti nama menjadi `Dimas Baru`
  (spasi dinormalisasi) dan `glasses-mangga`; browser Bu Rina menerima satu `profile_updated`
  dan merender identitas baru. Reload membuka profil dengan nama/avatar tersimpan.
  Nama terlarang ditolak dengan error field; Batal menutup dialog tanpa menyimpan draft.
  Akun legacy `default` tetap terlihat. Tidak ada exception aplikasi; HTTP 400 penolakan disengaja.
- Bukti lokal: `/tmp/kv-e2e/s4-profile.png`, `s4-map.png`, `s4-remote.png`,
  `/tmp/kv-p4-browser-final.txt`; skrip `/tmp/kv-e2e/slice4.mjs`.
- Demo tambahan: offline/reconnect memuat `glasses-mangga`; logout/login ulang mempertahankan
  `Dimas Baru` dan `glasses-mangga`. Escape menutup dialog dan mengembalikan fokus ke tombol Profil.
  Pengubahan profil bersamaan lintas tab dan audit screen reader lengkap belum diuji manual.
- Filter authored di `internal/auth/validate.go`, tanpa daftar eksternal; tidak menjamin mendeteksi
  semua ejaan tersamar. Tidak ada upload avatar, sprite animasi arah, atau perubahan moderasi/chat.
- Review manusia tetap pending; slice 5 belum dimulai.

# Revamp peta — usulan E, F, G (menunggu keputusan)

Status: **usulan, belum ada kode.** Ketiganya butuh keputusan produk dulu: `CLAUDE.md` meminta keputusan desain baru
diusulkan sebelum diterapkan, dan `docs/tasks/mvp2-learning-plan.md` (B2) melarang angka XP/misi sebelum formula
disepakati. Setiap bagian ditutup dengan pertanyaan yang perlu dijawab.

## E — Gaya avatar seperti prototipe (hijab, topi, kacamata)

**Sekarang:** `users.avatar_key` (text) menyimpan `look-<kulit 0..5>-<rambut 0..4>-<baju 0..9>`. Validasi ada di
`internal/auth/validate.go` dan `src/game/avatars.ts`; gambarnya dari `src/game/avatarPaint.ts`.

**Usulan termurah:** tambah satu digit aksesori ke key: `look-<kulit>-<rambut>-<baju>-<aksesori>`, misalnya
0 = tanpa, 1 = hijab, 2 = topi, 3 = kacamata. Key 3 digit lama tetap valid (= tanpa aksesori).
- **Tanpa migration**: kolomnya text. Perubahan ada di validasi BE+FE, pelukis avatar, pemilih di `ProfileDialog`,
  OpenAPI (deskripsi `avatar_key`), dan test.
- Aksesori **dipilih sendiri oleh pengguna**, bukan otomatis dari peran. Peran disimpan per sekolah
  (`memberships.role`), sedangkan avatar per pengguna, jadi "baju guru otomatis" akan salah untuk orang yang guru di
  satu sekolah dan peserta di sekolah lain. Pembeda guru tetap lewat warna label nama, seperti sekarang.

**Keputusan yang dibutuhkan:**
1. Daftar aksesori apa saja (hijab, topi, kacamata, lainnya)? Warna hijab ikut warna baju atau dipilih terpisah?
2. Setuju aksesori dipilih pengguna, bukan dari peran?

## F — HUD level/XP/koin/runtun, Misi harian, penanda "!"

Ini adalah slice **B1 → B2** di `mvp2-learning-plan.md` (Redis leaderboard sudah di ADR-011). Prasyarat dari rencana:
Q2 (Arena Kuis live) selesai dulu, lalu B1 (poin + leaderboard), baru B2 (bintang/runtun/misi/lencana).

**Keputusan yang dibutuhkan sebelum mulai (sudah tercatat sebagai "keputusan tersisa" no. 3):**
1. Rumus poin: berapa poin per modul selesai, per kuis (hanya nilai terbaik per minggu?), tanpa bonus kecepatan?
2. Rumus level dari poin, dan apakah "koin" berbeda dari poin (kalau koin bisa dibelanjakan, itu toko item = di luar
   scope rencana).
3. Runtun: hari belajar valid (WIB) = minimal apa (satu langkah modul? satu kuis?). Reset kapan?
4. Misi harian: daftar misi statis apa saja (contoh prototipe: kunjungi Menara Sains, main kuis di Arena, baca modul di
   Perpustakaan), hadiahnya, dan kapan reset. "Kunjungi gedung" butuh event kunjungan dari server (travel sudah ada).
5. Urutan: tetap Q2 → B1 → B2, atau B1 dulu tanpa Arena?

Tanpa jawaban di atas, HUD hanya boleh menampilkan progres belajar nyata (modul/kuis), bukan angka contoh.

## G — Kampus lebih dari 200 orang

**Sekarang:** satu room kampus per sekolah, kapasitas 200 (`CreateCampusRoom`). Uji beban: 500 di satu kampus melewati
jatah tick 100 ms (lihat `realtime-capacity.md`); di browser 500 avatar murah (bench `/bench/map.html`).

**Pilihan:**
- **G1 — Kampus paralel (shard):** sekolah punya beberapa room kampus ("Kampus 1", "Kampus 2"); pengguna masuk ke yang
  belum penuh, bisa pindah manual. Orang di kampus berbeda tidak saling melihat atau chat `near`. Butuh: migration
  (lebih dari satu room `map_key='campus'` per org), aturan penempatan di hub, pemilih kampus di UI, dan penyesuaian
  `travel` (keluar ruang kembali ke kampus asal).
- **G2 — Naikkan kapasitas dengan optimasi server:** AOI lebih kecil atau tick 5 Hz di kampus saja. Lebih murah, tapi
  belum ada bukti 500 lolos; perlu uji ulang dan bisa mengubah rasa gerak.
- **G3 — Tetap 200:** cukup untuk pilot (satu kelas 30 orang, kriteria M1). Orang ke-201 mendapat pesan penuh.

**Keputusan yang dibutuhkan:**
1. Berapa target orang bersamaan per sekolah untuk pilot berikutnya?
2. Kalau lebih dari 200: G1 (shard) atau G2 (optimasi + uji ulang)?

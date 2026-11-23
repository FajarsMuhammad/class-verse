# Handoff — Revamp peta kampus (tahap 1 + 2)

Status per sesi ini. Dibaca lewat `CLAUDE.md` lebih dulu; acuan desain: `kelasverse-map-revamp/` (`prototipe-peta.html`,
`map-revamp-claude-code.md`, `preview-peta.png`). Branch `kelola-management-api`, **belum di-push**.

## Sudah selesai

| Commit | Isi |
|---|---|
| `fb01657` | Tahap 1: lantai, jalan paving, Taman Diskusi, Alun-alun, Arena Kuis, 7 jenis pohon, 12 jenis dekor, bayangan, goyang pohon |
| `6cb195d` | Tahap 2: lencana jumlah orang di label gedung, avatar berjalan, nama dekat/hover, kupu-kupu + bayangan awan, minimap berwarna |

Berkas utama (semua di `classverse-frontend/src/`):
- `game/campusLayout.ts` (jalan, pohon, dekor; penempatan acak ber-seed, **sama untuk semua pemain**) + `campusLayout.spec.ts`
- `game/campusPaint.ts` (pelukis Canvas 2D: lantai, pohon, dekor)
- `game/avatarPaint.ts` + spec (sheet jalan 4 arah x 3 frame dari `avatarLook`)
- `game/mapLabels.ts` + spec (aturan nama dekat/hover dan lencana)
- `game/worldArt.ts` (`drawWorld`, label gedung, atlas, animasi, ambient), `game/CampusScene.ts`, `components/MapMinimap.vue`
- `assets/world/world.json`: `trees`, `lawns`, `npcs`, sprite zona dihapus; lima SVG lamanya ikut dihapus.

**Collision server tidak berubah.** `generate_world.py` hanya memakai `places` dan `zones`. Cek: `python3 classverse-frontend/scripts/test_generate_world.py`.

## Yang BELUM selesai (urut prioritas)

1. **Kecepatan gambar di GPU sungguhan belum terukur.** Di Chrome headless (SwiftShader) peta baru terukur 100–200 ms/frame,
   tetapi peta lama juga terukur 16–105 ms tergantung kondisi mesin, jadi selisihnya tidak bisa dipercaya
   (lihat kartu memex `kelasverse-map-revamp-perf-measurement`). **Uji di browser biasa dengan GPU atau HP.**
   Kalau berat, kandidat pertama:
   - tekstur lantai terlalu besar: `GROUND_SCALE = 1.5` di `campusPaint.ts` (3300x2250 px, ~30 MB); coba `1`;
   - `ambient()` di `worldArt.ts` memakai `Graphics` yang di-`clear()` dan digambar ulang tiap frame (awan + kupu-kupu);
     ganti ke satu tekstur awan yang digeser, atau kurangi jumlahnya;
   - animasi bendera/air mancur/papan skor (`ANIMATED` di `campusPaint.ts`): empat canvas kecil di-`refresh()` tiap frame.
   **Sudah diperbaiki (belum di-commit):** `ART_SCALE`/`GROUND_SCALE` = 1 (atlas 63→15 MB, lantai 30→13 MB),
   `ambient()` memakai tekstur awan/sayap yang digeser, prop animasi di-refresh 20x/detik. Tinggal uji di GPU/HP.
   Cara ukur yang berhasil: toggle sementara `window.__kv` (noGround/noTrees/noAmbient) + interval rAF 60 frame.
   **Hapus toggle sebelum commit** (`grep -rn __kv src` harus kosong).
2. **Cek commit `f3f7028`**: "Implement code changes…" menambah `db/kelasverse.sql` (8.900 baris, dump SQL). Bukan dari sesi ini.
   Periksa isinya (data pribadi? kredensial?) sebelum push; mungkin tidak boleh masuk repo.
3. **Tidak dikerjakan sengaja (butuh backend, kalau tidak jadi data palsu):** HUD level/XP/koin/runtun, panel Misi harian,
   penanda misi "!". Aturan proyek melarang data/kontrol palsu. Kalau mau dibuat, butuh keputusan produk + migration + API.
4. Gaya avatar belum membedakan peran (hijab, guru, topi) seperti prototipe karena data avatar hanya menyimpan kulit/rambut/baju.
5. Goyang pohon memakai rotasi kecil di pangkal batang (Phaser 3 tidak punya skew; tipe `Image` tidak mengekspos `skewX`).
6. Skrip e2e lama yang rusak karena selektor menu berubah sejak redesain Kelola (bukan karena peta):
   `workspace-exit`, `revamp-travel`, `seat-walk`, `minimap-card` di `/tmp/kv-e2e`. `map-input` dan `worldclick` lulus.

## Cara melanjutkan setelah restart laptop

```bash
cd /home/engineer/fajars/Ruparupa/class-verse/classverse-backend
make up                       # Postgres (5433) + Mailpit
make storage-up               # opsional: storage file belajar (127.0.0.1:9100), perlu untuk upload/baca PDF
setsid nohup make run > /tmp/kv-server.log 2>&1 < /dev/null &   # backend :8080, tunggu ~20 dtk, cek curl localhost:8080/healthz
cd ../classverse-frontend
setsid nohup npx vite --host > /tmp/vite-main.log 2>&1 < /dev/null &   # frontend :5173
```

Verifikasi sebelum commit (urutan proyek): `make lint` + `go test ./...` (butuh `TEST_DATABASE_URL`, jangan disimpan di dokumen),
lalu `npm run lint`, `npm run build`, `npx vitest run`, dan `python3 scripts/test_generate_world.py`.

## Jebakan yang sudah memakan waktu

- **Origin**: backend hanya mengizinkan `FRONTEND_URL=http://localhost:5173`. Membuka `127.0.0.1:5173` tidak bisa login,
  jadi skrip uji bisa mengukur halaman login dan melaporkan 16 ms. Selalu pastikan `document.querySelector('canvas')` ada.
- **Dev server Vite yang berjalan berhari-hari melambat.** Restart sebelum mengukur performa.
- **Cek `uptime` dulu**: proses `[java] <defunct>` pernah membuat load average ~96 dan merusak semua angka.
- `kill`/`pkill` yang digabung dengan perintah lain dalam satu pemanggilan ikut mematikan shell (exit 143). Pisahkan.
- Pembaca gambar kadang gagal (ENOENT) untuk file yang baru dibuat; jalankan ulang skrip screenshot sekali.
- Server yang sedang berjalan harus di-restart agar mengambil kode Go baru; frontend Vite otomatis (HMR).
- `.env` tidak bisa di-`source` (nilai `SMTP_FROM` berisi `<>`); jangan cetak isinya.

## Data uji dan skrip (di luar repo, hilang kalau `/tmp` dibersihkan saat restart)

- `/tmp/kv-e2e/` (puppeteer-core + `seed.json` organisasi uji: kredensial uji, **jangan di-commit**) dan `/tmp/kv-seed/`
  (konten seed modul/kuis/buku/ruang + `mktoken`). Setelah restart direktori ini mungkin hilang; skripnya bisa dibuat ulang
  dari pola di kartu memex `kelasverse-seed-content-via-api`.
- Data organisasi pengguna (`c744e096-…`) sudah berisi 6 modul, 6 kuis, 10 buku, 9 ruang; data awal lama diarsipkan.

## Memex (kartu yang relevan)

`kelasverse-map-revamp-perf-measurement`, `kelasverse-mute-all-realtime`, `kelasverse-db-helpers-adr-014`,
`kelasverse-seed-content-via-api`, `kelasverse-kelola-redesign-commit`.

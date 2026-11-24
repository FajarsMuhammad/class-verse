# Kelasverse — Revamp Main Map (prompt untuk Claude Code)

Referensi visual: prototipe "Kelasverse · Peta Kampus Baru" (artifact). Tujuan: main map terasa seperti game edukasi — hidup, rimbun, mudah dibaca — tanpa mengubah backend.

## Batasan (jangan dilanggar)
- **Jangan ubah** `places[].sprite_pos`, `size`, `entrance`, `interact_radius` di `src/assets/world/world.json`. Collision map dari server tetap valid.
- Semua tambahan di bawah **murni visual / client-side**. Pohon & dekorasi tidak masuk collision (kecuali nanti diputuskan lain di server).
- Tetap pakai gaya outline `#23203A` 2.5–3px, warna flat, `uiColor` per gedung.

## 1. Skema `world.json` (tambahan, backward-compatible)
```jsonc
"ground": { "color": "#a6db8e", "patches": true, "tufts": true },   // ganti abu-abu + dot grid
"paths": [ { "w": 56, "pts": [[200,440],[2000,440]] }, ... ],        // jalan paving + spur 38px ke tiap entrance
"trees": [ { "x": 882, "y": 790, "type": "beringin", "r": 62 }, ... ], // type: round|bloom-pink|bloom-orange|pine|palm|beringin|bush ; r = radius kanopi
"decor": [ { "type": "lamp", "x": 0, "y": 0 }, ... ],                 // lamp|bench|pendopo|flag|board|scoreboard|fountain|sign|easel|telescope|bikes
"ambient": { "butterflies": 14, "cloudShadows": 4 }
```
Koordinat jalan yang dipakai prototipe (y jalan atas 440, jalan bawah-tengah 1110, promenade 1415, sisi x 200/2000, vertikal x 395/1100/1780, penghubung y 830, lane x 755/1430) aman terhadap semua footprint gedung.

## 2. `src/game/worldArt.ts` → `drawWorld("campus")`
1. Ground: isi dasar rumput + bercak tonal + rumput kecil + kelompok bunga, render **sekali** ke RenderTexture (pakai `freezeGround` yang sudah ada).
2. Paths: stroke 2 lapis (tepi `#c7a877` 66px, isi pola paving `#f2e1ba` 56px), `lineCap/lineJoin = round`.
3. Zona: Taman Diskusi = rumput + pagar semak + kolam teratai + batu pijakan + pendopo; Alun-alun = paving kotak + lingkaran rumput + air mancur + **beringin kembar** + bendera merah putih + papan misi; Arena Kuis = stadion oval (tribun, lintasan, 4 pad jawaban merah/biru/kuning/hijau, papan skor).
4. Bayangan gedung & pohon ke ground layer (rgba(35,32,58,.15)).
5. Depth: semua objek tinggi (gedung, pohon, dekorasi, avatar) di-sort pakai `depth = y kaki/dasar`.

## 3. Pohon lebih besar & bagus
- Ganti `sprites/props/tree.svg` jadi set: `tree-round.svg`, `tree-bloom-pink.svg`, `tree-bloom-orange.svg`, `tree-pine.svg`, `tree-palm.svg`, `tree-beringin.svg`, `bush.svg`.
- Ukuran: kanopi r 30–64px (≈3× pohon lama); beringin r 62. Origin `(0.5, 1)` di pangkal batang.
- Kanopi 3 tone (gelap → utama → highlight kiri-atas) + titik-titik; bloom pakai bunga di atas kanopi hijau.
- Goyang halus: tween `skewX` ±0.026 rad (palm 0.045), fase acak, `yoyo: true, repeat: -1`. Matikan kalau `prefers-reduced-motion`.
- Penempatan (versi sudah dikurangi): hutan tipis di tepi peta, pohon di pinggir jalan berjarak ±300–340px, lahan tengah dibiarkan lapang (hanya sekitar 1 dari 10 titik acak yang jadi pohon/semak). Aturan: jangan di atas jalan, jangan di depan fasad gedung (base y di [top-30, bottom + 2.3r]), jarak ≥80px dari entrance.

## 4. Avatar = hanya user nyata (TANPA NPC)
- Tidak ada NPC. Siswa yang berkeliaran di prototipe hanya contoh suasana saat kampus ramai.
- Avatar yang tampil = user online dari realtime (`world.ts`/presence), dengan gaya gambar seperti prototipe: outline `#23203A`, bayangan di kaki, kaki/lengan berayun saat jalan, badan sedikit memantul.
- Nama tampil saat hover atau dalam radius 120px dari player; nama player sendiri selalu tampil (pill ungu).
- Emote user dikirim lewat realtime dan tampil sebagai gelembung di atas kepala ±2.5 dtk.
- Badge jumlah orang di label gedung = jumlah user nyata di ruangan gedung itu (dari server).
- Saat yang online sedikit, kampus tetap terasa hidup lewat efek ambient client-only: pohon bergoyang, air mancur & riak kolam, bendera berkibar, kupu-kupu, bayangan awan. Semua ini bukan avatar dan tidak ikut collision.

## 5. Lapisan game
- Label gedung = pill dengan titik `uiColor` + badge jumlah orang di dalam; ukuran font tetap di layar (tidak ikut zoom).
- Penanda misi "!" (kuning, memantul) di atas gedung yang punya misi harian aktif; hilang setelah selesai.
- Prompt "E · Masuk {gedung}" saat dalam radius entrance; "Jalan ke sini" = auto-path lewat graph.
- HUD: level + XP bar + koin + runtun; panel Misi harian; minimap dengan titik orang; tombol emote.

## Urutan kerja yang disarankan
1. Ground + paths + zona (tanpa ubah data gedung) → cek collision masih pas.
2. Set pohon baru + aturan penempatan + goyang.
3. Dekorasi (lampu, bangku, pendopo, air mancur, bendera, papan).
4. Rendering avatar user nyata (gaya baru) + emote + nama.
5. Label, penanda misi, prompt, auto-walk.

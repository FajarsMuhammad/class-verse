# Slice 10 — Perluasan kampus (US-05, US-06, US-13)

Status: **layout dan visual selesai, belum commit**.

Revisi terakhir pengguna: batalkan papan pengumuman; hapus Kebun, Lab Sains, Aula, UKS,
Studio Seni dan Mushola. Peta kini hanya 8 gedung/area: Kelas A/B/C, Ruang Guru,
Perpustakaan, Kantin, Taman dan Lapangan. Enam objek/pintu dihapus beserta jalan pendekat
khususnya; data ruang DB tidak dihapus. Ruang lama pada Aula perlu dipindah sebelum rollout.
Test `TestParseCampus` kini memeriksa tepat 8 gedung dan keenam key yang dihapus tidak ada.
Catatan 14 gedung berikut merupakan checkpoint sebelum revisi ini.
Branch: `slice-10-campus-world`, parent `a78a482`.

## Scope final

Koreksi pengguna menggantikan usulan awal: perluas isi peta dengan gedung/properti baru,
bukan zoom. Gedung bukan kotak flat. Kamera overview dipertahankan; follow dan minimap dibatalkan.

- Kampus 128×80 tile (4096×2560 px): dua kali dimensi, empat kali luas semula.
- 14 gedung/area: delapan key lama, ditambah kelas-c, lab, studio, mushola, uks, kebun.
- Jalan utara/selatan, promenade, alun-alun, kolam, kebun, pohon, bangku, lampu, bunga.
- Atap miring bertekstur dan sisi gelap, fasad bertingkat, lis/tiang, jendela, pintu besar.
- Jendela menyala dari pesan `buildings`. Overlay air/daun/bendera/lampu bergerak jarang;
  reduced motion mematikannya tanpa mengubah collision.
- Tekstur dunia 1×, bukan 8192×5120. Kamera, AOI 24 tile, speed 320 px/detik,
  chat/voice, kelas/taman dan mode HP tidak mengubah perilaku.
- Tidak menambah quest, hadiah, endpoint atau protokol baru. Papan pengumuman interaktif
  tidak masuk implementasi ini: belum ada kontrak konten/aksi; jangan menyebutnya tersedia.

## Bukti

- `python3 maps/generate_campus.py --check`: hasil sinkron, setiap pintu tersambung
  lewat tile jalan/plaza yang bebas collision (bukan sekadar bisa dicapai lewat rumput).
- Backend `make test`, `make lint`: lolos.
- Frontend `npm run lint`, `npm run test:unit -- --run`, `npm run build`: lolos.
- Unit renderer: dua lantai jendela, atap terpotong sesuai siluet, palette shade,
  overlay dekorasi berubah tanpa mengubah data peta.
- Browser `/tmp/kv-e2e/campus-world.mjs`: 14 gedung, kamera overview tidak bergerak saat jalan,
  klik pointer nyata ke pintu Kelas A, masuk kelas, berjalan keluar, kembali kampus;
  overlay bergerak, reduced motion kosong, HP tanpa canvas, tanpa page error.
- Screenshot `/tmp/kv-e2e/campus-final-desktop.png`, `/tmp/kv-e2e/campus-final-mobile.png`.
- Mode HP dipilih saat start sesi: setelah resize desktop ke HP, reload diperlukan untuk
  menguji list mode; resize saja sengaja tidak mengganti mode koneksi.

## Batas verifikasi

Voice outage tidak diuji ulang pada slice ini; tidak ada perubahan kode voice.
Tidak diuji pada GPU HP nyata atau VPS; frontend HP memakai daftar tanpa Phaser.
Papan pengumuman interaktif memerlukan task/kontrak terpisah.

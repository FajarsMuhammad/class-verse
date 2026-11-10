# Peta Tiled

File `.tmj` (Tiled JSON, orthogonal, tile 32 px) di folder ini di-embed ke binary server. Server memakainya untuk
collision, spawn, dan gedung; frontend mengambil file yang sama lewat `GET /api/maps/{map_key}` untuk render Phaser.
Nama file = `map_key` (`campus.tmj` → `campus`). Kelas/taman dapat diubah dengan [Tiled](https://www.mapeditor.org/).
Kampus 128×80 tile memakai sumber layout `generate_campus.py`: jalankan `python3 maps/generate_campus.py`
dari backend, atau `--check` untuk memeriksa hasil. Jangan edit `campus.tmj` langsung.
Kampus memuat 8 gedung/area: Kelas A/B/C, Ruang Guru, Perpustakaan, Kantin, Taman, Lapangan.
Aula, Lab, Studio, Mushola, UKS dan Kebun dihapus dari peta sesuai revisi pengguna.
Data ruang pada gedung yang dihapus tidak dihapus otomatis; perlu dipindahkan sebelum rollout bila masih dipakai.

## Properti peta

| Properti | Jenis | Isi |
| --- | --- | --- |
| `walk_speed` | int | Kecepatan jalan maksimum px/detik, 80–400, default 160. Server dan frontend memakai angka yang sama. |

## Layer (wajib)

| Layer | Jenis | Isi |
| --- | --- | --- |
| `ground` | tile | Tampilan lantai, jalan, atap, dinding, pintu |
| `collision` | tile, tersembunyi | Tile ≠ 0 tidak bisa dilewati; di luar peta juga tidak bisa dilewati |
| `spawn` | objek | Satu persegi; peserta muncul di tile bebas di dalamnya |
| `buildings` | objek | Peta sekolah: persegi `type=building` dan titik `type=door` |

Properti objek `building`: `key` (unik, disimpan sebagai `rooms.building_key`), `name`, `roof`
(`classroom` / `library` / `hall` / `canteen` / `staff` / `lab` / `studio` / `mosque` / `clinic` / kosong untuk area terbuka), `description`,
`staff_only` (bool, Ruang Guru), `quiet` (bool, Perpustakaan). Objek `door` adalah titik dengan properti `building`
yang menunjuk `key` gedungnya; titik ini harus berada di tile yang bisa dilewati (tempat berdiri untuk `travel`).

## Tileset `kelasverse` (kontrak id tile)

Frontend menggambar ilustrasi kampus pixel art dari bounds gedung dan token `tokens.css`; tileset placeholder mengikuti urutan id ini
(gid = id + 1, `firstgid` 1). Tile art baru harus mempertahankan urutan ini atau peta ikut diperbarui.

| gid | Tile | Token |
| --- | --- | --- |
| 1 | rumput | `--map-grass` |
| 2 | rumput gelap (pagar, taman, lapangan) | `--map-grass-dark` |
| 3 | jalan | `--map-path` |
| 4 | alun-alun | `--map-path` (lebih gelap) |
| 5 | dinding gedung | `--color-surface` |
| 6 | jendela | `--map-water` |
| 7 | pintu | `--map-wood` |
| 8 | atap kelas | `--roof-classroom` |
| 9 | atap perpustakaan | `--roof-library` |
| 10 | atap aula | `--roof-hall` |
| 11 | atap kantin | `--roof-canteen` |
| 12 | atap ruang guru | `--roof-staff` |
| 13 | air | `--map-water` |
| 14 | kayu (bangku) | `--map-wood` |
| 15 | garis lapangan | `--color-surface` |
| 16 | tiang bendera | `--color-ink` |
| 17 | pohon | `--color-speaking` |
| 18 | penanda collision | (tidak digambar) |
| 19 | atap lab | `--roof-lab` |
| 20 | atap studio | `--roof-studio` |
| 21 | atap mushola | `--roof-mosque` |
| 22 | atap UKS | `--roof-clinic` |
| 23 | lampu | `--map-lamp` |
| 24 | bunga | `--map-flower` |
| 25 | pasir | `--map-sand` |

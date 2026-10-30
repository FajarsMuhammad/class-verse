# Peta Tiled

File `.tmj` (Tiled JSON, orthogonal, tile 32 px) di folder ini di-embed ke binary server. Server memakainya untuk
collision, spawn, dan gedung; frontend mengambil file yang sama lewat `GET /api/maps/{map_key}` untuk render Phaser.
Nama file = `map_key` (`campus.tmj` → `campus`). Buka dan ubah dengan [Tiled](https://www.mapeditor.org/).

## Layer (wajib)

| Layer | Jenis | Isi |
| --- | --- | --- |
| `ground` | tile | Tampilan lantai, jalan, atap, dinding, pintu |
| `collision` | tile, tersembunyi | Tile ≠ 0 tidak bisa dilewati; di luar peta juga tidak bisa dilewati |
| `spawn` | objek | Satu persegi; peserta muncul di tile bebas di dalamnya |
| `buildings` | objek | Peta sekolah: persegi `type=building` dan titik `type=door` |

Properti objek `building`: `key` (unik, disimpan sebagai `rooms.building_key`), `name`, `roof`
(`classroom` / `library` / `hall` / `canteen` / `staff` / kosong untuk area terbuka), `description`,
`staff_only` (bool, Ruang Guru), `quiet` (bool, Perpustakaan). Objek `door` adalah titik dengan properti `building`
yang menunjuk `key` gedungnya; titik ini harus berada di tile yang bisa dilewati (tempat berdiri untuk `travel`).

## Tileset `kelasverse` (kontrak id tile)

Tile art final belum ada; frontend menggambar tileset flat dari token di `tokens.css` dengan urutan id ini
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

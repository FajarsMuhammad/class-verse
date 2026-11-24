# classverse-frontend

UI Kelasverse: Vue 3 + Vite + TypeScript + Pinia untuk shell aplikasi, Phaser 3 untuk peta.
Arah visual di `../docs/visual.md`; semua warna, font, dan ukuran dari `src/styles/tokens.css`.

## Prasyarat

- Node.js `^22.18` atau `>=24.12`
- Backend berjalan di `localhost:8080` (lihat `../classverse-backend/README.md`)

## Menjalankan

```sh
npm install
npm run dev   # http://localhost:5173
```

Vite mem-proxy `/api`, `/ws` (WebSocket), dan `/healthz` ke `localhost:8080`, sehingga browser melihat
satu origin (dibutuhkan cookie refresh `SameSite=Strict`, tanpa CORS).

> npm 10.9 bisa crash (`Cannot read properties of null (reading 'edgesOut')`) saat me-resolve dependency
> baru. `npm install` dari lockfile aman; untuk menambah paket pakai `npx npm@11 install <paket>`.

## Peta sekolah (slice 3)

Pilih sekolah lalu buka `/`. Fokus peta dengan klik atau Tab; WASD/panah untuk berjalan,
klik tujuan untuk mencari jalur. Keyboard membatalkan jalur klik. Daftar gedung belum membuka ruang
(travel menyusul slice 5). Tab kedua menggantikan tab pertama, tanpa reconnect otomatis di tab lama.

Backend menyajikan peta `/api/maps/campus`; Phaser menggambar tileset sementara dari CSS tokens.
`src/realtime/messages.ts` mencerminkan kontrak Go; ubah kedua sisi saat protokol berubah.
SVG avatar memakai `?no-inline`: loader SVG Phaser 3 menganggap URL data sebagai Base64,
sedangkan Vite dapat menghasilkan URL data SVG percent-encoded.

## Perintah

| Perintah | Fungsi |
| --- | --- |
| `npm run dev` | Dev server |
| `npm run test:unit` | Vitest |
| `npm run lint` | oxlint + ESLint |
| `npm run format` | Prettier |
| `npm run build` | Type-check + build produksi ke `dist/` |
| `npm run e2e` | Uji browser peta kampus (`e2e/campus.mjs`): login, minimap, kartu gedung, masuk ruang, kursi, keluar. Butuh backend + Mailpit + `npm run dev` berjalan dan Chrome di `/usr/bin/google-chrome` (atau `CHROME=`). Membuat akun/sekolah `@e2e.test` baru tiap run |
| `npm run api` | Generate `src/api/schema.d.ts` dari `../classverse-backend/api/openapi.yaml` (jangan diedit manual) |

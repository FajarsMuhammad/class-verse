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

## Perintah

| Perintah | Fungsi |
| --- | --- |
| `npm run dev` | Dev server |
| `npm run test:unit` | Vitest |
| `npm run lint` | oxlint + ESLint |
| `npm run format` | Prettier |
| `npm run build` | Type-check + build produksi ke `dist/` |

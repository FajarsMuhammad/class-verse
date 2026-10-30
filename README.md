# Kelasverse

Ruang belajar virtual 2D untuk SMA dan kampus. Monorepo dengan dua project yang berdiri sendiri:

| Folder | Isi |
| --- | --- |
| [`classverse-backend/`](classverse-backend/README.md) | Server Go (REST + WebSocket), PostgreSQL, migration, Docker Compose untuk dev |
| [`classverse-frontend/`](classverse-frontend/README.md) | Vue 3 + Vite + Phaser |
| `docs/` | Spec, desain teknis, desain visual, tugas per fase |

## Mulai cepat

```sh
# Terminal 1 — backend
cd classverse-backend
cp .env.example .env
make up && make migrate && make sqlc && make run

# Terminal 2 — frontend
cd classverse-frontend
npm install
npm run dev   # buka http://localhost:5173
```

Prasyarat dan perintah lengkap ada di README masing-masing project.

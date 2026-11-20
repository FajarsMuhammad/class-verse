# Kelasverse · Desain menu Kelola

Paket desain revisi menu **Kelola** (alat pengajar) Kelasverse, plus konsep awal alur modul & quiz.

## Isi

```
kelasverse-kelola-design/
├── README.md                  ← file ini
├── CLAUDE_CODE_HANDOFF.md     ← spesifikasi untuk diimplementasikan Claude Code
├── html/
│   ├── index.html             ← daftar semua layar, buka ini dulu
│   ├── kelola/                ← 12 layar revisi menu Kelola
│   └── alur-modul-quiz/       ← 4 layar konsep modul & quiz (QuizPlayer interaktif)
├── tokens/
│   └── tokens.css             ← warna, tipografi, radius, ukuran layout
└── source-dc/                 ← file sumber canvas desain (referensi, tidak perlu dijalankan)
```

## Cara melihat

Buka `html/index.html` di browser. Tidak perlu server atau build. Font Nunito dimuat dari Google Fonts, jadi perlu internet supaya tampilannya pas.

## Cara memakai dengan Claude Code

1. Ekstrak zip ini ke dalam repo Kelasverse, misalnya di `docs/design/kelola/`.
2. Jalankan Claude Code dari root repo, lalu beri prompt:

   > Baca `docs/design/kelola/CLAUDE_CODE_HANDOFF.md` dan implementasikan revisi menu Kelola sesuai urutan fase di dokumen itu. Mulai dari Fase 1, berhenti dan minta review saya di akhir setiap fase.

3. Mockup HTML adalah acuan visual dan perilaku, **bukan** kode yang di-copy. Komponen Vue dibuat ulang memakai struktur dan style yang sudah ada di frontend.

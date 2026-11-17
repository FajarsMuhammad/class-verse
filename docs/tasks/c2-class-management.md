# C2 — Kelola kelas, siswa, dan pengajar kelas

Lanjutan C1. Status: **C2a selesai; C2b–C2d belum.** Branch per slice, bertumpuk dari `teacher-workspace-fixes`.

## Keputusan (disetujui pengguna)

| Topik | Keputusan |
| --- | --- |
| Struktur Kelola | Submenu **Kelas**, **Siswa**, dan **Tahun ajaran** (admin) terpisah; tidak lagi satu halaman campuran |
| Peran kelas | **Wali kelas** (maksimal satu per kelas) dan **Guru mapel** (boleh banyak, mata pelajaran teks bebas opsional). Istilah "guru pengampu" tidak dipakai |
| Hak kelola (menggantikan baris "Persetujuan C1" di `mvp2-learning-plan.md`) | **Admin** mengelola tahun ajaran, semua kelas, pengajar kelas, dan penempatan siswa. **Wali kelas** mengelola siswa di kelasnya sendiri. **Guru mapel** dan guru lain hanya melihat |
| Rapor/tugas | Kelas yang diajar tampil lebih dulu; kelas lain tetap terbuka. Tidak ada pembatasan akses baru |
| Tahun ajaran baru | Salin struktur kelas (opsional dengan pengajarnya), siswa tidak disalin; lalu pindah massal. Wizard naik kelas belakangan, tetap tanpa kenaikan otomatis |
| Impor CSV siswa | Ditunda; tetap memakai undangan |

## Invariant

- Satu kelas aktif per siswa per sekolah per tahun ajaran (C1, tidak berubah). Riwayat penempatan tidak dihapus.
- Pengajar kelas harus anggota sekolah yang sama dengan peran teacher/admin; ID sekolah/kelas/pengguna asing = 404.
- Maksimal satu wali kelas aktif per kelas (unique index parsial). Satu orang tidak tercatat dua kali di kelas yang sama.
- Wali kelas hanya boleh: menambah siswa **tanpa kelas** ke kelasnya, dan mengeluarkan siswa dari kelasnya
  (menjadi tanpa kelas). Pindah antarkelas, membuat/arsip/ganti nama kelas, dan mengatur pengajar = admin.
- Kelas tanpa wali kelas hanya dapat dikelola admin. Server tetap memeriksa peran di setiap permintaan;
  UI hanya menyembunyikan aksi.
- Arsip kelas tetap hanya jika tanpa siswa aktif; arsip menutup penugasan pengajar kelas itu.

## Slice

### C2a — Pisahkan Kelas, Siswa, Tahun ajaran (frontend, tanpa migration) — selesai

- `TeacherTools`: submenu Kelas, Siswa; Tahun ajaran hanya untuk admin.
- Kelas: pilih tahun ajaran (default aktif), cari, daftar ringkas (nama, jumlah siswa) → detail satu kelas
  dengan tab **Siswa** dan **Pengaturan** (ganti nama, arsip). Pola daftar → satu item (`authoring.css`).
- Siswa: direktori dengan filter tahun ajaran, kelas, **Belum punya kelas**, cari; aksi pindah dan riwayat.
- Tahun ajaran: tambah, jadikan aktif. Memakai API C1 yang ada.
- Bukti: component test per halaman, browser 1440/850 px, `ClassManager` lama dihapus setelah setara.

### C2b — Wali kelas dan Guru mapel (migration 00017 + hak kelola)

- `class_teachers(org_id, class_id, user_id, role homeroom|subject, subject text null, started_at, ended_at, created_by)`;
  FK komposit ke `classes(org_id, id)` dan `memberships(org_id, user_id)`; unique wali aktif per kelas; unique
  orang aktif per kelas.
- API: daftar/tambah/lepas pengajar kelas (admin); daftar kelas memuat wali kelas dan jumlah guru mapel.
- Service C1 menerapkan hak kelola baru; tab **Pengajar** di detail kelas.
- Bukti: real-Postgres tests peran (admin/wali/guru mapel/guru lain/peserta), sekolah asing, wali ganda serentak.

### C2c — Siswa skala besar

- Roster dari server per halaman (maks 50), cari dan filter di server; pilih banyak → **Pindahkan ke kelas…**
  dalam satu transaksi dengan hasil per siswa.
- Bukti: pagination deterministik, pindah massal parsial gagal tidak merusak satu kelas aktif.

### C2d — Tahun ajaran baru

- Salin struktur kelas (dan opsional pengajarnya) dari tahun sebelumnya; konflik nama dilewati dan dilaporkan.
- Wizard naik kelas (pemetaan kelas lama → baru, pratinjau, konfirmasi) — dirancang setelah C2c dipakai.

## Lanjutan disetujui

- **Wali kelas boleh mengundang siswa ke kelasnya** (undangan berkelas): siswa yang menerima undangan menjadi
  peserta sekolah dan langsung ditempatkan di kelas itu pada tahun ajaran undangan. Dikerjakan setelah C2b
  (butuh data wali kelas). Invariant: undangan hanya untuk peran peserta, kelas belum diarsip, wali masih aktif
  saat undangan dibuat; penerimaan tetap gagal aman bila siswa sudah punya kelas aktif di tahun itu
  (keanggotaan sekolah tetap dibuat, penempatan dilaporkan konflik, bukan dipindah diam-diam).

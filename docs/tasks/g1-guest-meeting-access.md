# G1 — Akses tamu pertemuan lewat tautan

Status: G1a–G1d selesai dan terverifikasi lokal. Belum deploy/push; batas pilot tercatat di bawah.

## Tujuan

Guru atau admin dapat mengundang orang ke satu pertemuan tanpa membuat akun. Tamu membuka tautan,
mengisi nama tampilan, menunggu persetujuan host, lalu masuk hanya ke ruang tujuan.

Pembelajaran berkelanjutan tetap memakai akun: tugas, kuis, progres, rapor, berkas privat,
perpustakaan, keanggotaan sekolah, dan kelas/rombel.

## Bukan cakupan G1

- Tamu tidak menjadi anggota sekolah dan tidak ditempatkan di kelas.
- Tidak ada konversi tamu menjadi akun pada G1.
- Tidak ada tugas, kuis, progres, rapor, berkas, perpustakaan, laporan, atau peta sekolah untuk tamu.
- Tidak ada berbagi layar, rekaman, lobi kalender, atau pembayaran.
- Tamu tidak boleh menjelajah atau berpindah ke ruang lain.

## Alur pengguna

### Host

1. Guru/admin membuka detail ruang atau kontrol sesi lalu memilih **Undang tamu**.
2. Host menentukan masa berlaku tautan dan batas tamu.
3. Host menyalin tautan atau kode, lalu mencabutnya bila perlu.
4. Saat tamu meminta masuk, host melihat nama tamu dan memilih **Izinkan** atau **Tolak**.
5. Host dapat mengeluarkan tamu memakai moderasi sesi yang sudah ada.

### Tamu

1. Tamu membuka `/guest/{code}` dan melihat nama sekolah serta nama ruang.
2. Tamu mengisi nama tampilan. Usia tidak wajib diisi dan bukan syarat masuk.
3. Tamu menunggu persetujuan tanpa melihat isi ruang, chat, atau daftar peserta.
4. Setelah disetujui, tamu masuk ke ruang tujuan dalam mode yang sesuai perangkat.
5. Bila ditolak, tautan dicabut, sesi dikeluarkan, atau masa berlaku habis, tamu melihat pesan jelas.

## Keputusan desain yang disarankan

- **Identitas tamu memakai baris `users` khusus tamu.** Banyak riwayat realtime sudah merujuk ke
  `users(id)`: kehadiran, chat, moderasi, larangan sesi, dan presence. Ini menghindari tabel paralel
  serta pengecekan `user_id OR guest_id` di setiap jalur. Tamu tidak mendapat email, password,
  Google identity, membership organisasi, atau refresh token akun biasa.
- **Grant tamu terikat satu ruang.** Grant menyimpan tautan, ruang, organisasi, nama tamu, status,
  persetujuan host, masa berlaku, dan pencabutan. Grant adalah sumber hak akses tamu.
- **Token tamu terpisah dari token akun.** Claim/audience berbeda, masa hidup pendek, dan hanya
  diterima endpoint tamu, WebSocket ruang tujuan, serta token suara ruang itu. Semua API akun tetap
  menolak token tamu.
- **Host adalah guru/admin sekolah yang sama.** Persetujuan, penolakan, dan pengeluaran dicatat di
  audit moderasi.
- **Admission diperiksa ulang setiap koneksi dan reconnect.** Ruang arsip, grant dicabut, grant habis,
  atau larangan sesi segera menutup akses.

## Keputusan produk disetujui

1. **Usia opsional.** Tamu tidak wajib memberi tanggal lahir atau usia, dan usia tidak dipakai untuk menolak
   atau mengizinkan masuk. G1 tidak menyimpan data usia tamu. Host/sekolah bertanggung jawab memastikan undangan
   tamu sesuai aturan sekolah serta persetujuan orang tua/wali bila diperlukan.
2. **Batas waktu tautan.** Maksimal 24 jam.
3. **Batas tamu.** Maksimal 40 tamu per tautan dan tetap dibatasi kapasitas ruang.
4. **Suara.** Tamu boleh mendengar dan bicara sesuai kontrol ruang biasa; host dapat mute atau mengeluarkan.

Risiko terbuka: akses tamu tanpa verifikasi usia dapat dipakai anak. Tinjau kebutuhan PDP dan persetujuan
orang tua/wali dengan konsultan hukum sebelum pilot.

## Batas keamanan

- Kode tautan acak dari `crypto/rand`, dapat dicabut, berbatas waktu, dan pratinjau tidak membocorkan
  data selain nama sekolah dan ruang.
- Rate limit pratinjau, permintaan masuk, dan polling persetujuan.
- Nama tamu divalidasi dan dinormalisasi seperti nama tampilan akun.
- Tamu tidak bisa mengambil roster, daftar ruang, profil anggota, berkas, atau data pembelajaran.
- Pesan realtime tamu tetap memakai batas pesan, moderasi, dan filter ruang yang sudah ada.
- Pindah ruang (`travel`) ditolak untuk tamu.
- Token tamu tidak disimpan di URL setelah penukaran; respons memakai `Cache-Control: no-store`.
- Data tamu mengikuti retensi chat/audit yang sudah berlaku. Ketentuan PDP perlu ditinjau sebelum pilot.

## Rencana slice

### G1a — Kontrak dan data

- Tambah ADR karena tamu adalah prinsipal baru di luar keanggotaan organisasi.
- Migration berikutnya: jenis akun tamu dan tabel grant/link tamu.
- OpenAPI untuk membuat, melihat, dan mencabut tautan tamu; pratinjau; permintaan masuk; status
  menunggu; persetujuan atau penolakan host.
- Test real-Postgres: kode habis/dicabut, kuota, permintaan berulang, persetujuan serentak, dan akses
  lintas sekolah.

### G1b — Token dan admission realtime

- Tambah token tamu terpisah dan pastikan middleware API akun menolaknya.
- Izinkan WebSocket tamu langsung ke ruang tujuan tanpa melewati peta sekolah.
- Tolak `travel`, akses ruang lain, dan reconnect ke grant yang sudah tidak berlaku.
- Integrasikan token suara khusus ruang tujuan.
- Test unit/integration untuk autentikasi, admission, moderasi, dan pencabutan.

### G1c — Pengalaman host

- Tambah **Undang tamu** di detail ruang atau kontrol sesi.
- Tambah antrean permintaan masuk di panel sesi host dengan **Izinkan** dan **Tolak**.
- Tampilkan label **Tamu** di daftar orang dan chat.
- Ikuti aturan kontrol ringkas, garis `--kv-line`, tanpa kartu bersarang, dan aksesibilitas keyboard.

### G1d — Pengalaman tamu

- Tambah halaman `/guest/{code}` di luar alur login.
- Form nama, pernyataan usia, status menunggu, ditolak, habis, dan dikeluarkan.
- Ruang tamu memakai shell minimal: Peta/Chat atau mode HP tanpa menu Kelola, Rapor, Belajar,
  Perpustakaan, berkas, atau navigasi ke ruang lain.
- E2E: host membuat tautan, tamu meminta masuk, host mengizinkan, tamu chat di ruang tujuan, host
  mengeluarkan, dan tamu tidak dapat masuk kembali ke sesi itu.

## Kriteria selesai

- Tamu dapat mengikuti satu pertemuan tanpa mendaftar.
- Tamu tidak dapat mengakses ruang lain atau data sekolah apa pun.
- Akun dan alur sekolah yang sudah ada tidak berubah.
- Pencabutan tautan, kedaluwarsa, penolakan, dan pengeluaran bekerja di HTTP, WebSocket, dan suara.
- Backend race test, frontend test, lint, build, e2e desktop, dan e2e lebar HP lulus.

## Implementasi dan bukti

G1a–G1d selesai. Review keamanan independen menemukan race queued welcome setelah revoke;
fix menutup socket langsung sebelum write loop bila admission akhir gagal. Tes RED membuktikan
kebocoran welcome, lalu GREEN dengan race detector; re-review tidak menemukan blocker.

- Migration `00019_guest_access.sql`: identitas tamu tanpa email/kredensial, link/grant dan audit
  `guest_decisions`. Constraint/FK/trigger menjaga pemisahan akun; rollback menolak menghapus tamu.
- Retry permintaan memakai hash UUID v4 rahasia, tidak memakai slot baru. Keputusan dan pencabutan
  atomik dengan audit. Guru/admin lintas sekolah tidak mendapat akses.
- JWT audience terpisah, maksimal 15 menit; polling memperbarui token. Admission cek grant setiap
  koneksi; `travel`, sematan materi, data belajar, dan API akun tidak tersedia bagi tamu.
- Host: **Kontrol pengajar → Undang tamu**. Buat/salin/cabut tautan dan antrean Izinkan/Tolak.
  Batas default 2 jam/40 tamu; slot adalah total identitas yang meminta, bukan jumlah online.
- Tamu: `/guest/{code}`, nama saja, menunggu/ditolak/habis/dikeluarkan, shell ruang minimal.
  Desktop peta/chat/voice; HP daftar tempat/chat. Kredensial hanya in-memory; reload membuat
  permintaan baru. Tidak ada pemulihan lintas reload atau konversi akun pada G1.
- Tes PostgreSQL: replay serentak, kuota, keputusan serentak, grant pending/approved/denied,
  revoke/expiry, audit, kredensial akun ditolak, serta approval bersamaan dengan revoke.
  Unit gateway membuktikan admission akhir gagal tidak mengirim welcome/roster. Unit realtime menguji expiry/revoke termasuk
  held presence dan voice, moderasi dan isolasi travel/materi.
- Verifikasi: `go test -race -count=1 ./...` dengan PostgreSQL, `make lint`, frontend
  `npm run test:unit -- --run`, `npm run lint`, `npm run build` lulus. Package `guest` mencapai
  83,2% statement coverage dengan lifecycle service dan HTTP.
- E2E `/tmp/kv-e2e/g1-guest.mjs`: desktop 1440×1000 dan HP 390×844 lulus. Host buat link,
  tamu menunggu tanpa isi ruang, approval, chat, badge Tamu, mute/kick, penolakan reentry,
  API akun ditolak, voice desktop terhubung dan grant dicabut; link dicabut memberi status habis.
  Tidak ada page error atau overflow horizontal. Skrip masih harness lokal, belum CI.

Batas pilot tetap: tinjauan PDP/persetujuan wali, Safari/iOS/perangkat fisik, jaringan produksi,
soak dan verifikasi kapasitas global. Migrasi route lama ke English dikerjakan terpisah setelah G1.

# P1 — Beban gerak realtime

Basis: commit input peta `e943ac7`; branch `perf-realtime-spatial`. Tidak ada push otomatis.

## Prioritas dan batas

1. Grid spasial memakai aturan AOI existing (sel 8 tile, radius 3 sel), tanpa mengubah siapa yang terlihat.
2. Hindari loop snapshot saat tidak ada posisi/membership yang berubah; cache konversi UUID per tick,
   alokasikan daftar posisi dengan kapasitas awal. Kontrak JSON, otorisasi dan rate limit tetap.
3. Pakai kapasitas ruang existing **40**, kampus **200**. Tidak mengubah DB menjadi 500 atau otomatis
   memecah kampus; pembagian instance/penyembunyian avatar perlu keputusan produk terpisah.
4. Eksperimen 5 Hz hanya lokal, kemudian dikembalikan ke 10 Hz. Belum dipromosikan tanpa uji visual.
5. Benchmark sebelum/sesudah, oracle all-pairs, test race, serta opt-in 500 WebSocket loopback.

## Implementasi

- [x] `snapshot.go`: grid direbuild saat ada perubahan, dalam room actor tanpa goroutine/mutex baru.
  Receiver memeriksa sel sekitar dan daftar `seen` untuk `out`. Held avatar tetap terlihat; reconnect
  memicu sinkronisasi. Ruang interior tetap full-room (kapasitas kecil).
- [x] Snapshot idle berhenti setelah visibility tersinkronisasi; join/reconnect ditandai belum sinkron.
- [x] Profil alokasi menunjukkan `uuid.UUID.MarshalText` dominan. String UUID dibentuk sekali per tick,
  bukan per penerima; wire format tidak berubah. Percobaan `json.RawMessage` ditolak karena lebih lambat.
- [x] Oracle membandingkan snapshot dengan aturan all-pairs selama perpindahan sel dan reset visibility.
  Test existing mencakup AOI, held/reconnect, travel, gather, voice, kapasitas dan slow consumer.
- [x] Harness opt-in: 500 socket nyata, auth/admission fixture, 30 detik semua bergerak bolak-balik 8 px,
  reader aktif, hitung JSON throughput, error, dan ping round-trip. Bukan uji produksi/DB/voice/TLS.

## Benchmark awal

Linux amd64, Intel i5-13420H, Go project toolchain; 6 pengulangan per varian, 500 ms per kasus.
Median tick termasuk pembuatan JSON dan drain antrean, bukan transport. Fixture sebar memakai koordinat
seluruh peta produksi (termasuk tile terhalang); ini menguji snapshot, bukan validitas berjalan.

| Kasus | Sebelum | Sesudah | Perubahan |
| --- | ---: | ---: | ---: |
| Kampus 500 diam | 15,07 ms | 0,026 ms | sekitar −99,8% |
| Kampus 500 bergerak tersebar | 85,41 ms | 58,06 ms | sekitar −32% |
| Kampus 500 bergerak berkumpul | 129,18 ms | 101,16 ms | sekitar −22% |
| Interior 50 bergerak (stress di atas kapasitas normal) | 1,55 ms | 1,11 ms | sekitar −28% |

Alokasi/tick 500 berkumpul turun sekitar 65,2 MB menjadi 33,1 MB; masih besar. Grid saja tidak mengurangi
volume saat semua saling terlihat. Batas 100 ms/tick masih terlampaui pada kasus terpadat.

## Hasil dan perintah reproduksi

```sh
cd classverse-backend
go test -race ./internal/realtime
go test ./internal/realtime -run '^$' -bench '^BenchmarkRoomTick$' -benchmem -benchtime=500ms -count=6
KV_LOAD_TEST=1 go test ./internal/realtime -run '^TestWebSocketLoad500$' -v -count=1 -timeout=180s
```

Uji transport 10 Hz awal (30 detik, satu mesin server+client, auth/admission fixture):
- 500 dalam 1 kampus (kapasitas dinaikkan **hanya fixture**): 86,92 MB/s JSON diterima, 3.748 snapshot/s,
  ping p95 206 ms/p99 226 ms; nol disconnect/write error/rejected command. Pengiriman di bawah 5.000
  snapshot/s menunjukkan tekanan beban; **bukan bukti 500 satu area siap produksi**.
- 500 dalam 10 interior ×50 (stress di atas kapasitas normal): 19,02 MB/s, 4.982 snapshot/s,
  ping p95 19,2 ms/p99 26,1 ms; nol error.
- Eksperimen 5 Hz: 10×50 turun ke 9,55 MB/s; satu kampus tetap 90,93 MB/s karena loopback 10 Hz
  sebelumnya tertahan beban dan 5 Hz mengirim lebih banyak posisi per snapshot. Angka tidak boleh
  ditafsirkan sebagai bandwidth umum yang selalu turun setengah. `tickInterval` dikembalikan 100 ms.

Harness final memakai **13 interior berkapasitas 40** (38–39 peserta/ruang), bukan 10×50.
Hasil final 30 detik, 10 Hz: **14,59 MB/s**, 4.982 snapshot/s, ping p50 4,55 ms/p95 11,89 ms/
p99 18,63 ms, nol disconnect/write error/rejected command. Kampus tunggal final: 86,53 MB/s,
3.740 snapshot/s, ping p95 171 ms/p99 206 ms, nol error. Ini latency ping, bukan latency posisi.

Seluruh test backend `-race` dengan Postgres/AIStor, lint, oracle snapshot dan harness lulus.
Frontend/protokol tidak diubah dalam slice ini; validasi browser/voice produksi tidak dijalankan ulang.

Belum: soak berjam-jam, VPS target, TLS/internet, DB/auth asli, renderer 500 avatar, voice LiveKit,
monitoring produksi, pemisahan kampus dan kebijakan kerumunan. Tidak menambah Redis/Kafka/microservice.

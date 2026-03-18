# Roadmap Video Downloader (Clean)

_Last update: 2026-03-18_

## 1) Tujuan Produk

Bangun web downloader yang **cepat, stabil, dan aman** untuk flow utama:
1. User paste URL YouTube
2. System resolve metadata + format
3. User pilih format
4. Download berjalan sukses

> Scope aktif saat ini: **YouTube-first (MVP)**.

---

## 2) Stack & Arsitektur

- **Frontend**: Next.js 14 (App Router), Tailwind, Zustand
- **Backend**: Go + yt-dlp + FFmpeg
- **Queue**: Redis + Asynq
- **Data**: PostgreSQL (jobs/errors)
- **Artifact**: Cloudflare R2 (untuk MP3)
- **Deploy**: Ubuntu VPS + systemd

---

## 3) Status Saat Ini (Ringkas)

### ✅ Done

- [x] Runtime backend hardening (`yt-dlp` via PATH + startup validation)
- [x] Upgrade runtime `yt-dlp` host ke versi terbaru
- [x] API internal-only bind (`127.0.0.1:18080`)
- [x] Next.js internal proxy `/api/* -> backend internal`
- [x] Home flow **real resolve + MP4 download** (Step C)
- [x] API endpoint MVP sudah tersedia:
  - `/healthz`
  - `/v1/youtube/resolve`
  - `/v1/download/mp4`
  - `/v1/jobs/mp3`
  - `/v1/jobs/:id`
  - `/admin/jobs`
- [x] Deploy foundation script: `deploy.sh` (pull, build, restart, smoke)

### ⏳ In Progress / Pending

- [ ] Step D: MP3 pipeline end-to-end production
  - [ ] `ytd-worker.service` aktif permanen
  - [ ] R2 env real (`R2_ENDPOINT`, key, secret, bucket)
  - [ ] Smoke test MP3 success path
- [ ] `/history` masih mock data
- [ ] `/settings` masih mock data

### ⚠️ Known Gap

- Worker saat ini belum aktif (service inactive), jadi flow MP3 belum siap produksi penuh.

---

## 4) Prioritas Eksekusi

### Phase 0 — Foundation (DONE)

- [x] Repo + build backend/web stabil
- [x] Service web/api jalan di user systemd
- [x] API proxy internal aman untuk publik
- [x] Deploy script baseline

### Phase 1 — MVP Core (NOW)

### P1.1 MP3 End-to-End (Step D)
- [ ] Aktifkan `ytd-worker.service` (enable + restart policy)
- [ ] Isi config R2 real + validasi upload object
- [ ] Validasi flow:
  - [ ] `POST /v1/jobs/mp3` accepted
  - [ ] status `queued -> processing -> done`
  - [ ] `download_url` valid

### P1.2 Data Real di UI
- [ ] `/history` konsumsi data job real dari backend
- [ ] state loading/error/empty di history
- [ ] filter basic (status, date/search)

### P1.3 Settings Minimum Useful
- [ ] Ubah `/settings` dari mock ke preference nyata minimum
- [ ] Simpan preference sederhana (localStorage dulu)

### Phase 2 — Hardening Produksi

- [ ] Tambah observability minimum (log context + failure reason yang actionable)
- [ ] Smarter smoke (include one resolve + one MP3 test bila env lengkap)
- [ ] Backup/recovery notes (DB + env + service)
- [ ] Optional ingress production: domain + HTTPS (non-quick tunnel)

### Phase 3 — Expansion (Setelah MVP Stabil)

- [ ] Trim video YouTube (FFmpeg)
- [ ] Real-time progress UX (poll/SSE)
- [ ] Support platform tambahan (TikTok/IG/X) bertahap

---

## 5) Backlog Fitur (Dirapikan)

### Near-term (nilai cepat)
- [ ] Batch download sederhana
- [ ] Better error messaging (link private/invalid/restricted)
- [ ] Download retry UX

### Mid-term
- [ ] PWA install prompt
- [ ] QR transfer desktop -> mobile
- [ ] Subtitle extract / Video to GIF

### Long-term
- [ ] AI highlights / transcript
- [ ] Vocal-BGM splitter
- [ ] Cloud sync (Drive/Dropbox)
- [ ] Premium/token-gated features

---

## 6) Definisi "MVP Ready"

MVP dianggap ready jika semua ini true:
- [x] User bisa resolve URL YouTube dari Home
- [x] User bisa download MP4 dari pilihan format
- [ ] User bisa generate MP3 sampai dapat link unduh valid
- [ ] History tampil data real (bukan sample)
- [x] API tidak terekspos publik langsung (lewat proxy internal)
- [x] Deploy repeatable via script (`deploy.sh`)

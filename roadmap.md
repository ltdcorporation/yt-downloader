# Roadmap Video Downloader

_Last update: 2026-03-18_

> Struktur ini sengaja dipisah: **Frontend full di atas**, **Backend full di bawah**.

---

## 1) Frontend Roadmap (Section Atas)

### A. Status Frontend Saat Ini

- [x] Next.js app + basic pages (`/`, `/history`, `/settings`, `/admin`)
- [x] Home flow **real** untuk YouTube resolve + MP4 download (Step C)
- [x] Frontend pakai internal proxy `/api/*` (lebih aman buat publik)
- [ ] `/history` masih mock data
- [ ] `/settings` masih mock data
- [ ] MP3 flow UI (create job + polling status) belum lengkap di halaman utama

### B. Milestone FE-1 — Home Core Flow (MVP)

**Target:** user paste URL -> lihat opsi -> download MP4 sukses.

- [x] Input URL + paste clipboard
- [x] Call `POST /api/v1/youtube/resolve`
- [x] Render metadata (title, thumbnail, duration)
- [x] Render list format MP4
- [x] Download MP4 via `GET /api/v1/download/mp4`
- [ ] Tambah UX state yang lebih rapih (retry CTA, inline troubleshooting)
- [ ] Tambah fallback kalau `window.open` diblok browser (UX copy lebih jelas)

### C. Milestone FE-2 — MP3 UX di Home

**Target:** user bisa request MP3 dari UI, lihat progress, dan unduh saat done.

- [ ] Tombol/opsi “Convert to MP3” di modal/home
- [ ] Call `POST /api/v1/jobs/mp3`
- [ ] Poll `GET /api/v1/jobs/:id`
- [ ] Status UI: queued -> processing -> done/failed
- [ ] Tampilkan `download_url` saat done

### D. Milestone FE-3 — History (Data Real)

**Target:** `/history` tidak lagi sample statis.

- [ ] Ambil data job real dari backend
- [ ] Loading, empty-state, error-state yang jelas
- [ ] Filter dasar (status + search)
- [ ] Aksi relevan (download again untuk job done)

### E. Milestone FE-4 — Settings (Minimum Useful)

**Target:** `/settings` jadi kepake, bukan mock doang.

- [ ] Simpan preference sederhana (localStorage dulu)
- [ ] Sinkronkan preference dengan behavior UI yang relevan
- [ ] UX tombol save/discard yang benar-benar ngaruh

### F. Frontend Quality Checklist

- [ ] Error message human-readable (bukan raw error backend)
- [ ] State konsisten (loading/disabled/success/error)
- [ ] Mobile-first tetap enak di viewport kecil
- [ ] Tidak ada mock data tersisa di flow MVP

---

## 2) Backend Roadmap (Section Bawah)

### A. Status Backend Saat Ini

- [x] Runtime hardening `yt-dlp` (resolve via PATH + startup validation)
- [x] Host runtime `yt-dlp` sudah versi terbaru (`/usr/local/bin/yt-dlp`)
- [x] API bind internal-only `127.0.0.1:18080`
- [x] Endpoint MVP aktif:
  - [x] `GET /healthz`
  - [x] `POST /v1/youtube/resolve`
  - [x] `GET /v1/download/mp4`
  - [x] `POST /v1/jobs/mp3`
  - [x] `GET /v1/jobs/:id`
  - [x] `GET /admin/jobs`
- [x] Jobs store via PostgreSQL (fallback Redis saat DSN kosong)
- [x] Worker service aktif permanen (`ytd-worker.service` enabled + running)
- [x] R2 sudah diisi config real
- [x] Prefix object storage configurable via `R2_KEY_PREFIX` (aktif: `yt-downloader/prod`)
- [x] MP3 pipeline sudah lolos validasi end-to-end produksi

### B. Milestone BE-1 — Runtime & API Hardening (Done)

- [x] `YTDLP_BINARY` default jadi `yt-dlp` (no hardcoded path tunggal)
- [x] API/worker fail-fast check binary runtime saat start
- [x] Proxy web internal sudah siap (`/api/*`)
- [x] API tetap tidak diekspos publik langsung

### C. Milestone BE-2 — MP3 End-to-End (Done)

**Target:** flow MP3 benar-benar jalan dari request sampai link download valid.

- [x] Enable + run `ytd-worker.service` stabil (auto restart)
- [x] Isi env R2 real:
  - [x] `R2_ENDPOINT`
  - [x] `R2_BUCKET`
  - [x] `R2_KEY_PREFIX`
  - [x] `R2_ACCESS_KEY_ID`
  - [x] `R2_SECRET_ACCESS_KEY`
- [x] Verifikasi flow:
  - [x] `POST /v1/jobs/mp3` -> accepted
  - [x] job transisi `queued -> processing -> done`
  - [x] `download_url` bisa diakses
- [x] Tambah smoke test MP3 ke alur deploy (conditional saat env lengkap)

### D. Milestone BE-3 — Deploy & Operasional

- [x] `deploy.sh` pondasi (pull, build, restart, smoke)
- [x] `scripts/test-backend.sh` full backend suite (unit + Redis + Postgres integration, dengan preflight dependency check)
- [ ] Tambah observability minimum (log reason yang actionable)
- [ ] Tambah runbook backup/recovery (DB + env + service)
- [ ] Ingress production final (domain + HTTPS) saat siap publik luas

### E. Backend Quality Checklist

- [x] Full-suite backend test runner tersedia (`make backend-test` / `scripts/test-backend.sh`) dengan coverage output
- [ ] Semua dependency runtime tervalidasi saat startup
- [ ] Error backend konsisten & aman ditampilkan ke frontend
- [ ] Worker tidak silent-fail
- [ ] Retention/cleanup jobs tetap terkendali

---

## 3) MVP Ready Gate (Gabungan FE + BE)

MVP dianggap siap kalau semua checklist ini true:

### Frontend Gate
- [x] User bisa resolve URL YouTube dari Home
- [x] User bisa download MP4 dari pilihan format
- [ ] User bisa request MP3 + lihat progress + unduh hasil
- [ ] History pakai data real (bukan sample)

### Backend Gate
- [x] API tidak terekspos publik langsung (internal-only + proxy)
- [x] Deploy repeatable via script (`deploy.sh`)
- [x] Worker aktif stabil
- [x] R2 aktif dan MP3 end-to-end lulus test

### Security/Operational Gate
- [x] Web publik, API private internal
- [ ] Monitoring + alert minimum tersedia
- [ ] Dokumen troubleshooting dasar tersedia

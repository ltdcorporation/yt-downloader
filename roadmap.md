# Roadmap Video Downloader

_Last update: 2026-03-26 (Backend roadmap refresh: YouTube heatmap + video-cut enterprise plan)_

> Struktur ini sengaja dipisah: **Frontend full di atas**, **Backend full di bawah**.
>
> Referensi desain enterprise history backend: `docs/history-backend-enterprise-rfc.md`.

---

## 1) Frontend Roadmap (Section Atas)

### A. Status Frontend Saat Ini

- [x] Next.js app + basic pages (`/`, `/history`, `/settings`, `/admin`)
- [x] Home flow **real** dengan satu input dinamis untuk YouTube/X/Instagram/TikTok
- [x] Input bar auto-detect platform (ikon + placeholder + resolver endpoint)
- [x] Frontend pakai internal proxy `/api/*` (lebih aman buat publik)
- [x] UI picker kualitas MP4 aktif di modal download (YouTube/X/Instagram/TikTok)
- [x] UX warning HLS-only untuk X/Instagram/TikTok sudah ada (by design belum didukung)
- [x] MP3 flow UI aktif di home (create job + polling + state done/failed + download)
- [x] `/history` sudah terhubung ke API real (list/stats/redownload/delete)
- [x] `/settings` sudah terhubung ke API real (load + patch + save/discard)
- [x] Avatar user real sudah terpadu di UI (navbar + settings), fallback ke static default image
- [ ] Logging analytics event resolve success/fail per platform belum ditambahkan
- [ ] Fallback UX khusus untuk kasus auto-download diblok browser belum dipoles penuh

### B. Milestone FE-1 — Home Core Flow (MVP)

**Target:** user paste URL -> auto-detect platform -> lihat opsi -> download MP4 sukses.

- [x] Input URL + paste clipboard
- [x] Auto route resolver ke endpoint sesuai platform (`/v1/{platform}/resolve`)
- [x] Render metadata (title, thumbnail, duration)
- [x] Render list format MP4
- [x] Download MP4 via `GET /api/v1/download/mp4`
- [x] Tambah UX state lebih rapi (retry CTA, inline troubleshooting)
- [ ] Tambah fallback copy/CTA yang lebih eksplisit kalau auto-download diblok browser

### C. Milestone FE-2 — MP3 UX di Home

**Target:** user bisa request MP3 dari UI, lihat progress, dan unduh saat done.

- [x] Tombol/opsi “Convert to MP3” di modal/home
- [x] Call `POST /api/v1/jobs/mp3`
- [x] Poll `GET /api/v1/jobs/:id`
- [x] Status UI: queued -> processing -> done/failed
- [x] Tampilkan `download_url` saat done
- [ ] Ekstensi MP3 untuk platform non-YouTube belum dikerjakan (masih YouTube-only by design)

### D. Milestone FE-3 — History (Data Real)

**Target:** `/history` tidak lagi sample statis.

- [x] Ambil data real dari backend (`GET /v1/history` + cursor pagination)
- [x] Loading, empty-state, error-state jelas
- [x] Filter dasar (platform + search)
- [x] Aksi relevan (download again + delete)
- [x] Stats card dari backend (`GET /v1/history/stats`)
- [x] Redownload queued MP3 dipoll sampai siap lalu auto-trigger download

### E. Milestone FE-4 — Settings (Minimum Useful)

**Target:** `/settings` benar-benar kepake (bukan mock).

- [x] Load snapshot profile + settings dari backend (`GET /v1/profile`, `GET /v1/settings`)
- [x] Save patch profile + settings ke backend (`PATCH /v1/profile`, `PATCH /v1/settings`)
- [x] UX save/discard + dirty-state guard jalan
- [x] Handle conflict notice (`settings_version_conflict`) di UI
- [ ] Fallback offline/local-only mode belum dikerjakan
- [x] Avatar upload/remove terhubung ke backend (`POST/DELETE /v1/profile/avatar`)

### F. Milestone FE-5 — X/Twitter Flow di Home (Done Baseline, Hardening Lanjut)

**Target:** user paste link X/Twitter -> resolve -> pilih kualitas -> download MP4.

**Status saat ini:** baseline flow sudah aktif end-to-end di FE.

- [x] Auto-detect URL X/Twitter dari input tunggal
- [x] Call `POST /api/v1/x/resolve`
- [x] Render metadata + format list response resolver X
- [x] UI picker kualitas MP4 khusus X (list/card per format + size jika tersedia)
- [x] CTA download MP4 per kualitas hasil resolver X
- [x] UX pemilihan kualitas sudah konsisten dengan flow utama
- [x] Handle backend error code `x_hls_only_not_supported` -> warning human-friendly
- [x] State “belum support HLS-only” + CTA retry/fallback tampil di UI
- [ ] Error UX sangat spesifik untuk semua varian X (live/restricted/edge lainnya) masih bisa dipoles
- [ ] Logging event ringan success/fail resolve X belum ada

### G. Milestone FE-6 — Instagram Flow di Home (Done Baseline, Hardening Lanjut)

**Target:** user paste link Instagram -> resolve -> pilih kualitas -> download MP4.

**Status saat ini:** baseline flow sudah aktif end-to-end di FE.

- [x] Auto-detect URL Instagram dari input tunggal
- [x] Call `POST /api/v1/instagram/resolve`
- [x] Render metadata + format list dari resolver Instagram
- [x] UI picker kualitas MP4 khusus Instagram
- [x] CTA download MP4 per kualitas resolver Instagram
- [x] UX konsisten dengan flow platform lain
- [x] Handle backend error code `ig_hls_only_not_supported` -> warning human-friendly
- [x] State “belum support HLS-only” + CTA fallback tampil saat code muncul
- [ ] Logging event ringan success/fail resolve Instagram belum ada

### H. Milestone FE-7 — TikTok Flow di Home (Done Baseline, Hardening Lanjut)

**Target:** user paste link TikTok -> resolve -> pilih kualitas -> download MP4.

**Status saat ini:** baseline flow sudah aktif end-to-end di FE.

- [x] Auto-detect URL TikTok dari input tunggal
- [x] Call `POST /api/v1/tiktok/resolve`
- [x] Render metadata + format list dari resolver TikTok
- [x] UI picker kualitas MP4 khusus TikTok
- [x] CTA download MP4 per kualitas resolver TikTok
- [x] UX konsisten dengan flow platform lain
- [x] Handle backend error code `tt_hls_only_not_supported` -> warning human-friendly
- [x] State “belum support HLS-only” + CTA fallback tampil saat code muncul
- [ ] Logging event ringan success/fail resolve TikTok belum ada

### I. Frontend Quality Checklist

- [x] Error message mayoritas human-readable (termasuk mapping HLS-only)
- [ ] Normalisasi seluruh error backend edge-case ke copy non-teknis belum 100%
- [x] State utama konsisten (loading/disabled/success/error)
- [x] Mobile-first tetap layak untuk viewport kecil
- [x] Tidak ada mock data tersisa di flow MVP
- [x] Surface user utama tidak lagi pakai avatar huruf-only placeholder

---

## 2) Backend Roadmap (Section Bawah)

### A. Status Backend Saat Ini

- [x] Runtime hardening `yt-dlp` (resolve via PATH + startup validation)
- [x] Host runtime `yt-dlp` sudah versi terbaru (`/usr/local/bin/yt-dlp`)
- [x] API bind internal-only `127.0.0.1:18080`
- [x] Endpoint MVP aktif:
  - [x] `GET /healthz`
  - [x] `GET /v1/profile`
  - [x] `PATCH /v1/profile`
  - [x] `POST /v1/profile/avatar`
  - [x] `DELETE /v1/profile/avatar`
  - [x] `GET /v1/settings`
  - [x] `PATCH /v1/settings`
  - [x] `POST /v1/youtube/resolve`
  - [x] `POST /v1/x/resolve`
  - [x] `POST /v1/instagram/resolve` (alias: `POST /v1/ig/resolve`)
  - [x] `POST /v1/tiktok/resolve` (alias: `POST /v1/tt/resolve`)
  - [x] `GET /v1/download/mp4`
  - [x] `POST /v1/jobs/mp3`
  - [x] `GET /v1/jobs/:id`
  - [x] `GET /admin/jobs`
  - [x] `POST /v1/history` (create/resolve capture)
- [x] History backend enterprise **Phase 1** selesai:
  - [x] Store layer `internal/history` (memory + postgres)
  - [x] Auto schema bootstrap tabel `history_items` + `history_attempts`
  - [x] Unit + integration tests untuk store layer
- [x] History backend enterprise **Phase 2** selesai:
  - [x] Write-path authenticated di `POST /v1/jobs/mp3` (create queued attempt)
  - [x] Worker sinkron status history attempt by `job_id` (`processing/done/failed`)
  - [x] Write-path authenticated di `GET /v1/download/mp4` (processing -> done/failed)
- [x] History backend enterprise **Phase 3** selesai:
  - [x] Read API `GET /v1/history` (cursor pagination + filter platform/status/query)
  - [x] Stats API `GET /v1/history/stats`
  - [x] Action API `POST /v1/history/{id}/redownload` (mode direct/queued)
  - [x] Delete API `DELETE /v1/history/{id}` (soft delete owner-scoped)
  - [x] Test suite history handlers + store query path
- [x] History backend enterprise **Phase 4 (Resolve Capture API)** baseline masuk:
  - [x] Endpoint `POST /v1/history` (auth required) untuk simpan event resolve awal
  - [x] Enum/filter status history sudah mencakup `resolved`
  - [x] FE home flow kirim create-history saat resolve (best-effort, non-blocking)
- [ ] History backend enterprise **Phase 4 hardening** (next):
  - [ ] Sinkronkan constraint schema Postgres agar status `resolved` diterima native
  - [ ] Tambah integration test Postgres untuk jalur `POST /v1/history`
- [x] Settings/Profile backend enterprise baseline selesai:
  - [x] Store layer `internal/settings` (memory + postgres)
  - [x] Auto schema bootstrap tabel `user_settings` + `user_settings_audit`
  - [x] Optimistic concurrency (`meta.version`) aktif di `PATCH /v1/settings`
  - [x] Owner-scoped endpoint via session identity (`GET/PATCH /v1/profile`, `GET/PATCH /v1/settings`)
  - [x] Audit write untuk perubahan settings efektif (changed_fields)
- [x] Avatar backend enterprise baseline selesai:
  - [x] `auth_users.avatar_url` aktif (memory + postgres + schema backfill `ALTER TABLE ... IF NOT EXISTS`)
  - [x] Endpoint auth-required aktif: `POST /v1/profile/avatar`, `DELETE /v1/profile/avatar`
  - [x] Image normalization pipeline: center-crop + resize `512x512` + encode WebP
  - [x] R2 public-host delivery aktif (`AVATAR_PUBLIC_BASE_URL` + `AVATAR_R2_KEY_PREFIX`)
  - [x] Strict replace semantics aktif (delete avatar lama wajib sukses, rollback bila gagal)
- [ ] Settings/Profile hardening (next):
  - [ ] Tambah integration test Postgres untuk `internal/settings` (CRUD + conflict + audit)
  - [ ] Pindahkan schema rollout ke migration explicit (hindari runtime DDL drift)
  - [ ] Tegaskan kontrak payload null-field (saat ini null diperlakukan sebagai field omitted)
  - [ ] Phase-2 identity flow untuk perubahan email terverifikasi
- [ ] Avatar hardening (next):
  - [ ] Tambah virus/malware scanning hook sebelum persist object (opsional compliance mode)
  - [ ] Tambah endpoint rotate/default-avatar preset library (opsional UX)
  - [ ] Tambah observability metrics (`avatar_upload_total`, `avatar_delete_fail_total`)
- [x] Jobs store via PostgreSQL (fallback Redis saat DSN kosong)
- [x] Worker service aktif permanen (`ytd-worker.service` enabled + running)
- [x] R2 sudah diisi config real
- [x] Prefix object storage configurable via `R2_KEY_PREFIX` (aktif: `yt-downloader/prod`)
- [x] MP3 pipeline sudah lolos validasi end-to-end produksi
- [x] Resolver X/Twitter aktif untuk URL status (`/{user}/status/{id}` + `/i/status/{id}`)
- [x] Multi-cookie fallback aktif untuk resolver X (rotasi akun sampai resolve sukses)
- [x] Env runtime X resolver sudah aktif:
  - [x] `X_MAX_QUALITY`
  - [x] `X_COOKIES_DIR`
  - [x] `X_COOKIES_FILES`
  - [x] `X_RESOLVE_TRY_WITHOUT_COOKIES`
- [x] Cookie account X diekstrak dari `automation/postx/runtime/chromium-profiles` ke runtime `yt-downloader/runtime/x-cookies` (8 profile)
- [x] Patch selector format X sudah masuk: fallback direct `http-*` MP4 untuk kasus metadata codec kosong (false-negative fix)
- [x] Resolver Instagram aktif untuk URL `reel/reels/p/tv`
- [x] Multi-cookie fallback aktif untuk resolver Instagram (rotasi akun sampai resolve sukses)
- [x] Env runtime Instagram resolver sudah ditambah:
  - [x] `IG_MAX_QUALITY`
  - [x] `IG_COOKIES_DIR`
  - [x] `IG_COOKIES_FILES`
  - [x] `IG_RESOLVE_TRY_WITHOUT_COOKIES`
- [x] Error code typed untuk Instagram HLS-only sudah ada: `ig_hls_only_not_supported`
- [x] Resolver TikTok aktif untuk URL `@user/video`, `t`, `vm.tiktok.com`, `vt.tiktok.com`
- [x] Multi-cookie fallback aktif untuk resolver TikTok (rotasi akun sampai resolve sukses)
- [x] Env runtime TikTok resolver sudah ditambah:
  - [x] `TT_MAX_QUALITY`
  - [x] `TT_COOKIES_DIR`
  - [x] `TT_COOKIES_FILES`
  - [x] `TT_RESOLVE_TRY_WITHOUT_COOKIES`
- [x] Error code typed untuk TikTok HLS-only sudah ada: `tt_hls_only_not_supported`
- [x] UI trim + heatmap untuk YouTube sudah tersedia di modal frontend (`DownloadModal`)
- [ ] Contract backend YouTube untuk `heatmap` + `key_moments` belum diekspos
- [ ] Endpoint queue video cut (`POST /v1/jobs/video-cut`) belum tersedia
- [ ] Worker ffmpeg untuk proses cut + upload hasil ke R2 belum tersedia
- [ ] Feature flag + observability khusus heatmap/video-cut belum tersedia

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

### E. Milestone BE-4 — X Resolver + Multi-Cookies (Done, Hardening Lanjut)

**Target:** resolve link X/Twitter non-live secara stabil, termasuk konten yang butuh sesi akun.

- [x] Tambah endpoint `POST /v1/x/resolve`
- [x] Validasi host/path X/Twitter yang didukung
- [x] Rotasi cookie profile dari file list + directory scan
- [x] Response menyertakan `cookie_profile` yang berhasil dipakai
- [x] Policy explicit: live content ditolak
- [x] Fix false-negative selector format:
  - [x] progressive MP4 tetap prioritas
  - [x] direct `http-*` MP4 diterima sebagai fallback saat metadata codec kosong
- [x] Hardening test:
  - [x] edge cases resolver + endpoint
  - [x] coverage `internal/xresolver` stabil di ~95%
- [x] Deploy runtime + smoke pass setelah patch
- [ ] Next hardening (belum):
  - [ ] fallback HLS remux (`m3u8` -> mp4) untuk post yang tidak punya direct MP4
  - [ ] ranking cookie profile berdasarkan success-rate

### F. Milestone BE-5 — Instagram Resolver + Multi-Cookies (Done, Hardening Lanjut)

**Target:** resolve link Instagram non-live secara stabil, termasuk konten yang butuh sesi akun.

- [x] Tambah endpoint `POST /v1/instagram/resolve` + alias `POST /v1/ig/resolve`
- [x] Validasi host/path Instagram yang didukung (`reel/reels/p/tv`)
- [x] Rotasi cookie profile dari file list + directory scan
- [x] Response menyertakan `cookie_profile` yang berhasil dipakai
- [x] Policy explicit: live content ditolak
- [x] Error code typed untuk HLS-only: `ig_hls_only_not_supported`
- [x] Hardening test:
  - [x] edge cases resolver + endpoint
  - [x] coverage `internal/igresolver` stabil di ~95%
- [ ] Next hardening (belum):
  - [ ] fallback HLS remux (`m3u8` -> mp4) untuk post IG yang tidak punya direct MP4
  - [ ] ranking cookie profile berdasarkan success-rate

### G. Milestone BE-6 — TikTok Resolver + Multi-Cookies (Done, Hardening Lanjut)

**Target:** resolve link TikTok non-live secara stabil, termasuk konten yang butuh sesi akun.

- [x] Tambah endpoint `POST /v1/tiktok/resolve` + alias `POST /v1/tt/resolve`
- [x] Validasi host/path TikTok yang didukung (`@user/video`, `/t/`, `vm.tiktok.com`, `vt.tiktok.com`)
- [x] Rotasi cookie profile dari file list + directory scan
- [x] Response menyertakan `cookie_profile` yang berhasil dipakai
- [x] Policy explicit: live content ditolak
- [x] Error code typed untuk HLS-only: `tt_hls_only_not_supported`
- [x] Hardening test:
  - [x] edge cases resolver + endpoint
  - [x] coverage `internal/ttresolver` stabil di ~95%
- [ ] Next hardening (belum):
  - [ ] fallback HLS remux (`m3u8` -> mp4) untuk post TikTok yang tidak punya direct MP4
  - [ ] ranking cookie profile berdasarkan success-rate

### H. Milestone BE-7 — Settings + Profile Enterprise (Done Baseline, Hardening Lanjut)

**Target:** user login bisa update profile dasar + settings secara conflict-safe dan auditable.

- [x] Tambah endpoint owner-scoped:
  - [x] `GET /v1/profile`
  - [x] `PATCH /v1/profile` (phase-1: `full_name`)
  - [x] `POST /v1/profile/avatar`
  - [x] `DELETE /v1/profile/avatar`
  - [x] `GET /v1/settings`
  - [x] `PATCH /v1/settings`
- [x] Default snapshot settings auto-bootstrap saat user pertama kali akses
- [x] Optimistic concurrency aktif via `meta.version` + error code `settings_version_conflict`
- [x] Audit trail settings mutation tersimpan di `user_settings_audit` (before/after + changed_fields)
- [x] Validation contract dasar:
  - [x] unknown JSON field ditolak (`DisallowUnknownFields`)
  - [x] enum `default_quality` tervalidasi (`4k/1080p/720p/480p`)
- [ ] Next hardening (belum):
  - [ ] Tambah test integration Postgres khusus settings store
  - [ ] Migrasi skema settings ke migration file explicit
  - [ ] Endpoint read audit/settings activity (ops/support visibility)
  - [ ] Identity hardening phase-2 untuk email-change verified flow

### I. Milestone BE-8 — Avatar Pipeline Enterprise (Done Baseline, Hardening Lanjut)

**Target:** avatar profile tidak lagi placeholder, lifecycle upload/replace/remove kuat di backend.

- [x] Validasi upload avatar max `2MB`
- [x] Normalisasi image ke `512x512` WebP (metadata implicit stripped via re-encode)
- [x] Persist object ke R2 + publish via host publik `avatar.indobang.site`
- [x] Replace flow strict:
  - [x] update profile ke avatar baru
  - [x] delete avatar lama wajib sukses
  - [x] rollback DB + cleanup object baru saat delete lama gagal
- [x] Remove flow strict:
  - [x] clear avatar_url
  - [x] delete object lama wajib sukses
  - [x] rollback avatar_url bila delete gagal
- [x] Full test coverage baseline aktif (unit service + handler + storage + auth/store wiring)
- [ ] Next hardening (belum):
  - [ ] object-level lifecycle policies untuk stale orphan edge-case
  - [ ] image safety scanning hook

### J. Milestone BE-9 — YouTube Heatmap + Video-Cut Enterprise (Planned)

**Target:** mode Manual Trim + Heatmap Cut di UI YouTube benar-benar aktif end-to-end (backend + worker), aman, dan tidak merusak flow download existing.

- [ ] Freeze API contract (backward-compatible):
  - [ ] `POST /v1/youtube/resolve` return `heatmap`, `key_moments`, `heatmap_meta`
  - [ ] Tambah endpoint `POST /v1/jobs/video-cut` untuk mode `manual` + `heatmap`
  - [ ] Polling status tetap reuse `GET /v1/jobs/:id` (tanpa pecah kontrak job)
- [ ] Bangun domain `internal/heatmap`:
  - [ ] Parser + normalisasi bins heatmap dari payload yt-dlp
  - [ ] Smoothing + adaptive threshold + min-distance peak detection
  - [ ] Anti-intro bias supaya peak awal video tidak selalu auto-terpilih
- [ ] Validation & security hardening:
  - [ ] Validasi YouTube URL + `format_id` harus valid dari hasil resolve
  - [ ] Validasi trim range (`start < end`, batas durasi cut, batas output size)
  - [ ] Typed error code (`heatmap_not_available`, `invalid_trim_range`, `video_cut_failed`, dst)
- [ ] Queue + worker video-cut pipeline:
  - [ ] Enqueue task Asynq khusus video-cut (bukan redirect langsung)
  - [ ] Implement ffmpeg path untuk manual trim + heatmap cut
  - [ ] Upload artifact hasil cut ke R2 (TTL + retry policy + status sinkron)
- [ ] Rollout safety + observability:
  - [ ] Feature flag `YTD_HEATMAP_TRIM_ENABLED` + rollback instan
  - [ ] Structured log + metrics (`video_cut_job_total`, `heatmap_available_total`, error code rate)
- [ ] Testing gate enterprise:
  - [ ] Unit test `internal/heatmap` + validation path
  - [ ] Handler + integration test queue->worker->R2
  - [ ] Regression test flow existing (`/v1/download/mp4` no-trim dan job MP3) tetap aman

- [ ] Next hardening (belum):
  - [ ] Signed resolve snapshot/token untuk cegah tampering parameter trim
  - [ ] Owner-scope enforcement penuh untuk `GET /v1/jobs/:id`
  - [ ] Benchmark CPU/latency ffmpeg + tuning preset kualitas output

### K. Backend Quality Checklist

- [x] Full-suite backend test runner tersedia (`make backend-test` / `scripts/test-backend.sh`) dengan coverage output
- [x] Resolver X multi-cookie aktif + tervalidasi real-link
- [x] Resolver Instagram multi-cookie aktif + tervalidasi test-suite
- [x] Resolver TikTok multi-cookie aktif + tervalidasi test-suite
- [x] False-negative direct `http-*` MP4 untuk X sudah dipatch
- [x] History enterprise baseline aktif (write/read/redownload/delete + resolve-capture endpoint)
- [x] Settings/Profile enterprise baseline aktif (owner-scoped endpoint + versioned patch + audit trail)
- [x] Avatar enterprise baseline aktif (schema + endpoint + strict delete/rollback semantics)
- [ ] API contract heatmap YouTube (`heatmap` + `key_moments`) live dan tervalidasi
- [ ] Endpoint/job `video-cut` (manual + heatmap) lulus E2E staging
- [ ] Feature flag heatmap/video-cut + rollback path tervalidasi
- [ ] Metrics observability heatmap/video-cut tersedia
- [ ] Test integration Postgres untuk settings store belum ada
- [ ] Rollout schema settings masih runtime-ensure (belum migration file)
- [ ] Kontrak explicit reject untuk null-field patch settings belum enforced penuh
- [ ] Parity status `resolved` antara handler enum dan constraint schema Postgres
- [ ] Metrics observability khusus avatar pipeline belum lengkap
- [ ] Endpoint `GET /v1/jobs/:id` owner-scoped untuk session login
- [ ] Semua dependency runtime tervalidasi saat startup
- [ ] Error backend konsisten & aman ditampilkan ke frontend
- [ ] Worker tidak silent-fail
- [ ] Retention/cleanup jobs tetap terkendali
- [ ] Fallback untuk varian HLS-only post X
- [ ] Fallback untuk varian HLS-only post Instagram
- [ ] Fallback untuk varian HLS-only post TikTok

---

## 3) MVP Ready Gate (Gabungan FE + BE)

MVP dianggap siap kalau semua checklist ini true:

### Frontend Gate
- [x] User bisa resolve URL YouTube dari Home
- [x] User bisa download MP4 dari pilihan format
- [x] User bisa request MP3 + lihat progress + unduh hasil (YouTube)
- [x] History pakai data real (bukan sample)
- [x] User bisa resolve + **pilih kualitas** + download dari link X via UI
- [x] User bisa resolve + **pilih kualitas** + download dari link Instagram via UI
- [x] User bisa resolve + **pilih kualitas** + download dari link TikTok via UI
- [x] Saat dapat error code HLS-only (`x_hls_only_not_supported` / `ig_hls_only_not_supported` / `tt_hls_only_not_supported`), UI menampilkan warning terarah (by design belum support)

### Backend Gate
- [x] API tidak terekspos publik langsung (internal-only + proxy)
- [x] Deploy repeatable via script (`deploy.sh`)
- [x] Worker aktif stabil
- [x] R2 aktif dan MP3 end-to-end lulus test
- [x] User bisa resolve X/Twitter non-live via `POST /v1/x/resolve`
- [x] User bisa resolve Instagram non-live via `POST /v1/instagram/resolve`
- [x] User bisa resolve TikTok non-live via `POST /v1/tiktok/resolve`
- [x] User bisa baca/update profile dasar via `GET/PATCH /v1/profile` (owner-scoped)
- [x] User bisa baca/update settings via `GET/PATCH /v1/settings` dengan version conflict-safe (`settings_version_conflict`)
- [x] User bisa upload/remove avatar via `POST/DELETE /v1/profile/avatar` (owner-scoped)
- [ ] Semua varian post video X (termasuk HLS-only) 100% covered
- [ ] Semua varian post video Instagram (termasuk HLS-only) 100% covered
- [ ] Semua varian post video TikTok (termasuk HLS-only) 100% covered

### Security/Operational Gate
- [x] Web publik, API private internal
- [ ] Monitoring + alert minimum tersedia
- [ ] Dokumen troubleshooting dasar tersedia

---

## 4) Delivery Notes — Implementasi Fitur X (2026-03-18)

### A. Scope yang dikerjakan

- [x] Backend resolver X/Twitter dengan endpoint baru `POST /v1/x/resolve`
- [x] Multi-account cookie support untuk bypass lock konten tertentu
- [x] Integrasi env runtime + deploy + validasi real link

### B. Jalur pengerjaan

- [x] Patch code di repo **workspace**
- [x] Hardening test + coverage
- [x] Push ke `main`
- [x] Deploy dari repo **runtime** (`/home/ubuntu/yt-downloader`) via `./deploy.sh`

### C. Hasil validasi lapangan

- [x] Sukses di beberapa link real (contoh SpaceX/Reuters/CGTN)
- [x] Error handling jelas untuk live content
- [x] Issue utama yang ketemu: format direct MP4 dengan codec metadata kosong
- [x] Status issue utama: **fixed** (fallback selector `http-*` MP4)

### D. Catatan batasan saat ini

- [ ] Belum semua post X selalu punya direct MP4 (sebagian HLS-only)
- [ ] Untuk kasus HLS-only, belum ada remux fallback di backend saat ini (by design: belum dikerjakan)
- [x] (Update 2026-03-24) UI frontend resolve X + picker kualitas download sudah masuk baseline
- [x] (Update 2026-03-24) UI frontend sudah map code `x_hls_only_not_supported` ke warning user-friendly

---

## 5) Delivery Notes — Implementasi Fitur Instagram Resolver (2026-03-18)

### A. Scope yang dikerjakan

- [x] Backend resolver Instagram dengan endpoint baru `POST /v1/instagram/resolve`
- [x] Alias endpoint `POST /v1/ig/resolve`
- [x] Multi-account cookie support untuk auto-rotate saat resolve
- [x] Typed error code untuk HLS-only: `ig_hls_only_not_supported`

### B. Jalur pengerjaan

- [x] Patch code di repo **workspace**
- [x] Hardening test + coverage resolver + HTTP handler
- [x] Integrasi config/env baru untuk IG cookies

### C. Hasil validasi

- [x] Unit + integration suite backend lulus (`make backend-test`)
- [x] Endpoint IG menerima flow yang sama seperti X (resolve + format list + cookie_profile)
- [x] Error non-generic untuk kasus HLS-only sudah tersedia untuk wiring FE

### D. Catatan batasan saat ini (Instagram)

- [ ] Untuk post IG HLS-only, belum ada remux fallback (masih return typed warning)
- [x] (Update 2026-03-24) UI frontend resolve IG + picker kualitas download sudah masuk baseline
- [x] (Update 2026-03-24) UI frontend sudah map code `ig_hls_only_not_supported` ke warning user-friendly

---

## 6) Delivery Notes — Implementasi Fitur TikTok Resolver (2026-03-18)

### A. Scope yang dikerjakan

- [x] Backend resolver TikTok dengan endpoint baru `POST /v1/tiktok/resolve`
- [x] Alias endpoint `POST /v1/tt/resolve`
- [x] Multi-account cookie support untuk auto-rotate saat resolve
- [x] Typed error code untuk HLS-only: `tt_hls_only_not_supported`

### B. Jalur pengerjaan

- [x] Patch code di repo **workspace**
- [x] Hardening test + coverage resolver + HTTP handler
- [x] Integrasi config/env baru untuk TT cookies

### C. Hasil validasi

- [x] Unit + integration suite backend lulus (`make backend-test`)
- [x] Endpoint TikTok menerima flow yang sama seperti X/Instagram (resolve + format list + cookie_profile)
- [x] Error non-generic untuk kasus HLS-only sudah tersedia untuk wiring FE

### D. Catatan batasan saat ini (TikTok)

- [ ] Untuk post TikTok HLS-only, belum ada remux fallback (masih return typed warning)
- [x] (Update 2026-03-24) UI frontend resolve TikTok + picker kualitas download sudah masuk baseline
- [x] (Update 2026-03-24) UI frontend sudah map code `tt_hls_only_not_supported` ke warning user-friendly


---

## 7) Delivery Notes — History Backend Enterprise (2026-03-24 refresh)

### A. Scope yang dikerjakan

- [x] Implementasi model item+attempt (`history_items` + `history_attempts`) owner-scoped per user
- [x] Write-path history di flow:
  - [x] `POST /v1/jobs/mp3` (create queued attempt + `job_id` link)
  - [x] `GET /v1/download/mp4` (processing -> done/failed)
- [x] Sinkronisasi status dari worker by `job_id` (`queued -> processing -> done/failed`)
- [x] Read API history lengkap:
  - [x] `GET /v1/history`
  - [x] `GET /v1/history/stats`
  - [x] `POST /v1/history/{id}/redownload`
  - [x] `DELETE /v1/history/{id}`
- [x] Endpoint create ringan `POST /v1/history` untuk catat resolve event (status `resolved`)

### B. Jalur pengerjaan

- [x] Implement store abstraction `internal/history` (engine memory + postgres)
- [x] Auto schema bootstrap tabel/index saat startup backend
- [x] Integrasi auth boundary via `requireSessionIdentity` untuk endpoint history
- [x] Integrasi frontend:
  - [x] halaman `/history` pakai data API real
  - [x] home flow kirim create-history setelah resolve (best-effort, tidak blok UI)

### C. Hasil validasi

- [x] Owner scoping jalan (query/update history selalu berdasarkan `user_id` session)
- [x] Semantik redownload tervalidasi:
  - [x] MP3 unexpired -> `mode=direct`
  - [x] MP3 expired -> `mode=queued` + enqueue ulang job
  - [x] MP4/image -> `mode=direct` via endpoint proxy download
- [x] Soft delete item langsung menyembunyikan data dari list API
- [x] History test suite backend hijau untuk scope handler/store
  - [x] smoke lokal: `go test ./internal/history ./internal/http -run History -count=1`

### D. Catatan batasan saat ini (History)

- [ ] Constraint status di schema Postgres belum memasukkan `resolved` (perlu align dengan `POST /v1/history`)
- [ ] Endpoint `GET /v1/jobs/:id` belum owner-scoped untuk pengguna login
- [ ] Retention/purge terjadwal untuk history belum aktif
- [ ] Observability history (metrics + audit fields) belum lengkap

---

## 8) Delivery Notes — API Integration v1 (2026-03-24)

### A. Scope yang dikerjakan

- [x] Utility deteksi platform `detectPlatform` di `apps/web/src/lib/utils.ts`
- [x] Dynamic routing di `api.resolve` (`apps/web/src/lib/api.ts`) ke endpoint backend sesuai platform
- [x] Input bar adaptif (`apps/web/src/components/shared/InputBar.tsx`):
  - [x] ikon otomatis mengikuti platform URL
  - [x] placeholder otomatis mengikuti platform URL
  - [x] resolve flow otomatis sesuai platform tanpa mode selector manual

### B. Jalur pengerjaan

- [x] Patch FE helper (`utils.ts`) + API client (`api.ts`)
- [x] Wiring komponen input agar Paste/Enter langsung trigger resolve pipeline
- [x] Hook best-effort simpan history setelah resolve sukses untuk user login

### C. Hasil validasi

- [x] Satu kolom input sekarang bisa dipakai untuk YouTube/X/Instagram/TikTok
- [x] Tidak perlu ubah mode manual sebelum resolve
- [x] Home flow sinkron dengan endpoint backend platform masing-masing

### D. Catatan batasan saat ini (API integration v1)

- [ ] Analytics event FE (success/fail resolve per platform) belum ditambahkan
- [ ] Untuk URL yang tidak terdeteksi platform, UI tetap return error `Unsupported or invalid social media URL.`

---

## 9) Delivery Notes — Backend Settings + Profile Enterprise (2026-03-23 implementasi, 2026-03-24 audit)

### A. Scope yang dikerjakan

- [x] Blueprint enterprise ditulis dulu via RFC (`docs/settings-enterprise-rfc.md`)
- [x] Endpoint auth-required untuk domain profile/settings:
  - [x] `GET /v1/profile`
  - [x] `PATCH /v1/profile` (phase-1: update `full_name`)
  - [x] `GET /v1/settings`
  - [x] `PATCH /v1/settings`
- [x] Modul backend baru `internal/settings` (service + store abstraction)
- [x] Store engine ganda settings:
  - [x] in-memory backend (dev/fallback)
  - [x] postgres backend (prod path)
- [x] Mekanisme optimistic concurrency untuk settings (`meta.version` + conflict 409)
- [x] Audit trail perubahan settings ke `user_settings_audit` (before/after + changed_fields)

### B. Jalur pengerjaan

- [x] Phase design dulu, baru implementation:
  - [x] RFC baseline masuk (`89d5c74`)
  - [x] Implementasi API + store + wiring frontend masuk (`c2a5eb9`)
- [x] Route wiring di `internal/http/server.go` + handler dedicated `settings_handlers.go`
- [x] Owner scope dipastikan lewat `requireSessionIdentity` (user_id diambil dari session, bukan payload)
- [x] Schema tabel settings dibuat otomatis oleh backend saat startup akses store (runtime ensure-schema)

### C. Hasil validasi

- [x] First-read behavior: user baru langsung dapat snapshot default settings (version=1)
- [x] Patch behavior:
  - [x] update sukses menaikkan `version` (+1)
  - [x] stale version menghasilkan `settings_version_conflict` (HTTP 409)
  - [x] no-op patch tidak bikin bump version/audit baru
- [x] Profile mutation phase-1 jalan untuk `full_name` dengan validasi panjang minimum/maksimum
- [x] Test suite terkait settings/profile lulus di scope sekarang:
  - [x] `go test ./internal/settings ./internal/http -count=1`
  - [x] snapshot lokal coverage: `internal/settings ~49.5%`, `internal/http ~81.3%`

### D. Catatan batasan saat ini (Settings BE)

- [ ] Belum ada integration test Postgres khusus `internal/settings` (termasuk race/conflict + audit insert)
- [ ] Rollout schema masih runtime DDL (`ensureSchema`), belum migration file explicit
- [ ] Kontrak strict reject untuk `null` pada field patch belum eksplisit (saat ini `null` diperlakukan omitted oleh decoder)
- [ ] Endpoint baca audit settings (user/admin support view) belum ada
- [ ] Flow perubahan email terverifikasi (phase-2 identity hardening) belum diimplementasikan


---

## 10) Delivery Notes — Avatar Profile Enterprise (2026-03-24)

### A. Scope yang dikerjakan

- [x] Tambah domain avatar profile end-to-end (backend + frontend)
- [x] Backend data model:
  - [x] `auth_users.avatar_url` aktif untuk source-of-truth profile avatar
  - [x] Backward-safe schema ensure (`ALTER TABLE ... ADD COLUMN IF NOT EXISTS`)
- [x] Backend API:
  - [x] `POST /v1/profile/avatar` (multipart upload)
  - [x] `DELETE /v1/profile/avatar`
- [x] Processing pipeline:
  - [x] validasi upload max `2MB`
  - [x] normalisasi image ke WebP `512x512` (center-crop)
  - [x] store ke R2 + URL publish via `AVATAR_PUBLIC_BASE_URL`
- [x] Strict delete semantics:
  - [x] replace avatar wajib hapus object lama (gagal -> rollback)
  - [x] remove avatar wajib hapus object lama (gagal -> rollback)
- [x] Frontend wiring:
  - [x] settings page upload/remove profile photo real
  - [x] navbar desktop+mobile baca avatar user real
  - [x] fallback default image static (`/images/avatar-default.svg`)

### B. Jalur pengerjaan

- [x] Extend auth model + store abstraction (`UpdateUserAvatarURL`) untuk memory/postgres
- [x] Extend storage client R2 (`UploadObject`, `DeleteObject`)
- [x] Implement `internal/avatar` service + processor abstraction
- [x] Tambah avatar handlers di HTTP server + route wiring profile domain
- [x] Integrasi FE API client (`uploadProfileAvatar`, `removeProfileAvatar`) + state sync ke auth store

### C. Hasil validasi

- [x] Backend full-suite lulus:
  - [x] `go test ./... -count=1`
- [x] Frontend lint + build lulus:
  - [x] `npm run lint`
  - [x] `npm run build`
- [x] Auth payload sekarang sudah carry `avatar_url` untuk response register/login/me/profile

### D. Catatan batasan saat ini (Avatar)

- [ ] Dedicated Postgres integration test untuk race edge-case avatar replace belum ada
- [ ] Metrics/audit observability khusus avatar pipeline belum lengkap
- [ ] Offline fallback upload queue belum dibangun

---

## 11) Update: 25/03/2026

### Branch: `create/add-subscription-and-ui-admin`

- [x] The `/settings` page has been updated with an adjustable sidebar and a mobile hamburger menu
- [x] Create UI subscription and linked to sidebar in the settings
- [x] fix Ui subscription 1.0
- [x] fix Ui subscription 1.1
- [x] fix Ui subscription 1.2
- [x] fix Ui subscription 1.3
- [x] fix Ui subscription 1.4
- [x] fix Ui subscription 1.5: button home sidebar
- [x] fix Ui subscription 1.6: fix view konten page "settings" dan history ketika sidebar collapse
- [x] add mock admin page
- [x] fix Ui admin 1.0: perbaikan ui konten, sidebar dan navbar agar konsisten
- [x] fix Ui admin 1.1: perbaikan route sidebar, perbaikan opsi, penambahan mock opsi dan page users di admin page
- [x] fix Ui admin 1.2: fix 1.1
- [x] fix Ui admin 1.3: penyesuaian menu admin, menambahkan opsi dan page maintenance website

### Branch: `fix/route-all-pages`

- [x] fix route frontend: Perbaikan minor untuk Dynamic Routes/admin/users/[id]
- [x] fix route frontend 1.0: Perbaikan untuk commit sebelumnya dan Restrukturisasi Route Groups

### Struktur Route Saat Ini

#### 1. Halaman Utama (Main Application)
Halaman-halaman ini berada dalam grup `(main)` dan dapat diakses oleh user umum:

| Route | Deskripsi |
|-------|-----------|
| `/` | Beranda (Landing Page) |
| `/history` | Riwayat download user |
| `/settings` | Pengaturan akun dan aplikasi |
| `/subscription` | Manajemen paket langganan dan penagihan |

#### 2. Halaman Administrasi (Admin Console)
Halaman-halaman ini berada dalam grup `(admin)` dan hanya untuk administrator:

| Route | Deskripsi |
|-------|-----------|
| `/admin` | Dashboard ringkasan admin |
| `/admin/users` | Daftar manajemen seluruh user |
| `/admin/users/[id]` | (Dynamic) Detail profil dan aktivitas user spesifik |
| `/admin/maintenance` | Kontrol sistem dan status layanan |

### Branch: `add/trim-heatmap-modal`

- [x] add/trim-heatmap-modal: implementasi trim and heatmap di modal pop up youtube
- [x] add/trim-heatmap-modal: fix commit sebelumnya
- [x] add/trim-heatmap-modal: fix warning pada commit sebelumnya
- [x] add/trim-heatmap-modal: fix heatmap button and more settings on download button in modal popup

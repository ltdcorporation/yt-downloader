# Roadmap Video Downloader

_Last update: 2026-03-24 (History backend enterprise refresh + API integration v1 cleanup)_

> Struktur ini sengaja dipisah: **Frontend full di atas**, **Backend full di bawah**.
>
> Referensi desain enterprise history backend: `docs/history-backend-enterprise-rfc.md`.

---

## 1) Frontend Roadmap (Section Atas)

### A. Status Frontend Saat Ini

- [x] Next.js app + basic pages (`/`, `/history`, `/settings`, `/admin`)
- [x] Home flow **real** untuk YouTube resolve + MP4 download (Step C)
- [x] Frontend pakai internal proxy `/api/*` (lebih aman buat publik)
- [x] `/history` sudah terhubung ke API real (history list/stats/redownload/delete)
- [x] `/settings` masih mock data
- [x] MP3 flow UI (create job + polling status) belum lengkap di halaman utama
- [x] Home flow X/Twitter resolve belum ada di UI (backend sudah siap)
- [x] UI **pilih kualitas download MP4 untuk X/Twitter** belum dikerjakan (status FE: belum implementasi)
- [ ] UX warning khusus untuk kasus X **HLS-only (by design belum didukung)** belum ada di FE
- [x] Home flow Instagram resolve belum ada di UI (backend sudah siap)
- [ ] UI **pilih kualitas download MP4 untuk Instagram** belum dikerjakan (status FE: belum implementasi)
- [ ] UX warning khusus untuk kasus Instagram **HLS-only (by design belum didukung)** belum ada di FE
- [ ] Home flow TikTok resolve belum ada di UI (backend sudah siap)
- [ ] UI **pilih kualitas download MP4 untuk TikTok** belum dikerjakan (status FE: belum implementasi)
- [ ] UX warning khusus untuk kasus TikTok **HLS-only (by design belum didukung)** belum ada di FE

### B. Milestone FE-1 — Home Core Flow (MVP)

**Target:** user paste URL -> lihat opsi -> download MP4 sukses.

- [x] Input URL + paste clipboard
- [x] Call `POST /api/v1/youtube/resolve`
- [x] Render metadata (title, thumbnail, duration)
- [x] Render list format MP4
- [x] Download MP4 via `GET /api/v1/download/mp4`
- [x] Tambah UX state yang lebih rapih (retry CTA, inline troubleshooting)
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

- [x] Ambil data job real dari backend (`GET /v1/history` + cursor pagination)
- [x] Loading, empty-state, error-state yang jelas
- [x] Filter dasar (platform + search)
- [x] Aksi relevan (download again + delete)
- [x] Stats card dari backend (`GET /v1/history/stats`)

### E. Milestone FE-4 — Settings (Minimum Useful)

**Target:** `/settings` jadi kepake, bukan mock doang.

- [ ] Simpan preference sederhana (localStorage dulu)
- [ ] Sinkronkan preference dengan behavior UI yang relevan
- [ ] UX tombol save/discard yang benar-benar ngaruh

### F. Milestone FE-5 — X/Twitter Flow di Home (Next)

**Target:** user paste link X/Twitter -> resolve -> pilih kualitas -> download MP4.

**Status saat ini:** backend sudah siap, tapi implementasi FE untuk flow X masih **belum dikerjakan**.

- [x] Tambah source mode (YouTube / X) di UI home
- [x] Call `POST /api/v1/x/resolve` saat mode X aktif
- [ ] Render metadata + format list dari response resolver X
- [ ] Tambah UI picker kualitas MP4 khusus X (list/card per format + size jika tersedia)
- [ ] Tambah CTA download MP4 per kualitas hasil resolver X
- [ ] Samakan UX pola pemilihan kualitas X agar konsisten dengan flow YouTube
- [ ] Error UX spesifik X (live ditolak, format tidak tersedia, restricted media)
- [ ] Handle backend error code `x_hls_only_not_supported` -> tampilkan warning human-friendly (bukan generic error)
- [ ] Tampilkan state “belum support HLS-only” + CTA fallback (mis. coba link lain) saat code tersebut muncul
- [ ] Logging event ringan buat success/fail resolve X

### G. Milestone FE-6 — Instagram Flow di Home (Next)

**Target:** user paste link Instagram -> resolve -> pilih kualitas -> download MP4.

**Status saat ini:** backend sudah siap, tapi implementasi FE untuk flow Instagram masih **belum dikerjakan**.

- [ ] Tambah source mode (YouTube / X / Instagram) di UI home
- [ ] Call `POST /api/v1/instagram/resolve` saat mode Instagram aktif
- [ ] Render metadata + format list dari response resolver Instagram
- [ ] Tambah UI picker kualitas MP4 khusus Instagram (list/card per format + size jika tersedia)
- [ ] Tambah CTA download MP4 per kualitas hasil resolver Instagram
- [ ] Samakan UX pola pemilihan kualitas Instagram agar konsisten dengan flow YouTube/X
- [ ] Handle backend error code `ig_hls_only_not_supported` -> tampilkan warning human-friendly (bukan generic error)
- [ ] Tampilkan state “belum support HLS-only” + CTA fallback saat code tersebut muncul
- [ ] Logging event ringan buat success/fail resolve Instagram

### H. Milestone FE-7 — TikTok Flow di Home (Next)

**Target:** user paste link TikTok -> resolve -> pilih kualitas -> download MP4.

**Status saat ini:** backend sudah siap, tapi implementasi FE untuk flow TikTok masih **belum dikerjakan**.

- [ ] Tambah source mode (YouTube / X / Instagram / TikTok) di UI home
- [ ] Call `POST /api/v1/tiktok/resolve` saat mode TikTok aktif
- [ ] Render metadata + format list dari response resolver TikTok
- [ ] Tambah UI picker kualitas MP4 khusus TikTok (list/card per format + size jika tersedia)
- [ ] Tambah CTA download MP4 per kualitas hasil resolver TikTok
- [ ] Samakan UX pola pemilihan kualitas TikTok agar konsisten dengan flow YouTube/X/Instagram
- [ ] Handle backend error code `tt_hls_only_not_supported` -> tampilkan warning human-friendly (bukan generic error)
- [ ] Tampilkan state “belum support HLS-only” + CTA fallback saat code tersebut muncul
- [ ] Logging event ringan buat success/fail resolve TikTok

### I. Frontend Quality Checklist

- [ ] Error message human-readable (bukan raw error backend)
- [ ] State konsisten (loading/disabled/success/error)
- [ ] Mobile-first tetap enak di viewport kecil
- [x] Tidak ada mock data tersisa di flow MVP

---

## 2) Backend Roadmap (Section Bawah)

### A. Status Backend Saat Ini

- [x] Runtime hardening `yt-dlp` (resolve via PATH + startup validation)
- [x] Host runtime `yt-dlp` sudah versi terbaru (`/usr/local/bin/yt-dlp`)
- [x] API bind internal-only `127.0.0.1:18080`
- [x] Endpoint MVP aktif:
  - [x] `GET /healthz`
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

### H. Backend Quality Checklist

- [x] Full-suite backend test runner tersedia (`make backend-test` / `scripts/test-backend.sh`) dengan coverage output
- [x] Resolver X multi-cookie aktif + tervalidasi real-link
- [x] Resolver Instagram multi-cookie aktif + tervalidasi test-suite
- [x] Resolver TikTok multi-cookie aktif + tervalidasi test-suite
- [x] False-negative direct `http-*` MP4 untuk X sudah dipatch
- [x] History enterprise baseline aktif (write/read/redownload/delete + resolve-capture endpoint)
- [ ] Parity status `resolved` antara handler enum dan constraint schema Postgres
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
- [ ] User bisa request MP3 + lihat progress + unduh hasil
- [x] History pakai data real (bukan sample)
- [ ] User bisa resolve + **pilih kualitas** + download dari link X via UI
- [ ] User bisa resolve + **pilih kualitas** + download dari link Instagram via UI
- [ ] User bisa resolve + **pilih kualitas** + download dari link TikTok via UI
- [ ] Saat dapat error code HLS-only (`x_hls_only_not_supported` / `ig_hls_only_not_supported` / `tt_hls_only_not_supported`), UI menampilkan warning terarah (by design belum support)

### Backend Gate
- [x] API tidak terekspos publik langsung (internal-only + proxy)
- [x] Deploy repeatable via script (`deploy.sh`)
- [x] Worker aktif stabil
- [x] R2 aktif dan MP3 end-to-end lulus test
- [x] User bisa resolve X/Twitter non-live via `POST /v1/x/resolve`
- [x] User bisa resolve Instagram non-live via `POST /v1/instagram/resolve`
- [x] User bisa resolve TikTok non-live via `POST /v1/tiktok/resolve`
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
- [ ] UI frontend untuk resolve X + picker kualitas download masih backlog (belum implementasi)
- [ ] UI frontend belum map code `x_hls_only_not_supported` ke warning yang user-friendly

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
- [ ] UI frontend untuk resolve IG + picker kualitas download masih backlog (belum implementasi)
- [ ] UI frontend belum map code `ig_hls_only_not_supported` ke warning yang user-friendly

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
- [ ] UI frontend untuk resolve TikTok + picker kualitas download masih backlog (belum implementasi)
- [ ] UI frontend belum map code `tt_hls_only_not_supported` ke warning yang user-friendly


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

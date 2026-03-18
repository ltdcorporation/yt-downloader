# Master Roadmap: Video Downloader

**Tech Stack:**

- **Frontend:** 
- Next.js 14.2.28 (App Router), Tailwind CSS, Zustand, Radix UI.

- **Backend:** 
- Golang, 
- `yt-dlp` (Scraping), 
- `FFmpeg` (Processing).
- https://github.com/ramahueesha/youtube-most-watched-timestamp-scraper-heatmap


- **Deployment:** 
- Docker, 
- Ubuntu VPS,
---

## 1. App Flow & Core Features

### 🌊 App Flow
1. **Input:** User *paste* URL video di Home Page.
2. **Analyzing:** Animasi loading muncul (Skeleton + Spinner) saat sistem *fetching* metadata.
3. **Options State:**
   - **YouTube:** Muncul opsi *Full Download* atau *Trim Video* (menggunakan slider Start/End).
   - **TikTok:** Muncul opsi *No Watermark* / *With Watermark*.
   - **Twitter/IG:** Muncul resolusi video.
4. **Processing:** Sistem mengeksekusi *download* atau pemotongan (FFmpeg). *Progress bar real-time* berjalan.
5. **Success:** File siap diunduh, muncul tombol "Download Now".

### 🌟 Core Features
- **Auto-Platform Detection:** Otomatis mengenali link tanpa user milih manual.
- **YouTube Trimmer Engine:** Potong video langsung di server tanpa *re-encode* (`ffmpeg -c copy`).
- **TikTok Watermark Remover:** *Download* bersih tanpa logo.
- **Local History:** Riwayat unduhan tersimpan di *browser* (tanpa perlu database SQL).

---

## 2. Frontend Pages Breakdown (UI/UX)

### 🏠 1. Home / Main Workspace (`/`)
- **Hero Section:** Input Bar besar dengan fitur *Auto-paste* (Clipboard API).
- **Analyzing State:** *Skeleton loader* dinamis dengan teks "Analyzing Link...".
- **Result & Options State:**
  - Thumbnail Video & Metadata (Judul, Durasi).
  - *Trimmer Slider* (Radix UI) khusus YouTube.
  - Opsi Format (MP4/MP3).
- **Success State:** Ikon centang besar dan tombol "Download Now".

### 📊 2. History Page (`/history`)
- **Stats Card:** Info total unduhan dan kapasitas yang digunakan.
- **Data Table:** Menampilkan daftar *file* yang pernah diunduh (Thumbnail, Platform, Ukuran, Tanggal).

### ⚙️ 3. Settings Page (`/settings`)
- **Account/Profile:** Mockup info profil pengguna.
- **Preferences:** *Dropdown* resolusi *default* (misal: 1080p) dan opsi *Auto-Trim*.

---

## 3. Sprint Development Progress

### ✅ Production Readiness Patch (2026-03-18)

- [x] **Step A — Runtime hardening (yt-dlp):**
  - Default `YTDLP_BINARY` di backend diubah ke `yt-dlp` (resolve dari `PATH`, bukan hardcoded path tunggal).
  - API dan worker sekarang fail-fast check `yt-dlp` saat startup (`exec.LookPath`) + log binary path yang kepakai.
  - Runtime host sudah pakai `yt-dlp` terbaru (`/usr/local/bin/yt-dlp`).

- [x] **Step B — Internal API proxy (Web ↔ API):**
  - Tambah proxy route Next.js: `apps/web/src/app/api/[...path]/route.ts` untuk forward request `/api/*` ke backend internal.
  - Frontend client default call ke `NEXT_PUBLIC_API_URL=/api`.
  - API tetap internal-only (bind localhost), sehingga publik hanya akses web port.

### 🏃 Sprint 1: Setup & Backend Core Engine (Golang)
- [ ] `go mod init` dan setup *framework* API (Fiber/Gin).
- [ ] Buat *wrapper* fungsi `os/exec` untuk mengeksekusi `yt-dlp -j <URL>` dan tangkap output JSON-nya.
- [ ] Buat *wrapper* untuk mengeksekusi `ffmpeg -i <URL> -ss <Start> -to <End> -c copy temp/output.mp4`.
- [ ] Siapkan *endpoint*: `POST /api/analyze` dan `POST /api/process`.

### 🏃 Sprint 2: Frontend Base & UI Slicing (Next.js)
- [ ] `npx create-next-app@14.2.28 client` (App Router).
- [ ] Setup `tailwind.config.ts` dengan warna: `#8785A2`, `#F6F6F6`, `#FFE2E2`.
- [ ] Buat *layouting* dasar: Navbar dan Sidebar (Dashboard, History, Settings).
- [ ] *Slicing* komponen *Home* (Input Bar, Skeleton Loader, Success Card).
- [ ] Install `@radix-ui/react-slider` dan buat komponen `TrimmerSlider` untuk versi mobile & desktop.

### 🏃 Sprint 3: Integrasi API & State Management
- [ ] Setup `Zustand` untuk mengelola *Global State* (URL, Metadata, Progress Status).
- [ ] Hubungkan *Frontend* ke `/api/analyze` menggunakan `axios`.
- [ ] Implementasi *Server-Sent Events* (SSE) di Golang dan Next.js untuk indikator *progress trimming real-time*.
- [ ] Tambahkan *Error Handling* UI (*Toast notifications* jika link mati/error).

### 🏃 Sprint 4: Ekstra Fitur & Optimasi Server
- [ ] Implementasi `localStorage` di Next.js untuk menyimpan data ke halaman `/history` dan `/settings`.
- [ ] Buat *Goroutine ticker* di Golang untuk otomatis menghapus *file* di folder `/temp` setiap 30 menit.
- [ ] Tambahkan validasi *Regex* di *Input Bar* untuk mencegah *request* link asal-asalan ke *backend*.

### 🏃 Sprint 5: Deployment
- [ ] Buat `Dockerfile` untuk Backend (wajib *include* instalasi python3, ffmpeg, yt-dlp).
- [ ] Buat `Dockerfile` untuk Frontend (Next.js *standalone*).
- [ ] Satukan dengan `docker-compose.yml`.
- [ ] *Deploy* ke VPS Ubuntu, setup Nginx (*Reverse Proxy*), dan pasang SSL (Certbot).


### 🌟 Additional Features Plan
- [ ] Auto-Platform Detection: Otomatis ngebaca link YT, IG, TikTok, atau X.
- [ ] YouTube Trimmer: Potong video (Start/End) langsung di server tanpa re-encode pakai FFmpeg.
- [ ] TikTok Watermark Remover: Download video TikTok bersih dari logo.
- [ ] Format & Quality: Pilihan konversi ke MP4 (Video) atau MP3 (Audio) dengan opsi resolusi.
- [ ] Batch Download: Unduh IG Carousel langsung jadi satu file .zip.

- [ ] PWA (Progressive Web App): Web bisa di-install langsung ke homescreen HP user.
- [ ] Live Video Preview: Mini-player yang sinkron dengan slider saat nentuin durasi potong.
- [ ] Desktop-to-Mobile QR Code: Munculin QR Code biar user bisa scan dan langsung download ke HP.
- [ ] Real-time Progress: Pakai Server-Sent Events (SSE) biar indikator loading/rendering akurat.

- [ ] AI Auto-Highlights & Transcript: Ekstrak teks video atau potong otomatis momen terbaik pakai LLM.
- [ ] Vocal & BGM Splitter: Pisahin suara vokal dan instrumen musik.
- [ ] Video to GIF & Subtitle Extractor: Konversi instan ke GIF dan ekstrak file .srt.
- [ ] Telegram/Discord Bot: User bisa download langsung lewat chat.
- [ ] Smart Metadata Injector: Otomatis pasang cover art dan judul asli ke dalam file unduhan.

- [ ] Cloudflare R2 / S3 Offloading: Lempar hasil render ke cloud storage biar disk VPS nggak bengkak.
- [ ] Auto-Cleanup & Redis Cache: Hapus file temp otomatis tiap 30 menit dan cache metadata video viral.
- [ ] Proxy Rotation: Cegah IP ban dari YouTube/TikTok saat traffic tinggi.
- [ ] HEVC/AV1 Compression: Opsi kompresi tingkat tinggi biar file 4K lebih hemat ruang.

- [ ] Token-Gated Premium: Kunci fitur berat (kayak potong video > 1 jam) pakai micro-transaction jaringan koin low-fee (Base, Solana, BNB).
- [ ] Cloud Sync: Opsi push file langsung ke Google Drive/Dropbox (fitur premium).

---

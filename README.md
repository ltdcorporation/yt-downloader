# yt-downloader

Initial monorepo scaffold for a YouTube utility platform:

- `www.domain.com`: Next.js frontend (`EN/ID`, mobile-first, user flow + admin view-only)
- `api.domain.com`: Go API (`resolve formats`, `MP4 redirect`, queue endpoints for MP3)
- MP3 processing queue: Redis + Asynq worker
- Data: PostgreSQL for jobs/errors with 14-day retention
- Temp output: Cloudflare R2 with 1-hour object TTL policy

## Repository Layout

```text
apps/
  web/          # Next.js (frontend + /admin)
  backend/      # Go API + worker
docs/
  blueprint.md  # Locked MVP architecture and constraints
infra/
  cloudflared/  # Tunnel config template
  systemd/      # Service unit templates (native VPS deploy)
```

## Quick Start

### 0) Prepare env files

```bash
cp apps/backend/.env.example apps/backend/.env
cp apps/web/.env.example apps/web/.env
```

### 1) Frontend

```bash
cd apps/web
npm install
npm run dev
```

### 2) Backend API

```bash
cd apps/backend
go mod tidy
go run ./cmd/api
```

### 3) Worker

```bash
cd apps/backend
go run ./cmd/worker
```

### 4) Build Backend Binaries (for systemd)

```bash
make backend-build
```

### 5) Runtime Dependencies

```text
- Redis (required by Asynq queue)
- PostgreSQL (required by jobs/errors store)
- yt-dlp binary
- ffmpeg binary
- Cloudflare R2 credentials (for MP3 artifacts)
```

## API Summary (MVP)

```text
GET  /healthz
POST /v1/youtube/resolve    { url }
GET  /v1/download/mp4       ?url=&format_id=
POST /v1/jobs/mp3           { url }
GET  /v1/jobs/:id
GET  /admin/jobs            (basic auth)
```

## Notes

- MP4 redirects now require `url` + `format_id` (no raw target redirect).
- MP3 job lifecycle is stored in PostgreSQL (falls back to Redis only when `POSTGRES_DSN` is empty).
- `/admin` (web) and `/admin/jobs` (API) both use basic auth (`ADMIN_BASIC_AUTH_USER/PASS`).
- CORS allow-list is controlled by `CORS_ALLOWED_ORIGINS`.
- Jobs and `job_errors` tables are auto-created on first access.

## Smoke Test

```bash
make smoke
```

Optional full MP3 flow check:

```bash
SMOKE_TEST_YOUTUBE_URL="https://www.youtube.com/watch?v=..." make smoke
```

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

### 4) Windows: Run All Services (One-Click)

Double-click `start-all.bat` or run from terminal:

```bash
start-all.bat
```

This will open 3 separate windows:
- Backend API (port 8080)
- Worker (Redis/PostgreSQL consumer)
- Frontend (port 3000)

### 5) Windows: Run Individual Services

```bash
# Backend API only
start-backend.bat

# Worker only
start-worker.bat

# Frontend only
start-frontend.bat
```

### 6) Build Backend Binaries (for systemd)

```bash
make backend-build
```

### 7) Runtime Dependencies

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
POST /v1/auth/register      { full_name, email, password, keep_logged_in? }
POST /v1/auth/login         { email, password, keep_logged_in? }
POST /v1/auth/google        { id_token, keep_logged_in? }
GET  /v1/auth/me            (Bearer token or HttpOnly session cookie)
POST /v1/auth/logout        (idempotent; revokes active session)
GET  /v1/profile
PATCH /v1/profile           { profile: { full_name } }
GET  /v1/settings
PATCH /v1/settings          { settings: {...}, meta: { version } }
POST /v1/youtube/resolve    { url }
POST /v1/x/resolve          { url }
POST /v1/instagram/resolve  { url }
POST /v1/ig/resolve         { url } (alias)
POST /v1/tiktok/resolve     { url }
POST /v1/tt/resolve         { url } (alias)
GET  /v1/download/mp4       ?url=&format_id=
POST /v1/jobs/mp3           { url }
GET  /v1/jobs/:id
GET  /admin/jobs            (basic auth)
```

## Notes

- MP4 redirects now require `url` + `format_id` (no raw target redirect).
- MP3 job lifecycle is stored in PostgreSQL (falls back to Redis only when `POSTGRES_DSN` is empty).
- `/admin` (web) and `/admin/jobs` (API) both use basic auth (`ADMIN_BASIC_AUTH_USER/PASS`).
- Auth endpoints issue cryptographically random session tokens, persist only token hash in storage, and set HttpOnly cookie (`AUTH_SESSION_COOKIE_*` vars).
- Profile endpoint currently supports full-name updates only; email remains identity-managed.
- Settings endpoint uses optimistic concurrency (`meta.version`) and returns `settings_version_conflict` on stale writes.
- Google login endpoint (`/v1/auth/google`) validates Google ID token server-side; set `GOOGLE_CLIENT_IDS` (comma-separated) or `GOOGLE_CLIENT_ID`.
- Frontend Google flow requires `NEXT_PUBLIC_GOOGLE_CLIENT_ID` (must match one audience accepted by backend).
- Frontend defaults to Next.js proxy route (`/api/*`) to avoid CORS/cookie mismatch between `localhost:3000` and backend ports.
- API resolves `YTDLP_BINARY` from `PATH` (`yt-dlp` by default), so runtime is not tied to one fixed absolute path.
- X resolver supports multi-cookie fallback via `X_COOKIES_FILES` (comma-separated files) and/or `X_COOKIES_DIR` (directory scan). Public attempt can be toggled with `X_RESOLVE_TRY_WITHOUT_COOKIES`.
- Instagram resolver supports multi-cookie fallback via `IG_COOKIES_FILES` (comma-separated files) and/or `IG_COOKIES_DIR` (directory scan). Public attempt can be toggled with `IG_RESOLVE_TRY_WITHOUT_COOKIES`.
- Instagram resolver also returns machine-readable warning code `ig_hls_only_not_supported` for HLS-only posts (current design: no HLS remux fallback yet).
- TikTok resolver supports multi-cookie fallback via `TT_COOKIES_FILES` (comma-separated files) and/or `TT_COOKIES_DIR` (directory scan). Public attempt can be toggled with `TT_RESOLVE_TRY_WITHOUT_COOKIES`.
- TikTok resolver also returns machine-readable warning code `tt_hls_only_not_supported` for HLS-only posts (current design: no HLS remux fallback yet).
- MP3 artifact object key supports prefix via `R2_KEY_PREFIX` (example: `yt-downloader/prod`).
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

## Backend Test (Unit + Redis + Postgres Integration)

Run everything (with preflight Redis/Postgres checks):

```bash
./scripts/test-backend.sh
```

Via Makefile:

```bash
make backend-test
```

Override test dependencies if needed:

```bash
YTD_TEST_REDIS_ADDR=127.0.0.1:6382 \
YTD_TEST_POSTGRES_ADMIN_DSN='postgres://postgres:123987@127.0.0.1:5435/postgres?sslmode=disable' \
./scripts/test-backend.sh
```

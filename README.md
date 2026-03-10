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

## Notes

- This scaffold intentionally starts with placeholder handlers for format resolve and queue flow.
- Next step is wiring `yt-dlp`, Redis, PostgreSQL migrations, and R2 signed URL delivery.

# MVP Blueprint (Locked Decisions)

## Product Scope

- Platform: YouTube only (single video URL)
- Supported URL types: `watch`, `youtu.be`, `shorts`
- Output:
  - MP4 via redirect (quality list follows available formats)
  - MP3 (128 kbps) via queue
- UX: 2-step flow
  1. Paste URL and resolve metadata/formats
  2. Choose format/quality and download

## Non-Functional Limits

- No login for public users
- Rate limit: 3 requests/second/IP
- Video duration: max 60 minutes
- Output size: max 1 GB/request
- MP3 output retention in R2: 1 hour
- Admin panel: view-only with basic auth
- Admin route: `www.domain.com/admin`

## Runtime Architecture

- Frontend: Next.js (`EN/ID`, mobile-first)
- API: Go
- Queue: Redis + Asynq
- Storage:
  - PostgreSQL for jobs/errors (retention 14 days)
  - Cloudflare R2 for temporary MP3 artifacts
- Deploy: Single VPS (Ubuntu 22.04), native systemd
- Public ingress: Cloudflare Tunnel

## API Intent (MVP)

- `GET /healthz`
- `POST /v1/youtube/resolve`
- `POST /v1/jobs/mp3`
- `GET /v1/jobs/:id`
- `GET /v1/download/mp4`
- `GET /admin/jobs` (API-level admin data route, basic auth protected)

## Out of Scope (MVP)

- Playlist downloads
- Login/private/DRM content
- Captcha
- Actionable admin actions (retry/delete)
- Terms/DMCA/Takedown pages

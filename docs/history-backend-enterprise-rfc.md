# RFC: Backend History (Enterprise Grade)

- **Status:** Proposed (ready for owner sign-off)
- **Date:** 2026-03-22 (UTC)
- **Scope:** Backend architecture + API contract + rollout plan for `/history` real-data feature
- **Out of scope (this RFC):** frontend coding, UI polishing, non-history product flows

---

## 1) Why this RFC exists

Current `/history` UI is still mock-driven and not backed by persistent per-user history.
If we implement it quickly without a strong data contract, we will hit rework on:

- security (cross-user data access risk),
- scalability (pagination/filtering quality),
- analytics consistency (stats mismatch),
- redownload behavior for expired MP3 artifacts.

This RFC defines a **production-grade** backend baseline so implementation is deterministic, testable, and auditable.

---

## 2) Current-state audit summary (short)

### Frontend (`apps/web`)
- `/history` uses local sample data (`SAMPLE_DATA`, `STATS`), not API.
- `Download Again` / `Delete` are not wired to backend.
- Pagination is static UI only.
- Filter scope currently `youtube|tiktok|instagram` (missing `x`).

### Backend (`apps/backend`)
- Existing routes: auth, resolve, MP3 queue, job status, MP4 proxy stream, admin jobs.
- No dedicated history tables/endpoints.
- No per-user history ownership model yet.

---

## 3) Product goals (history backend)

1. Logged-in users can view **their own** history with real pagination/filter/search.
2. Users can perform **redownload** from history safely.
3. Users can **delete/hide** history entries.
4. History stats are consistent and computed from canonical backend data.
5. Anonymous/public flow remains available (no regression to core downloader behavior).

---

## 4) Domain model (proposed)

We separate logical media item and download attempts.

### 4.1 `history_items`
Represents canonical media entity per user/source.

Columns (proposed):
- `id` (text pk, e.g. `his_<uuid>`)
- `user_id` (fk -> `auth_users.id`, not null)
- `platform` (`youtube|tiktok|instagram|x`, not null)
- `source_url` (text, not null)
- `source_url_hash` (text, not null) — SHA256 of normalized URL
- `title` (text)
- `thumbnail_url` (text)
- `last_attempt_at` (timestamptz)
- `last_success_at` (timestamptz)
- `attempt_count` (int, denormalized cache, optional)
- `deleted_at` (timestamptz, nullable)
- `created_at`, `updated_at` (timestamptz, not null)

Indexes:
- `(user_id, last_attempt_at desc, id desc)`
- `(user_id, platform, last_attempt_at desc, id desc)`
- `(user_id, source_url_hash)` unique where `deleted_at is null`
- partial index on `deleted_at is null`

### 4.2 `history_attempts`
Represents each user download action.

Columns (proposed):
- `id` (text pk, e.g. `hat_<uuid>`)
- `history_item_id` (fk -> `history_items.id`, not null)
- `user_id` (fk -> `auth_users.id`, not null, duplicated for query speed)
- `request_kind` (`mp3|mp4|image`, not null)
- `status` (`queued|processing|done|failed|expired`, not null)
- `format_id` (text, nullable)
- `quality_label` (text, nullable)
- `size_bytes` (bigint, nullable)
- `job_id` (text, nullable) — link to MP3 async job when applicable
- `output_key` (text, nullable)
- `download_url` (text, nullable)
- `expires_at` (timestamptz, nullable)
- `error_code` (text, nullable)
- `error_text` (text, nullable)
- `created_at`, `updated_at`, `completed_at` (timestamptz)

Indexes:
- `(user_id, created_at desc, id desc)`
- `(history_item_id, created_at desc)`
- `(user_id, status, created_at desc)`
- unique partial index on `job_id` where `job_id is not null`

---

## 5) API contract (proposed)

All `/v1/history*` endpoints require authenticated session.

## 5.1 `GET /v1/history`
Keyset pagination + filters.

Query params:
- `limit` (1..50, default 20)
- `cursor` (opaque base64 cursor)
- `platform` (`youtube|tiktok|instagram|x|all`, optional)
- `q` (search text on title, optional)
- `status` (`done|failed|queued|processing|all`, optional)

Response:
```json
{
  "items": [
    {
      "id": "his_xxx",
      "title": "Video title",
      "thumbnail_url": "https://...",
      "platform": "youtube",
      "source_url": "https://...",
      "last_attempt_at": "2026-03-22T20:00:00Z",
      "latest_attempt": {
        "id": "hat_xxx",
        "request_kind": "mp4",
        "status": "done",
        "format_id": "22",
        "quality_label": "1080p",
        "size_bytes": 145299321,
        "download_url": null,
        "expires_at": null,
        "created_at": "2026-03-22T20:00:00Z"
      }
    }
  ],
  "page": {
    "next_cursor": "opaque",
    "has_more": true,
    "limit": 20
  }
}
```

## 5.2 `GET /v1/history/stats`
Response example:
```json
{
  "total_items": 158,
  "total_attempts": 241,
  "success_count": 220,
  "failed_count": 21,
  "total_bytes_downloaded": 13374206912,
  "this_month_attempts": 42
}
```

## 5.3 `POST /v1/history/{id}/redownload`
Triggers safe redownload logic from latest successful context.

Request (optional override):
```json
{
  "request_kind": "mp4",
  "format_id": "22"
}
```

Response variants:

Direct-ready:
```json
{
  "mode": "direct",
  "download_url": "/api/v1/download/mp4?url=...&format_id=22"
}
```

Queued (e.g. MP3 artifact expired, re-queue required):
```json
{
  "mode": "queued",
  "job_id": "job_xxx",
  "status": "queued"
}
```

## 5.4 `DELETE /v1/history/{id}`
Soft delete by owner.

Response:
```json
{ "ok": true }
```

Error codes for history endpoints:
- `unauthorized`
- `history_not_found`
- `history_conflict`
- `history_invalid_cursor`
- `history_invalid_request`
- `history_unavailable`

---

## 6) Write-path integration design

### 6.1 MP3 flow
On `POST /v1/jobs/mp3`:
- If request authenticated:
  1. Upsert `history_items` (by `user_id + source_url_hash`).
  2. Create `history_attempts` status=`queued` with `job_id`.
- Existing job behavior remains unchanged for anonymous requests.

On worker state updates:
- Map `job_id -> history_attempts`.
- Transition `queued -> processing -> done|failed`.
- Save `download_url`, `expires_at`, `error_*`, and timestamps.

### 6.2 MP4/image direct stream flow
On `GET /v1/download/mp4`:
- If request authenticated:
  1. Upsert `history_items`.
  2. Create `history_attempts` status=`processing`.
  3. After stream success: set `status=done`, `size_bytes`, `completed_at`.
  4. On stream failure: set `status=failed`, `error_*`.

Anonymous requests continue to work without history persistence.

---

## 7) Security and ownership rules

1. `history_items.user_id` is the ownership boundary.
2. Every history query/update must include `WHERE user_id = auth_user_id`.
3. Do not accept `user_id` from client payload.
4. Soft-deleted records are excluded by default (`deleted_at is null`).
5. Redownload endpoint resolves only owned records.

Recommended hardening extension (separate patch):
- restrict `/v1/jobs/{id}` visibility for authenticated jobs to owner only.

---

## 8) Data lifecycle and retention policy (recommended default)

- `history_items`: keep indefinitely (user-facing memory).
- `history_attempts`: keep 365 days online.
- Soft delete immediately hides item from API.
- Optional purge worker:
  - hard-delete soft-deleted records older than N days (default 30).
  - purge attempts older than policy horizon.

All retention values configurable via env.

---

## 9) Observability requirements

Structured logs on history write paths:
- `user_id`, `history_item_id`, `attempt_id`, `request_kind`, `status`, `duration_ms`, `error_code`.

Metrics:
- `history_attempt_created_total{kind}`
- `history_attempt_failed_total{kind,error_code}`
- `history_redownload_total{mode}`
- `history_query_latency_ms`

Auditability:
- include request correlation id in logs and response headers.

---

## 10) Testing strategy (must pass before release)

### Unit tests
- cursor encode/decode, invalid cursor handling
- URL normalization/hash stability
- ownership guard logic
- state transition rules for attempts

### Integration tests (Postgres)
- list/filter/search pagination correctness
- user A cannot read/delete/redownload user B history
- MP3 worker updates attempt status end-to-end
- MP4 stream success/failure updates attempt correctly

### Contract tests
- API response schema stability for frontend integration
- error code mapping consistency

### Regression gates
- existing public download flow stays functional without login
- existing auth, resolve, mp3 queue tests remain green

---

## 11) Rollout plan (phased)

### Gate 0 — Stability precondition
- Fix frontend build blocker in `LoginModal.tsx` syntax before merge to shared branch.

### Phase 1 — Schema + store layer
- Add tables/indexes and repository methods.
- No endpoint exposure yet.

### Phase 2 — Write-path instrumentation
- Integrate history writes into MP3 + MP4 flows for authenticated users.
- Keep behavior backward compatible for anonymous users.

### Phase 3 — Read API
- Implement `/v1/history`, `/v1/history/stats`, `/v1/history/{id}/redownload`, `/v1/history/{id}` delete.

### Phase 4 — Frontend wiring
- Replace sample data with API.
- Wire search/tab/pagination/actions to backend.

### Phase 5 — Hardening
- Additional auth scoping for job status endpoint.
- load/perf test + SLO verification.

---

## 12) Acceptance criteria (Definition of Done)

- `/history` renders real per-user data with cursor pagination.
- Redownload works for mp4/mp3 with correct mode semantics.
- Delete action hides entry immediately (soft delete).
- Stats endpoint values match persisted attempts.
- No cross-user data leakage under security tests.
- Existing public no-login download flow remains intact.
- CI passes backend + frontend lint/build/test gates.

---

## 13) Decisions requiring owner confirmation

If not overridden, this RFC uses these defaults:
1. **Delete model:** soft delete (default yes).
2. **Retention:** items indefinite, attempts 365 days.
3. **History granularity:** item + attempt model (both), not single flat table.

---

## 14) Implementation note

This RFC is intentionally **no-code** and can be used as blueprint for implementation PRs.
Coding should start only after explicit sign-off on Section 13 defaults.

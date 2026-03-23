# RFC: Settings + Profile Backend (Enterprise Grade)

- **Status:** Proposed (ready for owner sign-off)
- **Date:** 2026-03-23 (UTC)
- **Scope:** Backend architecture + API contract + rollout plan for `/settings` and profile editing
- **Out of scope (this RFC):** UI visual redesign details, non-settings downloader features

---

## 1) Why this RFC exists

Current `/settings` page is still mock-driven and not backed by persistent user-level preferences.
If we patch this quickly without a strict contract, we risk:

- silent data overwrite across multi-tab/device,
- profile identity bugs (email edited unsafely),
- broken UX (save/discard not deterministic),
- poor auditability for account-level changes.

This RFC defines a **production-grade baseline** so implementation is deterministic, testable, and safe.

---

## 2) Current-state audit summary (short)

### Frontend (`apps/web`)
- `/settings` still mock (`DEFAULT_SETTINGS`), save button is no-op (`console.log`).
- Discard resets to hardcoded sample values, not server snapshot.
- No local persistence and no backend integration for settings.
- Some controls are visual-only (profile photo upload/remove, bell/help action).

### Backend (`apps/backend`)
- Existing routes cover auth/history/resolve/jobs/admin.
- No dedicated table or endpoint for user settings/preferences.
- No concurrency control for mutable profile/settings payload.

---

## 3) Product goals

1. Logged-in users can read/update their own profile + app settings reliably.
2. Multi-device edits are conflict-safe (no silent overwrite).
3. Settings changes are auditable.
4. Settings integrate with real app behavior (starting with default quality selection).
5. API contract is stable and version-aware.

---

## 4) Domain boundaries (important)

### 4.1 Profile domain (identity)
- `full_name`
- `email` (treated as sensitive identity field)

### 4.2 Settings domain (application preferences)
- default download quality
- processing toggles
- notification preferences

**Rule:** Profile and settings are separate endpoints and service modules.

---

## 5) Data model (proposed)

## 5.1 `user_settings`
One row per user.

Columns:
- `user_id` (text pk, fk -> `auth_users.id`)
- `default_quality` (text not null, enum-like check: `4k|1080p|720p|480p`)
- `auto_trim_silence` (boolean not null default false)
- `thumbnail_generation` (boolean not null default false)
- `email_alert_processing` (boolean not null default true)
- `email_alert_storage` (boolean not null default true)
- `email_alert_summary` (boolean not null default false)
- `version` (bigint not null default 1)
- `created_at` (timestamptz not null)
- `updated_at` (timestamptz not null)

Indexes/constraints:
- PK(`user_id`)
- CHECK default_quality in allowed set
- CHECK version >= 1

## 5.2 `user_settings_audit`
Immutable audit stream for settings changes.

Columns:
- `id` (text pk, e.g. `usa_<uuid>`)
- `user_id` (text not null)
- `actor_user_id` (text not null)
- `request_id` (text nullable)
- `source` (text not null, e.g. `web|api|admin`)
- `before_json` (jsonb not null)
- `after_json` (jsonb not null)
- `changed_fields` (text[] not null)
- `created_at` (timestamptz not null)

Indexes:
- (`user_id`, `created_at desc`)
- (`actor_user_id`, `created_at desc`)
- (`request_id`) where not null

## 5.3 Profile mutation policy
Phase-1:
- `full_name` editable directly.
- `email` read-only in settings/profile UI (avoid unsafe direct change).

Phase-2 (recommended):
- `user_email_change_requests` flow with verification token + expiry.

---

## 6) API contract (v1)

All endpoints require authenticated session.

### 6.1 `GET /v1/settings`
Response:
```json
{
  "settings": {
    "preferences": {
      "default_quality": "1080p",
      "auto_trim_silence": true,
      "thumbnail_generation": false
    },
    "notifications": {
      "email": {
        "processing": true,
        "storage": true,
        "summary": false
      }
    }
  },
  "meta": {
    "version": 7,
    "updated_at": "2026-03-23T08:00:00Z"
  }
}
```

### 6.2 `PATCH /v1/settings`
Request (partial allowed):
```json
{
  "settings": {
    "preferences": {
      "default_quality": "720p"
    }
  },
  "meta": {
    "version": 7
  }
}
```

Behavior:
- Server merges patch with current snapshot.
- Version must match current record version.
- On success: increment `version` by 1 and return full latest snapshot.

Conflict response (`409`):
```json
{
  "error": "settings version conflict",
  "code": "settings_version_conflict",
  "meta": {
    "current_version": 8
  }
}
```

### 6.3 `GET /v1/profile`
Response:
```json
{
  "profile": {
    "id": "usr_xxx",
    "full_name": "Jane Doe",
    "email": "jane@example.com",
    "created_at": "2026-03-01T12:00:00Z"
  }
}
```

### 6.4 `PATCH /v1/profile` (phase-1)
Request:
```json
{
  "profile": {
    "full_name": "Jane D."
  }
}
```

Response:
```json
{
  "profile": {
    "id": "usr_xxx",
    "full_name": "Jane D.",
    "email": "jane@example.com",
    "created_at": "2026-03-01T12:00:00Z"
  }
}
```

### 6.5 Standard error codes
- `invalid_session` (401)
- `session_expired` (401)
- `settings_invalid_request` (400)
- `settings_version_conflict` (409)
- `profile_invalid_request` (400)
- `email_taken` (409, reserved for future email-change flow)
- `settings_unavailable` (503)

---

## 7) Concurrency & consistency model

- Settings writes use optimistic concurrency (`meta.version`).
- Last-write-wins **not** allowed silently.
- Every successful write emits one audit row.
- Read-after-write consistency guaranteed for same request cycle.

---

## 8) Backend module design (Go)

Proposed new module:

```text
internal/settings/
  types.go
  service.go
  store.go
  store_postgres.go
  store_memory.go
  service_test.go
  store_postgres_test.go
```

HTTP layer:
- `internal/http/settings_handlers.go`
- route registration in `server.go`

Validation rules:
- strict enum for `default_quality`
- payload unknown fields rejected (`DisallowUnknownFields`)
- partial patch supports omitted fields, rejects `null` for booleans/enums

---

## 9) Frontend integration contract

Settings page requirements:
- Load from API: `GET /v1/profile` + `GET /v1/settings`.
- Maintain:
  - `serverSnapshot`
  - `draftState`
- Save sends PATCH with `meta.version`.
- Discard resets to `serverSnapshot` (never hardcoded defaults).
- Show explicit UI states: loading/saving/success/error/conflict.
- Unsaved-changes guard on navigation/refresh.

First behavior sync target:
- `settings.preferences.default_quality` used for initial format preselect on download modal.

---

## 10) Security, observability, and operations

Security:
- Auth required for all profile/settings endpoints.
- Owner-scoped access only (from session identity, never payload user_id).
- Rate limit inherited from existing HTTP limiter.

Observability:
- structured logs for read/write with `request_id`, `user_id`, `status`, `latency_ms`.
- audit table queryable by support/admin tooling.

Operations:
- prefer explicit SQL migrations (`apps/backend/migrations`) for enterprise change control.
- avoid schema drift from ad-hoc runtime DDL in production rollout.

---

## 11) Testing strategy (required)

Backend:
- unit: validation, merge logic, version conflict.
- integration (postgres): CRUD, conflict race, audit insertion.
- HTTP contract tests: 200/400/401/409/503 branches.

Frontend:
- integration tests for settings page states (load/save/discard/conflict).
- auth-expired scenario handling.

E2E:
- happy path save + reload persistence.
- multi-tab conflict path.

---

## 12) Rollout plan

### Phase A (foundational)
- add DB tables + migrations
- add GET/PATCH settings + GET/PATCH profile(full_name)
- return stable error codes

### Phase B (frontend wiring)
- remove mock settings source
- wire save/discard/dirty-state to real API
- add conflict UX

### Phase C (behavior integration)
- connect `default_quality` to download preselect
- add telemetry for settings adoption

### Phase D (identity hardening, optional next)
- verified email change flow (`start/verify`)
- security notifications for identity change

---

## 13) Definition of done (enterprise bar)

- No mock fallback in `/settings` for authenticated users.
- Save/discard behavior deterministic and tested.
- Conflict-safe writes (409 path proven).
- Audit rows created on every settings mutation.
- API contract documented + tested + backward-safe.
- Rollback plan available (migration down + feature flag guard if needed).

---

## 14) Open decisions for owner sign-off

1. Should email stay read-only in phase-1? (**Recommended: yes**)
2. Keep one combined settings endpoint vs split endpoints by section? (**Recommended: combined with typed subtrees**)
3. Should profile and settings be saved in one transaction endpoint? (**Recommended: no, keep domain separation**)
4. Audit retention horizon (e.g., 180/365 days)?

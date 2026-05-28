## Why

The SDR transcript fallback is live and ingesting at production volume. Phase 1 established the pipe, but the admin dispatch surface needs to exceed the reliability bar of the older active-incident pipeline before monitoring or Phase 2 promotion builds on it.

## What Changes

- Harden `POST /v1/admin/dispatch/transcript` validation for timestamp bounds, filename safety, and per-field transcript text size.
- Add dispatch-specific structured access logs with `request_id`, `status`, and `latency_ms`.
- Keep duplicate uploads idempotent with a single conflict-safe database statement that returns the row id.
- Keep `GET /v1/admin/dispatch/transcripts/recent` aligned with the `received_at` index order.
- Add `GET /v1/admin/dispatch/health` for manual and future automated staleness monitoring.
- Fix Windows uploader install gotchas: default in-script log no longer collides with batch stdout, and `tzdata` is installed/checked for `America/Phoenix`.
- Document the new health endpoint in OpenAPI.

## Findings Audited

| Finding | Fix | Production Value |
|---------|-----|------------------|
| Upload validation was thinner than active filter validation. | Reject unsafe `wav_filename`, bad or absurd `captured_at`, and `display_text` over 16 KiB. | Bad payloads fail at the edge with clear 400s instead of reaching storage. |
| Dispatch endpoints lacked request lifecycle access logs. | Add dispatch route middleware logging `request_id`, method, path, status, and latency. | Manual curl/debug sessions and future alert triage can correlate requests. |
| Duplicate insert used `DO NOTHING` plus fallback select. | Use `ON CONFLICT DO UPDATE ... RETURNING id, duplicate`. | Concurrent duplicate posts return a row id in one database round trip. |
| Recent query had a secondary `id DESC` sort. | Order only by `received_at DESC LIMIT $1`. | Allows the existing `idx_dispatch_transcripts_received_at` index to satisfy the query. |
| Admin limiter could not support multiple configured admin tokens. | Preserve `ADMIN_TOKEN` but accept comma-separated tokens and bucket by presented token hash. | Multiple admin users can have independent upload limiter buckets. |
| No dispatch staleness endpoint existed. | Add `GET /v1/admin/dispatch/health` with last-received age and rolling counters. | Dan/Claude can poll freshness without scraping recent rows manually. |
| Uploader log default collided with batch stdout redirect. | Default `CACTUS_UPLOADER_LOG` to `uploader_inscript.log` and warn on collision. | Windows scheduled task avoids two writers to the same file. |
| Windows `zoneinfo` can lack `America/Phoenix`. | Add `tzdata`, requirements install step, and startup self-check. | Clean installs fail once with an actionable error instead of per-file exceptions. |
| Metrics package was absent. | Deferred new counters/histograms; no new dependency introduced. | Avoids inventing a metrics stack outside this hardening scope. |

## Phase Boundary

This change does not parse transcripts into incidents, modify `/v1/incidents/active`, change the ingester, alter scraper behavior, rotate `ADMIN_TOKEN`, touch the Flutter app, or trigger deployment.

## Impact

- Affected API code: `internal/api/admin_dispatch_transcripts.go`, `internal/api/router.go`, `internal/api/public.go`, `internal/api/admin_recent.go`.
- Affected store code: dispatch transcript insert/recent/health functions in `internal/store/store.go`.
- Affected scripts: `scripts/cactus_uploader/`.
- Affected tests: focused Go dispatch/store/OpenAPI tests and Python uploader tests.
- Affected docs/specs: OpenAPI document and this OpenSpec change.

## Overview

The dispatch transcript pipe is now a production data source, so the hardening work treats it like the existing active incident API: reject unsafe inputs early, preserve request context in logs, keep database access predictable under growth, and expose staleness in a compact ops endpoint.

## Request Validation

The upload endpoint still accepts the Phase 1 JSON payload shape and stores unknown fields in `raw_payload`. The stricter checks apply only to fields the backend already promotes or relies on:

- `wav_filename` must be a filename, not a path: `^[A-Za-z0-9_]+\.wav$`, no separators, no `..`.
- `captured_at` must parse as RFC3339, is normalized to UTC, must be on or after `2024-01-01T00:00:00Z`, and must not be more than 30 minutes ahead of API server time.
- `display_text` is capped at 16 KiB even though the whole request remains capped at 256 KiB.

Normalization is chosen over rejecting non-UTC offsets because the uploader sends UTC today, while accepting explicit RFC3339 offsets remains safe and observable.

## Observability

Dispatch admin routes get a scoped access-log middleware instead of changing public endpoint behavior. Each dispatch route log includes:

- `request_id` from `X-Request-ID` or a generated dispatch id.
- `method` and route path.
- final HTTP `status`.
- `latency_ms`.

No metrics counters are added because the repository has no `internal/metrics` package or existing metrics dependency. Introducing one would be a separate observability change.

## Database Access

Duplicate safety uses one `INSERT ... ON CONFLICT (wav_filename) DO UPDATE ... RETURNING` statement. This avoids the known race where a `DO NOTHING` conflict can observe the conflict but not return the concurrent row from a follow-up select in the same statement snapshot.

The recent transcript list now orders by `received_at DESC LIMIT $1`, matching `idx_dispatch_transcripts_received_at`. The final validation includes `EXPLAIN ANALYZE` against local Postgres to confirm the index scan.

The health endpoint uses independent time-window subqueries keyed on `received_at` so each rolling count can use the existing received-at index as the table grows. High confidence is counted at `verification_confidence >= 0.75`; null or lower confidence rows are counted as low confidence.

## Admin Tokens And Rate Limits

`ADMIN_TOKEN` remains the environment variable. To support multiple admin users without changing the env var name, it may contain comma-separated bearer tokens. The presented token is hashed for the admin dispatch limiter bucket, so each configured token receives an independent upload bucket.

## Windows Uploader

`CACTUS_UPLOADER_LOG` now defaults to `C:\cactus\logs\uploader_inscript.log`, leaving `run_uploader.bat` free to redirect process stdout/stderr to `uploader.stdout` and `uploader.stderr`. Startup warns if an operator overrides the in-script log back to either redirect target.

The uploader now ships `requirements.txt` with `requests` and `tzdata`. Startup verifies that `America/Phoenix` can be loaded and exits with a direct install command if Windows lacks timezone data.

## Non-Goals

- No Phase 2 transcript parser.
- No incident schema changes.
- No changes to `/v1/incidents/active`, active meta semantics, ingester polling, source scraper, or Flutter app.
- No deployment automation or token rotation.

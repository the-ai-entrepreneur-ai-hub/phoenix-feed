## Overview

Phase 1 establishes a durable transcript upload path without changing incident promotion behavior. Dan's Windows SDR box writes one `*.transcript.json` file per radio clip; the uploader reads those files, adds normalized metadata, and posts the raw JSON to phoenix-feed. The backend stores both the complete raw payload and selected queryable monitoring columns.

## Backend Storage

The `dispatch_transcripts` table stores one row per `wav_filename`. `wav_filename` is unique so uploads are idempotent: repeat POSTs return the existing row id with `duplicate=true`.

`raw_payload JSONB` is intentionally retained in full because Phase 2 parsing will read the original transcript shape, including model-specific metadata that Phase 1 does not validate. Phase 1 only validates the upload envelope fields needed to safely store a row: non-empty `wav_filename` and RFC3339 `captured_at`.

Promoted columns such as `display_text`, `primary_text`, `verification_confidence`, and `domain_keyword_matches` are extracted during ingestion for quick ops inspection and future monitoring. These columns do not imply parser confidence and do not create incidents in Phase 1.

## Incident Link Placeholder

The table includes nullable `parsed_incident_id` and `parsed_at` fields for Phase 2 promotion bookkeeping. The current `incidents` table in this repository uses composite `(source, incident_id)` keys and has no numeric `id`, so Phase 1 does not enforce a foreign key. The placeholder remains nullable until Phase 2 defines the exact promoted incident key.

## Admin API

Both endpoints reuse the existing `Authorization: Bearer $ADMIN_TOKEN` helper and fail closed when `ADMIN_TOKEN` is unset. Upload bodies are limited to 256 KiB and must be JSON objects. The upload endpoint applies a per-token 60 request burst limiter using the existing in-memory rate-limit package.

The recent endpoint returns the newest 50 rows ordered by `received_at DESC` and intentionally exposes only operational fields, not the full raw transcript JSON.

## Windows Uploader

The uploader uses a 5-second polling loop instead of watchdog/inotify because Dan's target environment is Windows and reliability matters more than elegance. Polling also makes scheduled-task restarts straightforward.

Local sqlite state lives by default at `D:\cactus\state\uploaded_transcripts.sqlite3`. This makes the uploader restart-safe without depending on an extra server roundtrip before every file. Accepted uploads, duplicate responses, and permanent local/server failures are recorded locally so the script does not retry them forever.

The uploader treats network failures, 5xx responses, 401 responses, and 429 responses as retryable. Other 4xx responses are recorded as permanent failures because they indicate a file or payload that the server will not accept without manual repair.

## Non-Goals

- Do not parse transcripts into the `incidents` table.
- Do not change active incident responses or app UI.
- Do not change the mapserver scraper, ingester cadence, or incident lifecycle logic.
- Do not rotate or regenerate `ADMIN_TOKEN`.
- Do not delete or mutate files in `D:\cactus\recordings`.

## Verification

Focused Go handler tests cover admin auth, upload validation, duplicate response semantics, body-size enforcement, rate limiting, and recent transcript output. Python pytest coverage verifies local sqlite skipping, America/Phoenix timestamp conversion, retry behavior, permanent 400 handling, and duplicate response handling.

Final validation runs:

- `go test ./...`
- `go vet ./...`
- `go build ./...`
- `python -m pytest scripts/cactus_uploader/tests/`
- `openspec validate sdr-audio-fallback-phase1 --strict`

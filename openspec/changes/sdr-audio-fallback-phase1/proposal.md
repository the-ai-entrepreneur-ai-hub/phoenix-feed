## Why

Phoenix Fire's mapserver upstream has been frozen for 52+ hours, leaving the live feed without fresh public-safety content even though phoenix-feed itself is healthy. Dan approved an SDR-radio fallback so the system can keep collecting dispatch context while the mapserver is dry.

## What Changes

- Add `dispatch_transcripts` storage for raw SDR transcript uploads.
- Add admin-only `POST /v1/admin/dispatch/transcript` to ingest one transcript JSON payload idempotently by `wav_filename`.
- Add admin-only `GET /v1/admin/dispatch/transcripts/recent` for ops visibility into the last 50 received transcripts.
- Add a Windows-friendly `scripts/cactus_uploader/cactus_uploader.py` polling uploader for Dan's Ledyz box.
- Document both endpoints in the OpenAPI document.

## Phase Boundary

This change is Phase 1 only: upload pipe plus raw transcript storage. It does not parse transcripts into incidents, does not deduplicate against mapserver incidents, and does not change any app/UI behavior.

Planned follow-ups:

- Phase 2: regex parser promotes transcripts into the incidents table only when transcript content has a clear incident type, address, and units.
- Phase 3: confidence gating, deduplication against mapserver data, and optional UI source badge.

## Capabilities

### New Capabilities

- `sdr-dispatch-transcript-ingestion`: Admin-authenticated SDR transcript uploads and ops visibility.

### Modified Capabilities

- None. Existing active incidents, mapserver ingest, scraper polling, and app surfaces remain unchanged.

## Impact

- Affected API code: `internal/api`, `internal/store`, `internal/ratelimit`.
- Affected database: new `dispatch_transcripts` table via migration only.
- Affected scripts: new Windows uploader under `scripts/cactus_uploader/`.
- No changes to `incidents` table schema, active feed behavior, ingester polling, Phoenix mapserver scraper, Flutter app, or dashboard.
- New Python runtime dependency for the uploader: `requests` only.

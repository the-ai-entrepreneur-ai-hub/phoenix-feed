## Why

Phase 1 stores SDR dispatch transcripts but does not surface them to users. Live transcript samples show a small, useful subset of high-confidence rows has a parseable Phoenix dispatch format: units, CDEC channel marker, incident nature, and address. Promoting only those conservative matches gives the active feed a fresh fallback when the Phoenix Fire mapserver is stale.

## What Changes

- Add a `dispatch-parser` worker process that polls unparsed `dispatch_transcripts` rows and promotes only publishable rows into `incidents`.
- Add a regex gate for high-confidence Phoenix dispatch structure: `verification_confidence >= 0.80`, CDEC channel marker, fire/EMS unit, and address/intersection.
- Add Mapbox geocoding with a server-only `MAPBOX_TOKEN`, Phoenix proximity/bbox bias, 5/sec outbound rate limit, and durable `geocode_cache`.
- Add a secondary unique numeric `incidents.id` so `dispatch_transcripts.parsed_incident_id` can reference promoted rows by traceable database id.
- Extend `/v1/admin/dispatch/health` with parser batch/backlog/promotion/failure counters.
- Add parser, geocode, cache, and integration tests for promotion and multi-worker race behavior.

## Phase Boundary

This is Phase 2 only. It does not deduplicate SDR incidents against mapserver incidents, does not add source badging to the app, does not change `/v1/incidents/active` response shape, and does not trigger deploy.

Phase 3 remains the right place for mapserver-vs-SDR deduplication, app badges, and ops alerts. We are not deduplicating yet because the current fallback yield is intentionally small and the safest behavior is to expose traceable SDR rows without risking accidental suppression of live mapserver data.

## Capabilities

### New Capabilities

- `sdr-dispatch-incident-promotion`: Conservative parser worker promotes clean SDR transcripts into active incidents.
- `server-side-geocoding-cache`: Mapbox geocoding through a server-only token with persistent cache and rate limiting.

### Modified Capabilities

- `sdr-dispatch-transcript-ingestion`: Dispatch health includes parser progress and failure counters.
- Active incident serving remains source-agnostic and naturally includes `sdr_audio` rows because they are inserted into `incidents` with non-null point geometry.

## Impact

- Affected command code: new `cmd/dispatch-parser`.
- Affected packages: new `internal/dispatch/parser`, new `internal/geocode`, small `internal/ratelimit` token bucket helper, `internal/store` dispatch health counters, `internal/api` dispatch health response.
- Affected database: migration `0004_incidents_id_and_geocode_cache.sql`.
- Affected production config: new `MAPBOX_TOKEN` environment variable for parser only. Dan/Claude must provision a server-side Mapbox token before deploy. Do not reuse the app `ACCESS_MAP_TOKEN`.
- No changes to mapserver ingester logic, scraper logic, active feed response shape, app code, or `dispatch_transcripts` schema.

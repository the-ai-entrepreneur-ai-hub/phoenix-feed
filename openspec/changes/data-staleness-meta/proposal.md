## Why

Users saw apparently fresh data even when Phoenix Fire's mapserver was serving the same six stale incident records every poll. The existing `data_age_seconds` field measures scrape pipeline health, so it stayed fresh while the newest returned `incident_date` was about 49 hours old.

## What Changes

- Add `meta.newest_incident_at` to incident response envelopes as the UTC timestamp of the newest `incident_date` in the rows returned by that endpoint.
- Add `meta.data_staleness_seconds` as the server-computed integer seconds between response time and `newest_incident_at`.
- Return `null` for both fields when the endpoint returns zero incident rows.
- Preserve `meta.source_last_success_at` and `meta.data_age_seconds` exactly as scrape pipeline health signals.
- Document the new fields in the public OpenAPI document.

## Capabilities

### New Capabilities

- `incident-freshness-meta`: Incident response metadata distinguishes upstream scrape freshness from actual incident data freshness.

### Modified Capabilities

- None. There is no current living `openspec/specs/` baseline in this repository; this change adds a new capability delta.

## Impact

- Affected API code: `internal/api` response metadata and OpenAPI document builder.
- Affected DTO: `internal/store.StalenessMeta` gains two nullable JSON fields.
- Affected endpoints: `GET /v1/incidents/active`, `POST /v1/incidents/refresh`, and incident detail responses that share the staleness meta envelope.
- No database schema, ingester polling, scraper behavior, routing, auth, or incident array shape changes.
- No new dependencies.

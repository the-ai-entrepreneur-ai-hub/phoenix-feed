## Overview

v0.2 keeps the current pull-based Go/Postgres architecture and fills the missing product loop. The API is split into a testable `internal/api` package backed by a small store interface, while `cmd/api` remains responsible for config, logging, DB connection, and server lifecycle.

No new backend dependencies are planned. SQL stays in `internal/store`, HTTP routing stays on `chi/v5`, and all source-specific parsing stays in `internal/source/phxfire`.

## API Design

`internal/api` owns route registration and JSON handlers:

- `GET /v1/incidents/active`
- `GET /v1/incidents/{source}/{incident_id}`
- `GET /v1/incidents/history`
- `GET /v1/health`

Responses that expose incident data include:

```json
{
  "meta": {
    "source_last_success_at": "2026-05-04T06:01:58Z",
    "data_age_seconds": 47,
    "parser_version": "phx-fire-2026-05"
  }
}
```

The active endpoint validates filters before hitting the store. Bad filters return `400` with an error body. Missing detail records return `404`.

The API sends permissive CORS headers for the v0.2 local static client, including `Access-Control-Allow-Origin: *` and `Access-Control-Allow-Methods: GET, OPTIONS`.

## Store Design

`internal/store` adds query methods but keeps SQL hand-written:

- active incident search from `active_incidents`;
- incident detail with unit/event histories;
- latest source health and canary health;
- unit interval/event writes during `UpsertIncident`;
- canary inserts and baseline lookup;
- raw retention update plus vacuum/analyze.

`UpsertIncident` becomes transactional because it must compare the prior snapshot, write the current state, update unit intervals, and append lifecycle events as a single operation.

## Unit Diff Design

Unit history is observed, not authoritative. Poll time is the only timestamp we can defend.

For each `(source, incident_id)` upsert:

1. Read prior incident snapshot and `cleared_at`.
2. Compute a structured delta:
   - `units_added`: units present now but not before.
   - `units_removed`: units present before but not now.
   - `units_changed`: same unit, different status.
   - `fields_changed`: material non-unit field changes.
3. Extend the latest `incident_units` interval when the same unit/status repeats.
4. Insert a new interval when a unit is new or changes status.
5. Emit exactly one lifecycle event for the upsert:
   - `created` for new incident;
   - `reopened` if prior `cleared_at` was non-null;
   - `updated` if an existing non-cleared row has a non-empty delta;
   - no event for unchanged repeat observations.

Clears are emitted by lifecycle after `SweepCleared` returns newly cleared incident IDs.

## Canary Design

`internal/canary` fetches the ArcGIS query URL directly so it can inspect raw fields and spatial reference before handing the body to the production parser. A check returns one result with expected fields, actual fields, output spatial reference, feature count, geometry plausibility, drift JSON, passed boolean, and parser version.

The six checks from `docs/architecture.md` section 7 are implemented in one pass:

1. HTTP 200.
2. Required field coverage.
3. `outSR=4326` honored by spatial reference or plausible lon/lat geometry.
4. Feature count plausible relative to 7-day successful-poll baseline.
5. `Date` parses as recent epoch milliseconds when a sample exists.
6. Production parser accepts the response.

## Janitor Design

The janitor runs every six hours. Each sweep computes `cutoff = now - RAW_RETENTION`, sets `raw = NULL` and `raw_dropped_at = now` for old rows, logs the affected count, then runs `VACUUM ANALYZE incidents`.

## Web Design

The `web/` client is intentionally static and local-first:

- `index.html` loads Leaflet from a CDN.
- `app.js` fetches `http://localhost:8080/v1/incidents/active`.
- `styles.css` uses a two-pane responsive layout.
- The emergency-use banner and attribution are fixed parts of the UI.

## Deferred

Auth, paid history search, notifications, geofences, scanner+Whisper ingestion, cold archive partitions, and production alert integrations remain deferred.

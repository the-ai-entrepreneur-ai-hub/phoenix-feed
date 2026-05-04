## Why

The repository currently has a working ingestion skeleton, schema, and design documents, but the product does not yet run end-to-end for a user. v0.2 completes the smallest useful loop: poll Phoenix, persist current and historical observations, expose read APIs with staleness, show a browser map/list, and surface source drift before users depend on stale data.

## What Changes

### W1. `/v1/incidents/active`

Scope: add a read-only REST endpoint to `cmd/api` that queries cached active incidents from Postgres, supports `bbox`, `lat` + `lon` + `radius_meters`, and `since` / `until` time-window filters, and returns the top-level staleness `meta` block required by `docs/architecture.md` section 8.

Acceptance criteria:

- `GET /v1/incidents/active` returns JSON with `meta` and `incidents`.
- `meta` includes `source_last_success_at`, `data_age_seconds`, and `parser_version`.
- `bbox=minLon,minLat,maxLon,maxLat` filters using `geom`.
- `lat`, `lon`, and `radius_meters` filter using `geom`.
- `since` and `until` filter on `incident_date`.
- Supplying both `bbox` and radius params returns `400`.
- Empty datasets return an empty array and a valid `meta` object.

Files affected: `cmd/api/main.go`, new `internal/api`, `internal/store/store.go`, `internal/model/incident.go`, API tests.

Test plan: table-driven handler tests for query validation and JSON shape; store integration path verified during final DB run; `go vet ./...`; `go test ./...`.

Risk: coordinate filtering bugs can silently hide incidents. The implementation will reject malformed filters instead of falling back to unfiltered results.

Tasks:

- [ ] Define response DTOs and staleness metadata.
- [ ] Add store query for active incidents.
- [ ] Add handler tests for empty response, bbox, radius, time window, and bad mixed filters.
- [ ] Wire route in `cmd/api`.
- [ ] Run vet/test and commit `feat(W1): active incidents endpoint`.

### W2. Incident unit and event history writes

Scope: update the lifecycle/store write path so each successful poll compares the prior `incidents.units` snapshot against the newly observed `Units`, appends or extends `incident_units`, and writes `incident_events` for `created`, `updated`, `cleared`, and `reopened`.

Diff algorithm:

1. Load the prior incident snapshot by `(source, incident_id)` before the upsert.
2. Build maps keyed by `unit`.
3. For each newly observed unit:
   - if absent before, record `units_added`;
   - if present with different `status`, record `units_changed`;
   - if the most recent `incident_units` row for that unit has the same `status`, extend `last_observed_at` and `last_poll_id`;
   - otherwise insert a new interval row.
4. For each prior unit absent from the new snapshot, record `units_removed` in `incident_events.delta`.
5. Compare `nature_code`, `nature_desc`, `channel`, `symbol_code`, `location_text`, and coordinates for material field changes.
6. Emit `created` for new incidents, `reopened` when prior `cleared_at` was non-null, and `updated` when an existing non-cleared incident has any non-empty structured delta.
7. Cleared events are emitted when `SweepCleared` crosses the configured miss threshold.

Acceptance criteria:

- Consecutive identical unit statuses extend one `incident_units` row.
- Status changes create a new `incident_units` interval.
- Removed units appear in an `updated` event delta.
- New incidents create a `created` event.
- Reappearing cleared incidents create a `reopened` event.
- Cleared incidents create a `cleared` event.

Files affected: `internal/store/store.go`, `internal/lifecycle/lifecycle.go`, `internal/model/incident.go`, tests.

Test plan: unit tests for the diff builder; store/lifecycle integration checked against local Postgres; `go vet ./...`; `go test ./...`.

Risk: the upstream `Units` field is mutable and only observed once per poll, so the history is an observation timeline, not an agency timestamp log. API docs must preserve that distinction.

Tasks:

- [ ] Add unit-diff data structures.
- [ ] Write failing tests for add/change/remove/extend behavior.
- [ ] Update `UpsertIncident` to run in a transaction and persist unit intervals/events.
- [ ] Add cleared-event write path from lifecycle.
- [ ] Run vet/test and commit `feat(W2): incident unit history`.

### W8. `/v1/health` upgrade

Scope: replace the skeleton health response with per-source freshness, latest canary status, and aggregate `ok`, `degraded`, or `down` state. Return `503` when any source `last_success_at` is older than 10 minutes or missing.

Acceptance criteria:

- Response includes `server_time`, `state`, `db_reachable`, and a `sources` object keyed by source.
- Each source includes `last_success_at`, `seconds_since_success`, `parser_version`, and canary status.
- State is `down` and status code is `503` when Phoenix Fire has no successful poll or last success is older than 10 minutes.
- State is `degraded` when the DB is reachable and source is fresh but the latest canary failed.
- State is `ok` only when DB is reachable, source is fresh, and latest canary passed or has not run yet.

Files affected: `cmd/api/main.go`, `internal/api`, `internal/store/store.go`, tests.

Test plan: handler tests with fake source health rows for fresh/stale/canary-failed cases; final DB check; `go vet ./...`; `go test ./...`.

Risk: health endpoints often become too optimistic. This one intentionally fails closed on stale source data.

Tasks:

- [ ] Add store query for latest source and canary health.
- [ ] Add handler tests for `ok`, `degraded`, and `down`.
- [ ] Replace skeleton `/v1/health`.
- [ ] Run vet/test and commit `feat(W8): source-aware health`.

### W6. `/v1/incidents/{source}/{incident_id}`

Scope: add an incident detail endpoint returning current incident fields, full `incident_units` history, full `incident_events` history, and the same staleness `meta` block shape as active incidents.

Acceptance criteria:

- Existing incidents return `200`.
- Missing incidents return `404`.
- Response includes `incident`, `unit_history`, `events`, and `meta`.
- Unit and event histories are ordered by observation time ascending.

Files affected: `cmd/api/main.go`, `internal/api`, `internal/store/store.go`, tests.

Test plan: handler tests for found and missing cases; store query verified against local DB; `go vet ./...`; `go test ./...`.

Risk: source names contain hyphens, so the route must bind `{source}` and `{incident_id}` without assuming slash-free synthetic IDs beyond normal URL path rules.

Tasks:

- [ ] Add detail DTOs.
- [ ] Add store detail query.
- [ ] Add handler tests for `200` and `404`.
- [ ] Wire route.
- [ ] Run vet/test and commit `feat(W6): incident detail endpoint`.

### W3. Contract canary assertions

Scope: implement all six checks from `docs/architecture.md` section 7 and persist each check to `contract_canary`. Alerting is a stub that logs at error level for failed checks.

Acceptance criteria:

- Canary checks endpoint reachability and HTTP 200.
- Canary verifies expected fields: `OBJECTID`, `Incident`, `Nature`, `NatureDesc`, `Units`, `Channel`, `SymbolCode`, `Date`, `GenLocInfo`.
- Canary verifies `outSR=4326` via spatial reference and plausible lon/lat geometry.
- Canary compares `feature_count` to a rolling 7-day successful-poll baseline.
- Canary validates `Date` as epoch milliseconds in a recent window when a sample feature is present.
- Canary parses the sample through the production Phoenix parser.
- Every run inserts one `contract_canary` row.
- Failed checks produce a structured `drift` JSON object and `log.Error`.

Files affected: `cmd/canary/main.go`, new `internal/canary`, `internal/source/phxfire`, `internal/store/store.go`, tests.

Test plan: canary unit tests using `httptest.Server` for pass, missing field, bad geometry, bad date, and parser failure; `go vet ./...`; `go test ./...`.

Risk: first-run baseline is unavailable. Decision: baseline plausibility passes when no successful poll baseline exists and records `baseline_unavailable` in drift notes.

Tasks:

- [ ] Export minimal parser helpers from `internal/source/phxfire`.
- [ ] Add canary result model and store insert/query helpers.
- [ ] Write failing canary tests for success and representative failures.
- [ ] Wire `cmd/canary` to config, store, check runner, and `slog`.
- [ ] Run vet/test and commit `feat(W3): contract canary checks`.

### W5. Janitor raw JSONB retention

Scope: implement the `cmd/janitor` retention sweep that drops `incidents.raw` after `RAW_RETENTION`, sets `raw_dropped_at`, and runs vacuum/analyze. The operation must be idempotent and logged.

Acceptance criteria:

- Rows with non-null `raw` and `last_seen_at` older than retention have `raw = NULL` and `raw_dropped_at` set.
- Rows inside retention are untouched.
- Already-dropped rows are untouched.
- Sweep logs how many rows were changed.
- Vacuum/analyze runs after successful updates.

Files affected: `cmd/janitor/main.go`, `internal/store/store.go`, tests.

Test plan: unit tests around retention cutoff logic; DB path verified manually; `go vet ./...`; `go test ./...`.

Risk: `VACUUM` cannot run inside a transaction. Keep the retention update and vacuum as separate store calls.

Tasks:

- [ ] Add store method for raw retention update.
- [ ] Add store method for `VACUUM ANALYZE incidents`.
- [ ] Add janitor tests for cutoff behavior where practical.
- [ ] Wire and log `runSweep`.
- [ ] Run vet/test and commit `feat(W5): raw retention janitor`.

### W4. Vanilla web client at `web/`

Scope: add a static browser client using vanilla HTML, CSS, JavaScript, and Leaflet. It calls `http://localhost:8080/v1/incidents/active`, shows a map and list, displays staleness, includes a persistent "not for emergency use; call 911" banner, and attributes data to the City of Phoenix Fire Department.

Acceptance criteria:

- `web/index.html` opens directly from disk.
- The client fetches `http://localhost:8080/v1/incidents/active`.
- Incidents render as Leaflet markers and list rows.
- Empty arrays render a clear empty state.
- The banner and attribution are always visible.
- Layout works at mobile and desktop widths.

Files affected: new `web/index.html`, `web/styles.css`, `web/app.js`, `README.md`, API CORS middleware if needed.

Test plan: static file inspection; browser/manual smoke with API running; API CORS handler test if added; `go vet ./...`; `go test ./...`.

Risk: opening from `file://` creates an origin of `null`; the API must send permissive CORS headers for local MVP use.

Tasks:

- [ ] Add CORS middleware for local static client.
- [ ] Create HTML shell with Leaflet assets.
- [ ] Create responsive CSS.
- [ ] Create JS fetch/render logic.
- [ ] Run vet/test and commit `feat(W4): static incident map client`.

### W7. `/v1/incidents/history` paid placeholder

Scope: wire the paid history route without building auth, payment, notifications, geofences, or history search. Add `PAID_TIER_ENABLED`, default false. When false, return `402 Payment Required` with a JSON explanation.

Acceptance criteria:

- `PAID_TIER_ENABLED` defaults to false.
- `GET /v1/incidents/history` returns `402` with stable JSON explaining that paid history is disabled for v0.2.
- No auth system is introduced.
- No paid history query is implemented.

Files affected: `internal/config/config.go`, `cmd/api/main.go`, `internal/api`, tests.

Test plan: config tests for default/env parsing; handler test for `402`; `go vet ./...`; `go test ./...`.

Risk: accidentally building real paid behavior before the legal/product gates. The route is intentionally a hard placeholder.

Tasks:

- [ ] Add `PaidTierEnabled` config field.
- [ ] Add config tests for default false and env true.
- [ ] Add history placeholder handler.
- [ ] Wire route.
- [ ] Run vet/test and commit `feat(W7): paid history placeholder`.

## Capabilities

### New Capabilities

- `v0-2-mvp-api`: active incident listing, incident detail, health, and paid-history placeholder API behavior.
- `v0-2-mvp-observability`: source staleness, contract canary persistence, and raw-retention operations.
- `v0-2-mvp-web`: static Leaflet client for the free MVP.

### Modified Capabilities

- None. There are no existing OpenSpec capability specs in this repository.

## Impact

Affected code: `cmd/api`, `cmd/canary`, `cmd/janitor`, `internal/store`, `internal/lifecycle`, `internal/model`, `internal/config`, `internal/source/phxfire`, new `internal/api`, new `internal/canary`, new `web/`, `README.md`, and `docs/v0.2-summary.md`.

Database impact: no schema change is expected because `db/schema.sql` already contains `source_polls`, composite-key `incidents`, `incident_units`, `incident_events`, `contract_canary`, `raw_dropped_at`, and `active_incidents`.

Dependency impact: no new Go dependencies are planned. The web client may load Leaflet from a CDN at runtime; it is not vendored into `go.mod`.

## Decisions

1. OpenSpec change path: use `openspec/changes/v0.2-mvp-complete/` exactly as requested, scaffolded by hand because OpenSpec 1.2.0 rejects dotted change names.
2. Active time-window params: use `since` and `until` as RFC3339 timestamps against `incident_date`.
3. Radius params: use `lat`, `lon`, and `radius_meters`; reject partial radius filters.
4. Mixed spatial filters: reject requests that provide both `bbox` and radius params instead of guessing precedence.
5. Canary baseline: if no 7-day baseline exists, do not fail the feature-count check; record the baseline absence in drift metadata.
6. Paid history: `PAID_TIER_ENABLED=false` returns `402`; enabling the flag is reserved for a later paid implementation and does not create auth in v0.2.

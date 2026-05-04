# v0.2 MVP Completion Tasks

## W1. `/v1/incidents/active`

- [ ] Create internal API response DTOs and staleness metadata.
- [ ] Add active-incident filter parsing for `bbox`, `lat`, `lon`, `radius_meters`, `since`, and `until`.
- [ ] Write failing handler tests for empty response, valid bbox, valid radius, valid time window, and invalid mixed spatial filters.
- [ ] Add `Store.ListActiveIncidents`.
- [ ] Wire `GET /v1/incidents/active`.
- [ ] Run `go vet ./...` and `go test ./...`.
- [ ] Commit `feat(W1): active incidents endpoint`.

## W2. Incident unit and event history writes

- [ ] Add unit delta model and diff helper.
- [ ] Write failing tests for unit add, remove, status change, and unchanged repeat.
- [ ] Make `Store.UpsertIncident` transactional.
- [ ] Extend or insert `incident_units` rows during upsert.
- [ ] Write `incident_events` rows for `created`, `updated`, and `reopened`.
- [ ] Write cleared events after `SweepCleared`.
- [ ] Run `go vet ./...` and `go test ./...`.
- [ ] Commit `feat(W2): incident unit history`.

## W8. `/v1/health` upgrade

- [ ] Add health DTOs.
- [ ] Add store query for per-source last success and latest canary.
- [ ] Write failing handler tests for `ok`, `degraded`, and stale-source `down`.
- [ ] Replace skeleton `/v1/health`.
- [ ] Return `503` when source freshness is older than 10 minutes.
- [ ] Run `go vet ./...` and `go test ./...`.
- [ ] Commit `feat(W8): source-aware health`.

## W6. `/v1/incidents/{source}/{incident_id}`

- [ ] Add detail DTOs for incident, unit history, and event history.
- [ ] Add store query for one incident plus histories.
- [ ] Write failing handler tests for found and missing incidents.
- [ ] Wire `GET /v1/incidents/{source}/{incident_id}`.
- [ ] Run `go vet ./...` and `go test ./...`.
- [ ] Commit `feat(W6): incident detail endpoint`.

## W3. Contract canary assertions

- [ ] Export Phoenix parser helpers needed by canary.
- [ ] Add canary result model and store persistence helpers.
- [ ] Write failing canary tests for success, missing field, bad geometry, bad date, and parser failure.
- [ ] Implement all six architecture section 7 checks.
- [ ] Wire `cmd/canary` to config, store, checker, and failed-check `log.Error`.
- [ ] Run `go vet ./...` and `go test ./...`.
- [ ] Commit `feat(W3): contract canary checks`.

## W5. Janitor raw JSONB retention

- [ ] Add store method to null old `raw` and set `raw_dropped_at`.
- [ ] Add store method for `VACUUM ANALYZE incidents`.
- [ ] Write failing retention cutoff tests where practical.
- [ ] Wire `cmd/janitor` sweep and logging.
- [ ] Run `go vet ./...` and `go test ./...`.
- [ ] Commit `feat(W5): raw retention janitor`.

## W4. Static web client

- [ ] Add local CORS support to API.
- [ ] Create `web/index.html` with Leaflet map/list shell.
- [ ] Create `web/styles.css` with mobile-friendly layout.
- [ ] Create `web/app.js` fetching `http://localhost:8080/v1/incidents/active`.
- [ ] Smoke-check direct file opening against local API.
- [ ] Run `go vet ./...` and `go test ./...`.
- [ ] Commit `feat(W4): static incident map client`.

## W7. Paid history placeholder

- [ ] Add `PAID_TIER_ENABLED` config field defaulting false.
- [ ] Write failing config and handler tests.
- [ ] Wire `GET /v1/incidents/history`.
- [ ] Return `402` JSON explanation while paid tier is disabled.
- [ ] Run `go vet ./...` and `go test ./...`.
- [ ] Commit `feat(W7): paid history placeholder`.

## Closeout

- [ ] Update `README.md` with v0.2 status, commands, API routes, and web instructions.
- [ ] Add `docs/v0.2-summary.md`.
- [ ] Run `go vet ./...`.
- [ ] Run `go test ./...`.
- [ ] Run `go build ./...`.
- [ ] Run `make db-up && make db-init`.
- [ ] Run ingester + API and curl `/v1/incidents/active`.
- [ ] Confirm `web/index.html` works against `localhost:8080`.

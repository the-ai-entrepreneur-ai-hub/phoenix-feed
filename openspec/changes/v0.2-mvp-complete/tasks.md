# v0.2 MVP Completion Tasks

## W1. `/v1/incidents/active`

- [x] Create internal API response DTOs and staleness metadata.
- [x] Add active-incident filter parsing for `bbox`, `lat`, `lon`, `radius_meters`, `since`, and `until`.
- [x] Write failing handler tests for empty response, valid bbox, valid radius, valid time window, and invalid mixed spatial filters.
- [x] Add `Store.ListActiveIncidents`.
- [x] Wire `GET /v1/incidents/active`.
- [x] Run `go vet ./...` and `go test ./...`.
- [x] Commit `feat(W1): active incidents endpoint`.

## W2. Incident unit and event history writes

- [x] Add unit delta model and diff helper.
- [x] Write failing tests for unit add, remove, status change, and unchanged repeat.
- [x] Make `Store.UpsertIncident` transactional.
- [x] Extend or insert `incident_units` rows during upsert.
- [x] Write `incident_events` rows for `created`, `updated`, and `reopened`.
- [x] Write cleared events after `SweepCleared`.
- [x] Run `go vet ./...` and `go test ./...`.
- [x] Commit `feat(W2): incident unit history`.

## W8. `/v1/health` upgrade

- [x] Add health DTOs.
- [x] Add store query for per-source last success and latest canary.
- [x] Write failing handler tests for `ok`, `degraded`, and stale-source `down`.
- [x] Replace skeleton `/v1/health`.
- [x] Return `503` when source freshness is older than 10 minutes.
- [x] Run `go vet ./...` and `go test ./...`.
- [x] Commit `feat(W8): source-aware health`.

## W6. `/v1/incidents/{source}/{incident_id}`

- [x] Add detail DTOs for incident, unit history, and event history.
- [x] Add store query for one incident plus histories.
- [x] Write failing handler tests for found and missing incidents.
- [x] Wire `GET /v1/incidents/{source}/{incident_id}`.
- [x] Run `go vet ./...` and `go test ./...`.
- [x] Commit `feat(W6): incident detail endpoint`.

## W3. Contract canary assertions

- [x] Export Phoenix parser helpers needed by canary.
- [x] Add canary result model and store persistence helpers.
- [x] Write failing canary tests for success, missing field, bad geometry, bad date, and parser failure.
- [x] Implement all six architecture section 7 checks.
- [x] Wire `cmd/canary` to config, store, checker, and failed-check `log.Error`.
- [x] Run `go vet ./...` and `go test ./...`.
- [x] Commit `feat(W3): contract canary checks`.

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

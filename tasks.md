# Cactus Watch Data Quality Overhaul Tasks

## Goal
Ship the Phoenix Fire data quality fixes and public API polish on `feat/data-quality-overhaul` without new paid infrastructure or new third-party Go dependencies.

## Constraints
- Keep all v1 response shapes backwards compatible; only add fields.
- Do not modify or commit local secret files.
- Push only `feat/data-quality-overhaul`.
- Use existing droplet resources and current Go dependencies.

## Execution Plan
- [x] A1: Add failing parser tests for Phoenix space-delimited unit/status pairs, empty units, weird spacing, NB hyphen unit names, and production smashed samples.
- [x] A1: Replace comma-split `parseUnits` with whitespace token state machine and keep legacy comma sample working.
- [x] A3/A4/B4: Add nature description overrides, non-nil empty units, and `unit_type` derivation in parser/model with tests.
- [x] B3: Add severity derivation to active/detail incident response structs with API tests.
- [x] B1: Add `Store.Stats` SQL query and `GET /v1/stats` handler with tests.
- [x] B2: Add code dictionary helpers and `GET /v1/codes` handler with tests.
- [x] B5: Add concise static OpenAPI document at `GET /v1/openapi.json` with tests.
- [x] A5: Add root JSON handler at `GET /` with tests.
- [x] B6: Change canary zero-feature behavior to require three consecutive zero checks before failure; single zero logs/stores INFO-level drift only.
- [ ] A2: Add `cmd/backfill_units` one-shot command that re-parses raw Phoenix `Units`, updates `incidents.units`, rebuilds `incident_units`, and reports scanned/updated/smashed counts.
- [ ] C1/C2: Update README and architecture docs for new endpoints and fields if Group A+B fit within budget.
- [ ] Verify: run targeted tests, `go test ./...`, `go build ./...`, `go mod tidy && git diff go.mod go.sum`.
- [ ] Deploy: push branch, pull on droplet, rebuild `api ingester canary janitor`, run backfill, capture `docs/backfill-2026-05-05.log`.
- [ ] Verify live: root, active data labels/units, `/v1/stats`, `/v1/codes`, `/v1/openapi.json`, compose health.
- [ ] Report: write `docs/codex-overnight-report-2026-05-05.md` with commit list, changed files, tests, backfill results, live proof, and PR-ready summary.

## Files Expected To Change
| File | Purpose | Risk |
| --- | --- | --- |
| `internal/source/phxfire/phxfire.go` | Parser fixes, overrides, code/severity/unit-type helpers | Medium: source normalization touches all incidents |
| `internal/source/phxfire/phxfire_test.go` | Parser and enrichment regression tests | Low |
| `internal/model/incident.go` | Add `unit_type` JSON field | Low: additive |
| `internal/store/store.go` | Add stats query, severity fields, non-nil decoded units | Medium: SQL and public JSON shape |
| `internal/api/*.go` | Root/stats/codes/openapi handlers and tests | Low/medium: public endpoints |
| `internal/canary/*.go` | Three-zero canary rule and tests | Low |
| `cmd/backfill_units/main.go` | Production one-shot backfill | Medium: writes production incident snapshots/history |
| `deploy/docker/api.Dockerfile` | Include backfill binary in API image if needed | Low |
| `README.md`, `docs/architecture.md` | Document new behavior | Low |

## Test Strategy
- Follow RED/GREEN for parser, API endpoints, canary behavior, and backfill parsing helpers.
- Run targeted tests after each group before committing.
- Run full `go test ./...` and `go build ./...` before deploy.
- Use live curl/SSH checks only after branch push and deploy.

## Rollback
If deploy or backfill fails, stop containers on the feature branch and checkout the previous production branch/commit on the droplet with `git checkout main && git pull --ff-only`, then rebuild the same services from the previously deployed code.

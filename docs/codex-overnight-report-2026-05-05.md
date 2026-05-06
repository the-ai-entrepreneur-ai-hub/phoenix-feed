# Cactus Watch Data Quality Overhaul — Complete

## 1. Branch and commits

Branch: `feat/data-quality-overhaul`

Commits:

- `0f8e7fe` Merge remote-tracking branch `origin/main` into `feat/data-quality-overhaul`
- `a3734f5` chore(ops): add production healthchecks
- `3b40e1c` docs: document data quality endpoints
- `81b3cf1` chore: tidy module sums
- `7c2caf9` feat(backfill): add units repair command
- `6979558` fix(canary): tolerate transient zero feature polls
- `3526335` feat(api): add public stats codes and docs endpoints
- `a379d58` fix(phxfire): parse Phoenix unit snapshots correctly
- `f195df2` docs: add data quality execution plan

## 2. Files changed

Before this report/log commit: 23 files changed, 1,289 insertions, 56 deletions.

Added:
- `cmd/backfill_units/main.go`
- `internal/api/enrichment.go`
- `internal/api/public.go`
- `internal/api/public_test.go`
- `internal/backfill/backfill.go`
- `internal/backfill/backfill_test.go`
- `tasks.md`

Modified:
- `README.md`
- `deploy/docker/api.Dockerfile`
- `docker-compose.prod.yml`
- `docs/architecture.md`
- `go.sum`
- `internal/api/active.go`
- `internal/api/active_test.go`
- `internal/api/detail.go`
- `internal/api/detail_test.go`
- `internal/api/router.go`
- `internal/canary/canary.go`
- `internal/canary/canary_test.go`
- `internal/model/incident.go`
- `internal/source/phxfire/phxfire.go`
- `internal/source/phxfire/phxfire_test.go`
- `internal/store/store.go`

Deleted: none.

## 3. Tests added

Added 15 test functions.

- Parser: 10-case table for single unit, legacy comma tolerance, production smashed samples, weird spacing, empty input, whitespace-only input, no-colon best effort, and Unicode non-breaking hyphen in `M‑174`.
- Parser JSON: empty units serialize as `[]`.
- Parser labels: 962/962BC overrides apply, meaningful Phoenix labels are preserved.
- API: empty units render as `[]`, active/detail responses add `severity`, root/stats/codes/openapi endpoints return documented shapes.
- Canary: one zero-feature response passes with informational drift; three consecutive zero-feature checks fail.
- Backfill helpers: raw Phoenix feature units parse correctly, empty raw units are non-nil, smashed status detection works.

Verification:

```text
go test ./...
ok / no failures

go build ./...
exit 0

go mod tidy && git diff -- go.mod go.sum
no diff
```

## 4. Backfill results

Backfill log saved at `docs/backfill-2026-05-05.log`.

Command output:

```text
backfill_units complete rows_processed=116 rows_updated=116 before_smashed=0 after_smashed=0 flipped_to_multi_unit=0 elapsed=172ms
```

Independent SQL captured immediately before deploy/backfill:

```text
before_smashed_count=11
```

Independent SQL after backfill:

```text
after_smashed_count=0
```

The five sampled broken incidents below all changed from one smashed unit/status object to multi-unit arrays. Note: the successful command invocation reported `before_smashed=0`; the pre-backfill SQL above is the reliable before count captured before any production mutation.

## 5. Side-by-side proof

| Incident | v1 broken JSON | v2 fixed JSON |
| --- | --- | --- |
| `F26200309` | `{"units":[{"Unit":"BR701","Status":"Responding E701: Responding"}]}` | `{"units":[{"Unit":"BR701","Status":"Responding","unit_type":"Brush truck"},{"Unit":"E701","Status":"Responding","unit_type":"Engine"}]}` |
| `F26200313` | `{"nature_code":"962A","nature_desc":"962","units":[{"Unit":"E185","Status":"Responding R185: Responding"}]}` | `{"nature_code":"962A","nature_desc":"Vehicle Crash","units":[{"Unit":"E185","Status":"Responding","unit_type":"Engine"},{"Unit":"R185","Status":"Responding","unit_type":"Rescue"}]}` |
| `F26200326` | `{"units":[{"Unit":"E41","Status":"On Scene HM41: On Scene"}]}` | `{"units":[{"Unit":"E41","Status":"On Scene","unit_type":"Engine"},{"Unit":"HM41","Status":"On Scene","unit_type":"Hazmat unit"}]}` |
| `F26200424` | `{"nature_code":"962P","nature_desc":"962 INVOLVING PEDEST","units":[{"Unit":"E45","Status":"Dispatched R45: Dispatched"}]}` | `{"nature_code":"962P","nature_desc":"Crash Involving Pedestrian","units":[{"Unit":"E45","Status":"Dispatched","unit_type":"Engine"},{"Unit":"R45","Status":"Dispatched","unit_type":"Rescue"}]}` |
| `F26200725` | `{"units":[{"Unit":"E58","Status":"Responding R58: Responding"}]}` | `{"units":[{"Unit":"E58","Status":"Responding","unit_type":"Engine"},{"Unit":"R58","Status":"Responding","unit_type":"Rescue"}]}` |

Live active-unit proof after deploy included multi-unit active incidents such as `F26201218` with 11 separate units and no smashed status chain.

## 6. New endpoints

Root:

```json
{"docs":"https://feed.cactuswatch.com/v1/openapi.json","health":"https://feed.cactuswatch.com/v1/health","name":"cactus-watch-feed","version":"v1"}
```

Stats:

```json
{"current_active_count":17,"today_total_incidents":124,"last_24h_total":124,"active_units_now":36,"data_age_seconds":2,"tier":"free"}
```

Codes:

```json
{"version":"phx-fire-2026-05","codes":[{"code":"962","label":"Vehicle Crash","category":"traffic"},{"code":"962BC","label":"Crash Involving Bicycle","category":"traffic"}]}
```

OpenAPI:

```json
{"openapi":"3.0.3","info":{"title":"Cactus Watch Feed API","version":"v1"},"paths":{"/v1/stats":{},"/v1/codes":{},"/v1/openapi.json":{}}}
```

Live checks:

```text
GET / -> HTTP_STATUS=200
GET /v1/stats -> HTTP 200
GET /v1/codes -> HTTP_STATUS=200
GET /v1/openapi.json -> HTTP_STATUS=200
GET /v1/health -> {"state":"ok","db_reachable":true,...}
```

Active 962 label check from production:

```text
"Vehicle Crash"
"Vehicle Crash"
"Vehicle Crash"
"Vehicle Crash"
"Vehicle Crash"
```

Container health after final no-build recreate:

```text
app-api-1      Up (healthy)
app-caddy-1    Up (healthy)
app-canary-1   Up (healthy)
app-db-1       Up (healthy)
app-ingester-1 Up (healthy)
app-janitor-1  Up (healthy)
```

## 7. Worth flagging

- The production checkout has a pre-existing local modification: `scripts/pg_backup_local.sh`. I left it untouched.
- `docker compose run --rm api /usr/local/bin/backfill_units` starts the API entrypoint with the path as an argument. I ran the repair with `--entrypoint /usr/local/bin/backfill_units`.
- The first rebuild saturated the $21.60/mo droplet for several minutes. I recovered with a no-build recreate and added lightweight healthchecks so future `ps` output is clearer.
- `/v1/stats` uses the same free read rate limit as active incidents. Verification needs a unique `X-Client-ID` or it can return `429`.
- The live dataset now includes codes not in the initial audit (`962HM`, `A2`, `ALLEY`). They fall through as labels in stats and `severity:"unknown"` unless explicitly mapped later.

## 8. PR-ready summary

```markdown
## Summary
- Fix Phoenix unit parsing by replacing comma splitting with a whitespace token state machine, preserving multi-word statuses and deriving `unit_type`.
- Normalize bare/truncated Phoenix crash labels, add additive `severity`, and guarantee empty unit snapshots serialize as `[]`.
- Add public `/`, `/v1/stats`, `/v1/codes`, and `/v1/openapi.json` endpoints.
- Add `cmd/backfill_units` and run it in production to repair historical `incidents.units` and `incident_units`.
- Make canary zero-feature alerts require 3 consecutive zero checks, and add production healthchecks for all compose services.

## Verification
- `go test ./...`
- `go build ./...`
- `go mod tidy && git diff -- go.mod go.sum` produced no diff.
- Production `/v1/health` returned `state:"ok"`.
- Production smashed-unit SQL count dropped from 11 to 0.
- `/`, `/v1/stats`, `/v1/codes`, `/v1/openapi.json` returned 200.
- `docker compose ps` showed all 6 services healthy.

## Notes
- No new third-party Go dependencies.
- No new infrastructure or paid services.
- Backwards-compatible v1 response changes only; new fields are additive.
```

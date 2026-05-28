# Backend Integrity Audit - 2026-05-28

| Rank | Severity | Finding |
| --- | --- | --- |
| 1 | P1 silent corruption | SDR-promoted incidents never clear from the active feed |
| 2 | P1 silent corruption | Lifecycle applies successful polls after incident write failures |
| 3 | P1 silent corruption | Transient geocode errors permanently consume transcripts |
| 4 | P1 silent corruption | SQL migration/deploy commands can continue after statement errors |
| 5 | P2 correctness edge | Freshness metadata is global and can mask per-source staleness |
| 6 | P2 correctness edge | Worker healthchecks prove only that the process exists |
| 7 | P2 resource exhaustion | Parser holds row locks and DB connections while calling Mapbox |
| 8 | P2 resource exhaustion | Upstream response bodies are unbounded in ingester and canary |
| 9 | P2 correctness edge | Invalid env values silently fall back to defaults |
| 10 | P2 correctness edge | Cleanup migration rewrites SDR nature data by heuristic |

## Audit Baseline

- Branch/head verified before finalizing: `main` and `origin/main` both at `a9f1e035457c164b2224d9e38511a0aaf9145932`.
- `origin/main` advanced during the audit from `980a9fc` to `a9f1e03`; final parser and migration evidence was rechecked against `a9f1e03`.
- Production schema was checked by SSH with `pg_dump --schema-only`. I found no application table, column, or index drift from `origin/main` `db/schema.sql` plus migrations `0002` through `0005`; `0005` is data-only. Production also has PostGIS support schemas/extensions (`tiger`, `topology`, `fuzzystrmatch`, `postgis_tiger_geocoder`, `postgis_topology`) not represented as app migrations.
- Production `incidents_id_seq` covers the current max id: observed `last_value=3539`, `max(id)=3534`, `sequence_covers_max=true`.
- I did not find missing `rows.Close()` on production `Query` call sites.
- I did not find a raw SQL injection path in production code. Dynamic incident filters build positional placeholders, and `dayStart` is a constant expression.
- Admin dispatch upload rate limiting is per bearer token: `internal/api/admin_dispatch_transcripts.go:115` hashes the token into `Identity.KeyHash`, and `internal/ratelimit/limiter.go:105` keys buckets by hash.
- `ADMIN_TOKEN` was not found in runtime log fields. Admin access logs include request id, method, path, status, and latency only.
- The dispatch `FOR UPDATE SKIP LOCKED` claim is correct for concurrent parser workers: the row is selected in a transaction at `internal/dispatch/parser/worker.go:151` and locked at `internal/dispatch/parser/worker.go:209`. If the process crashes before commit, the transaction rolls back and `parsed_at` remains null.
- `MaxBytesReader` covers the dispatch transcript POST body at `internal/api/admin_dispatch_transcripts.go:177`; `/v1/incidents/refresh` is a POST but does not read a body.

## SDR-promoted incidents never clear from the active feed

Severity: P1 silent corruption

Files affected:
- `internal/dispatch/parser/worker.go:242`
- `internal/dispatch/parser/worker.go:263`
- `internal/dispatch/parser/worker.go:264`
- `db/schema.sql:144`
- `db/schema.sql:155`
- `cmd/ingester/main.go:89`
- `cmd/ingester/main.go:92`
- `internal/store/active_view_test.go:21`

Reproduction or evidence:
- The active view includes every row where `cleared_at IS NULL` (`db/schema.sql:144`, `db/schema.sql:155`).
- The SDR parser inserts incidents directly and does not set a clear time (`internal/dispatch/parser/worker.go:242`). On conflict it explicitly resets `missing_since` and `cleared_at` to null (`internal/dispatch/parser/worker.go:263`, `internal/dispatch/parser/worker.go:264`).
- The only lifecycle clearer is driven by the ingester, and the ingester only returns `phxfire.New()` (`cmd/ingester/main.go:89`, `cmd/ingester/main.go:92`).
- The active-view test explicitly bans an incident-date/age cutoff (`internal/store/active_view_test.go:21`).
- Live production evidence on 2026-05-28: `SELECT source, COUNT(*), MIN(incident_date), MAX(incident_date), COUNT(*) FILTER (WHERE cleared_at IS NULL) FROM incidents GROUP BY source` returned `sdr_audio|20|2026-04-28 21:40:06+00|2026-04-30 08:51:48+00|20`. A sample of active SDR rows had April `incident_date` values but May 28 `last_seen_at` values.

Recommended fix:
- Give SDR incidents an explicit active-window policy. For example, set `cleared_at = captured_at + interval '90 minutes'` during promotion, or add an SDR-specific janitor that clears `source='sdr_audio'` rows after a configured TTL.
- Prefer a durable rule in the database or store layer, not only API filtering, so `/v1/incidents/active`, stats, and detail views agree.
- Add an integration test that promotes an SDR transcript, advances time beyond the TTL, runs the clear path, and verifies the row is absent from `active_incidents`.

Effort estimate: M

## Lifecycle applies successful polls after incident write failures

Severity: P1 silent corruption

Files affected:
- `internal/lifecycle/lifecycle.go:36`
- `internal/lifecycle/lifecycle.go:54`
- `internal/lifecycle/lifecycle.go:56`
- `internal/lifecycle/lifecycle.go:61`
- `internal/lifecycle/lifecycle.go:65`
- `internal/lifecycle/lifecycle.go:75`
- `internal/lifecycle/lifecycle.go:81`
- `internal/store/incident_history.go:35`
- `internal/store/store.go:718`
- `internal/store/store.go:732`

Reproduction or evidence:
- A successful poll is recorded first (`internal/lifecycle/lifecycle.go:36`).
- Each incident upsert is atomic on its own (`internal/store/incident_history.go:35`), but the poll is not atomic as a whole.
- If an incident upsert fails, the error is logged and the loop continues (`internal/lifecycle/lifecycle.go:54`, `internal/lifecycle/lifecycle.go:56`).
- The failed incident is not added to `observedIDs`, then `MarkMissing` and `SweepCleared` still run (`internal/lifecycle/lifecycle.go:61`, `internal/lifecycle/lifecycle.go:65`).
- `Apply` logs `"poll applied"` and returns nil even after upsert, mark-missing, sweep, or cleared-event failures (`internal/lifecycle/lifecycle.go:75`, `internal/lifecycle/lifecycle.go:81`).

Recommended fix:
- Treat any incident write failure in a successful poll as a hard poll-apply failure. Do not run `MarkMissing` or `SweepCleared` unless every observed incident was persisted.
- Consider one transaction for `RecordPoll`, all incident upserts, mark-missing, sweep, and clear-event writes. If that is too large, at minimum return an error and skip clearing on partial writes.
- Add a regression test where one `UpsertIncident` fails and assert no existing incident is marked missing or cleared.

Effort estimate: M

## Transient geocode errors permanently consume transcripts

Severity: P1 silent corruption

Files affected:
- `internal/dispatch/parser/worker.go:181`
- `internal/dispatch/parser/worker.go:183`
- `internal/dispatch/parser/worker.go:184`
- `internal/dispatch/parser/worker.go:190`
- `internal/dispatch/parser/worker.go:304`
- `internal/dispatch/parser/worker.go:307`
- `internal/geocode/cache.go:39`
- `internal/geocode/cache.go:53`
- `internal/geocode/cache.go:56`
- `internal/geocode/geocode.go:125`
- `internal/geocode/geocode.go:130`

Reproduction or evidence:
- After parser gate success, any `Geocode` error is logged only at debug (`internal/dispatch/parser/worker.go:181`, `internal/dispatch/parser/worker.go:183`).
- The worker then marks the transcript parsed with no incident id (`internal/dispatch/parser/worker.go:184`, `internal/dispatch/parser/worker.go:304`, `internal/dispatch/parser/worker.go:307`) and returns `outcomeGeocodeFailed` (`internal/dispatch/parser/worker.go:190`).
- The geocoder can return transient errors from cache lookup (`internal/geocode/cache.go:39`), HTTP transport (`internal/geocode/geocode.go:125`), or Mapbox non-2xx responses (`internal/geocode/geocode.go:130`).
- The cache stores a negative result for every provider error (`internal/geocode/cache.go:53`, `internal/geocode/cache.go:56`), so a temporary Mapbox outage can become a 24-hour no-result cache entry.
- Live production evidence: `dispatch_transcripts` currently has `0` unparsed rows, `10402` parsed rows with no incident id, and `20` parsed rows with an incident id. The schema does not store the failure reason, so transient drops cannot be distinguished afterward.

Recommended fix:
- Split geocode outcomes into permanent `ErrNoResult` and retryable errors. Only permanent no-result should set `parsed_at`.
- Do not negative-cache transport, timeout, rate-limit, 5xx, context-canceled, or cache-layer errors.
- Add `parse_status`, `parse_error`, and `parse_attempts` columns so the worker can retry transient failures with a cap and operators can audit why rows were not promoted.
- Raise retryable geocode failures to warn or error, with counters in dispatch health.

Effort estimate: M

## SQL migration/deploy commands can continue after statement errors

Severity: P1 silent corruption

Files affected:
- `cloud-init.yml:123`
- `cloud-init.yml:132`
- `Makefile:57`
- `Makefile:64`
- `scripts/smoke.sh:42`
- `scripts/smoke.ps1:41`
- `docs/deployment.md:59`
- `docs/deployment.md:60`

Reproduction or evidence:
- Cloud-init applies schema and migrations through stdin psql without `ON_ERROR_STOP` or `--single-transaction` (`cloud-init.yml:123`, `cloud-init.yml:132`).
- The Makefile and smoke scripts use the same pattern (`Makefile:57`, `Makefile:64`, `scripts/smoke.sh:42`, `scripts/smoke.ps1:41`).
- Deployment docs copy the same production command (`docs/deployment.md:59`, `docs/deployment.md:60`).
- I verified the behavior against the production Postgres container with a read-only failing `SELECT`: piping `SELECT 1; SELECT 1/0; SELECT 2;` into psql printed `ERROR: division by zero`, still ran `SELECT 2`, and exited `remote_exit:0`.

Recommended fix:
- Apply every schema file with `psql -v ON_ERROR_STOP=1 --single-transaction`.
- For deployment loops, stop immediately on the first failed migration and log the migration filename.
- Add a smoke test that feeds an intentionally failing SQL file to the migration helper and asserts a non-zero exit.

Effort estimate: S

## Freshness metadata is global and can mask per-source staleness

Severity: P2 correctness edge

Files affected:
- `internal/store/store.go:548`
- `internal/store/store.go:550`
- `internal/store/store.go:552`
- `internal/store/store.go:562`
- `internal/store/store.go:792`
- `internal/store/store.go:794`
- `internal/store/store.go:796`
- `internal/api/active.go:60`
- `internal/api/active.go:80`
- `cmd/api/main.go:50`

Reproduction or evidence:
- `activeStalenessMeta` selects the single newest successful poll across all sources (`internal/store/store.go:548`, `internal/store/store.go:550`, `internal/store/store.go:552`) and computes one `data_age_seconds` (`internal/store/store.go:562`).
- `PublicStats` does the same global latest-success lookup (`internal/store/store.go:792`, `internal/store/store.go:794`, `internal/store/store.go:796`).
- `applyActiveIncidentFreshnessMeta` reports one newest incident timestamp across the response, not per source (`internal/api/active.go:60`, `internal/api/active.go:80`).
- The API health source list is hard-coded to Phoenix Fire MapServer only (`cmd/api/main.go:50`), so `sdr_audio` freshness is absent from `/v1/health`.
- With two sources, a fast or recently reprocessed source can make aggregate metadata look fresh while the other source is stale or stopped.

Recommended fix:
- Return per-source freshness fields, for example `sources: { phoenix-fire-mapserver: {...}, sdr_audio: {...} }`, plus an explicit aggregate policy.
- Include `sdr_audio` in API health configuration.
- For active feed meta, calculate staleness per source present in the response and per configured source, not only newest global values.

Effort estimate: M

## Worker healthchecks prove only that the process exists

Severity: P2 correctness edge

Files affected:
- `docker-compose.prod.yml:63`
- `docker-compose.prod.yml:64`
- `docker-compose.prod.yml:81`
- `docker-compose.prod.yml:82`
- `docker-compose.prod.yml:88`
- `docker-compose.prod.yml:120`

Reproduction or evidence:
- Ingester, parser, canary, and janitor healthchecks are `grep -q /usr/local/bin/... /proc/1/cmdline` checks (`docker-compose.prod.yml:63`, `docker-compose.prod.yml:64`, `docker-compose.prod.yml:81`, `docker-compose.prod.yml:82`).
- A parser process that is alive but marking every transcript geocode-failed, a canary that cannot write drift rows, or an ingester stuck in backoff still looks healthy to Docker.
- This is especially risky because the API health endpoint only checks configured sources and currently omits `sdr_audio`.

Recommended fix:
- Replace process-name checks with state checks:
  - parser: fail if backlog age or retryable error count exceeds threshold.
  - ingester: fail if latest successful poll per source is stale.
  - canary: fail if latest canary check is stale or failed.
  - janitor: fail if no successful sweep heartbeat within expected interval.
- Store heartbeats in Postgres or expose local health endpoints that validate recent progress.

Effort estimate: M

## Parser holds row locks and DB connections while calling Mapbox

Severity: P2 resource exhaustion

Files affected:
- `internal/dispatch/parser/worker.go:151`
- `internal/dispatch/parser/worker.go:156`
- `internal/dispatch/parser/worker.go:209`
- `internal/dispatch/parser/worker.go:215`
- `internal/dispatch/parser/worker.go:181`
- `internal/dispatch/parser/worker.go:187`
- `internal/geocode/geocode.go:96`
- `internal/geocode/geocode.go:125`
- `internal/store/store.go:216`
- `cmd/dispatch-parser/main.go:34`

Reproduction or evidence:
- `processNext` begins a transaction before locking a transcript row (`internal/dispatch/parser/worker.go:151`, `internal/dispatch/parser/worker.go:209`, `internal/dispatch/parser/worker.go:215`).
- The external geocoder call happens before the transaction commits (`internal/dispatch/parser/worker.go:181`, `internal/dispatch/parser/worker.go:187`).
- The geocoder can block on the outbound rate limiter (`internal/geocode/geocode.go:96`) and on the HTTP call (`internal/geocode/geocode.go:125`).
- Store and parser pools use `pgxpool.New` with default pool sizing (`internal/store/store.go:216`, `cmd/dispatch-parser/main.go:34`). The installed pgx source says the default max pool size is the greater of 4 or `runtime.NumCPU()`.

Recommended fix:
- Claim work quickly in a short transaction by setting a `parse_claimed_at`/`parse_owner` field, commit, then geocode outside the transaction.
- Finish in a second transaction that verifies ownership and writes either promoted incident or retryable failure state.
- Configure explicit `pool_max_conns` per process in `DATABASE_URL` or pgx config, with a production budget across API, ingester, parser, canary, janitor, and any future parser replicas.

Effort estimate: M

## Upstream response bodies are unbounded in ingester and canary

Severity: P2 resource exhaustion

Files affected:
- `internal/source/phxfire/phxfire.go:155`
- `internal/canary/canary.go:172`
- `internal/api/fire_stations.go:53`
- `internal/api/admin_dispatch_transcripts.go:177`

Reproduction or evidence:
- Phoenix Fire polling reads the entire upstream response with `io.ReadAll(resp.Body)` (`internal/source/phxfire/phxfire.go:155`).
- The contract canary does the same (`internal/canary/canary.go:172`).
- The fire-stations proxy uses a 5 MiB `LimitReader` (`internal/api/fire_stations.go:53`), but the ingester and canary do not have comparable caps.
- The dispatch POST body is capped (`internal/api/admin_dispatch_transcripts.go:177`), so the gap is upstream reads rather than the admin upload route.

Recommended fix:
- Add explicit max response sizes for MapServer and canary fetches, with a clear error if the limit is exceeded.
- Use a helper that reads `limit + 1` bytes so truncation can be detected instead of parsing partial JSON.
- Record oversized upstream payloads as failed polls/canary drift with error details.

Effort estimate: S

## Invalid env values silently fall back to defaults

Severity: P2 correctness edge

Files affected:
- `internal/config/config.go:76`
- `internal/config/config.go:78`
- `internal/config/config.go:82`
- `internal/config/config.go:85`
- `internal/config/config.go:87`
- `internal/config/config.go:91`
- `internal/config/config.go:94`
- `internal/config/config.go:103`

Reproduction or evidence:
- `envInt` returns the fallback when parsing fails (`internal/config/config.go:76`, `internal/config/config.go:78`, `internal/config/config.go:82`).
- `envDuration` does the same (`internal/config/config.go:85`, `internal/config/config.go:87`, `internal/config/config.go:91`).
- `envBool` also falls back on unrecognized values (`internal/config/config.go:94`, `internal/config/config.go:103`).
- A typo like `POLL_INTERVAL=6Os`, `RAW_RETENTION=thirty-days`, or `PAID_TIER_ENABLED=ture` silently changes runtime behavior instead of failing startup.

Recommended fix:
- Make env parsing return `(value, error)` and fail `Load` when a present env var is malformed.
- Keep fallback behavior only for absent variables, not invalid ones.
- Add config tests for malformed int, duration, and bool inputs.

Effort estimate: S

## Cleanup migration rewrites SDR nature data by heuristic

Severity: P2 correctness edge

Files affected:
- `db/migrations/0005_cleanup_bad_natures.sql:1`
- `db/migrations/0005_cleanup_bad_natures.sql:2`
- `db/migrations/0005_cleanup_bad_natures.sql:3`
- `db/migrations/0005_cleanup_bad_natures.sql:4`
- `db/migrations/0005_cleanup_bad_natures.sql:5`

Reproduction or evidence:
- Migration `0005` updates production incident data in place (`db/migrations/0005_cleanup_bad_natures.sql:1`).
- It keeps only the text before the first comma for every `sdr_audio` row whose `nature_desc` is longer than 50 chars and contains a comma (`db/migrations/0005_cleanup_bad_natures.sql:2`, `db/migrations/0005_cleanup_bad_natures.sql:3`, `db/migrations/0005_cleanup_bad_natures.sql:4`, `db/migrations/0005_cleanup_bad_natures.sql:5`).
- The predicate is a heuristic, not tied to a parser version, parsed transcript id range, or a saved preimage. If a legitimate dispatch nature contains a comma and is longer than 50 chars, the original value is lost.
- The production deploy path currently applies migrations through stdin psql without `ON_ERROR_STOP` or `--single-transaction`, so this data rewrite also inherits the partial-apply risk described above.

Recommended fix:
- Make data cleanup migrations auditable and reversible: select candidate ids first, store old/new values in a migration audit table, then update by id.
- Constrain cleanup to rows known to have been produced by the buggy parser version or by an explicit candidate list reviewed before deploy.
- Add a dry-run query to deployment notes showing candidate count and sample rows before applying destructive data migrations.

Effort estimate: S

## Admin token compare is not fixed-length despite ConstantTimeCompare

Severity: P3 polish

Files affected:
- `internal/api/admin_recent.go:70`
- `internal/api/admin_recent.go:75`
- `internal/api/admin_recent.go:76`
- `internal/api/admin_dispatch_transcripts.go:115`
- `internal/api/admin_dispatch_transcripts.go:117`

Reproduction or evidence:
- The validator uses `subtle.ConstantTimeCompare` (`internal/api/admin_recent.go:76`), which is good for equal-length byte slices.
- It compares raw bearer tokens directly, so length mismatches and the number of configured comma-separated candidates still influence timing (`internal/api/admin_recent.go:70`, `internal/api/admin_recent.go:75`).
- The admin dispatch limiter already hashes the bearer token for bucket identity (`internal/api/admin_dispatch_transcripts.go:115`, `internal/api/admin_dispatch_transcripts.go:117`), so fixed-length hash comparison would fit the existing pattern.

Recommended fix:
- Store/admin-configure SHA-256 token hashes and compare fixed-length decoded hashes with `ConstantTimeCompare`.
- Keep support for multiple admin tokens by comparing against fixed-length hashes and avoid early returns based on candidate length.

Effort estimate: S

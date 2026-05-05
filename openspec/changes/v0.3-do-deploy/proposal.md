## Why

v0.2 proved the local Phoenix incident cache loop, but Cactus Alert now needs deployable infrastructure and server-enforced freemium rules before the iOS client can rely on it. v0.3 adds the smallest production-ready surface for DigitalOcean: light API key auth, per-tier cadence enforcement, pull-to-refresh throttling, deployment artifacts, smoke tests, and a runbook the owner can execute manually.

## Scope

In scope:

- API key table, lookup, and `cmd/keygen`.
- In-memory per-client rate limiting for active/detail/manual refresh endpoints.
- Manual refresh route that reads cache only.
- Cactus-facing metadata strings and refresh hints in active/refresh responses.
- Dev/prod Docker Compose files, per-binary Dockerfiles, `.env.example`, and Caddy reverse proxy.
- DigitalOcean cloud-init artifact.
- Local smoke scripts for Bash and PowerShell.
- Deployment and v0.3 summary docs.

Out of scope:

- Billing, Stripe, subscription state, app-store receipt validation, notifications, geofences, scanner+Whisper, managed Postgres, Terraform, Redis, and real production alerting.

## Workstreams

### W1. Per-tier rate limiting

Scope: add server-side rate limiting to `GET /v1/incidents/active` and `GET /v1/incidents/{source}/{incident_id}`. Free tier allows one request per 10 minutes per API key, `X-Client-ID`, or IP fallback. Paid tier allows one request per 50 seconds per API key. `/v1/health` bypasses the limiter. Use an in-memory limiter and document Redis as the later multi-instance swap.

Acceptance criteria:

- Anonymous clients are treated as free.
- Free keyed or anonymous clients receive `429` on the second active/detail request within 10 minutes.
- Paid API keys receive the paid cadence.
- `Retry-After` is set on `429`.
- `/v1/health` is not rate limited.
- A concurrent integration-style test verifies only one request is admitted for a free burst.

Files affected: `internal/api`, new `internal/ratelimit`, `cmd/api/main.go`, `go.mod`, tests, `docs/architecture.md`.

Test plan: table-driven limiter tests; API middleware tests for anonymous/free/paid/health; concurrent burst test; `go vet ./...`; `go test ./...`.

Risk: in-memory buckets reset on process restart and do not coordinate across multiple API replicas. v0.3 explicitly runs one API process; Redis is documented for horizontal scale.

Tasks:

- [ ] Add `golang.org/x/time/rate`.
- [ ] Add tier-aware limiter package.
- [ ] Add concurrent burst test.
- [ ] Add API middleware around active/detail routes.
- [ ] Return `429` with `Retry-After`.
- [ ] Run vet/test and commit `feat(W1): tier-aware rate limits`.

### W2. Manual refresh endpoint

Scope: add `POST /v1/incidents/refresh`. The route does not poll Phoenix; it runs the same cache read as `/v1/incidents/active` and returns the same response shape. It is throttled per resolved client identity to one request per 120 seconds.

Acceptance criteria:

- `POST /v1/incidents/refresh` exists.
- It accepts the same filters as active where meaningful.
- It returns active incident cache data, not a Phoenix poll result.
- It returns `refresh_min_seconds: 120`.
- The second request from the same client within 120 seconds returns `429` with `Retry-After`.

Files affected: `internal/api`, `internal/ratelimit`, tests.

Test plan: handler tests for response shape and throttling; verify the fake store active query is called; `go vet ./...`; `go test ./...`.

Risk: client naming can imply upstream freshness. Route docs and response metadata will say it reads cache only.

Tasks:

- [ ] Add manual refresh limiter.
- [ ] Write failing refresh route tests.
- [ ] Reuse active cache query path.
- [ ] Wire `POST /v1/incidents/refresh`.
- [ ] Run vet/test and commit `feat(W2): manual refresh endpoint`.

### W3. API key auth

Scope: add light API key auth with a new `api_keys` table and middleware. `X-API-Key` is optional. If present, hash the key, look it up, reject revoked/unknown keys, and attach the resolved tier. If absent, treat as anonymous free and resolve identity from `X-Client-ID` or remote IP. Add `cmd/keygen` to generate a random key, print it once, hash it, and store it with `tier`, `label`, and `owner_email`.

Acceptance criteria:

- Migration `db/migrations/0002_api_keys.sql` creates `api_keys`.
- Store lookup returns `free` or `paid` and ignores revoked keys.
- Missing key resolves to anonymous free.
- Invalid key returns `401`.
- `cmd/keygen` can create free or paid keys and prints the secret once.
- No billing logic is added.

Files affected: `db/migrations/0002_api_keys.sql`, `internal/store`, `internal/api`, new `internal/auth`, new `cmd/keygen`, `docs/architecture.md`, tests.

Test plan: hash tests, middleware tests, store SQL covered by integration path when DB is available, keygen unit helpers, `go vet ./...`, `go test ./...`.

Risk: key storage must never store plaintext. Use `crypto/rand` plus SHA-256 hash; print the plaintext only from `cmd/keygen`.

Tasks:

- [ ] Add API key migration.
- [ ] Add auth types and SHA-256 helpers.
- [ ] Write failing middleware tests for anonymous, valid, invalid, and revoked behavior.
- [ ] Add store API key lookup/create helpers.
- [ ] Add `cmd/keygen`.
- [ ] Run vet/test and commit `feat(W3): api key auth and keygen`.

### W4. Dockerfiles + production Compose

Scope: provide one multi-stage Dockerfile per binary (`api`, `ingester`, `canary`, `janitor`, `keygen`) plus `docker-compose.prod.yml`, `.env.example`, Caddy config, service healthchecks, restart policy, and log rotation. Rename current `docker-compose.yml` to `docker-compose.dev.yml` and keep dev usage working.

Acceptance criteria:

- `docker-compose.dev.yml` starts the local PostGIS service.
- Each binary has a Dockerfile.
- `docker-compose.prod.yml` wires API, ingester, canary, janitor, PostGIS, and Caddy.
- `.env.example` includes all required production env vars.
- Caddy routes public HTTP/S to API and uses `ADMIN_EMAIL`.
- Services use `restart: always` and log rotation.

Files affected: `docker-compose.yml` renamed to `docker-compose.dev.yml`, new `deploy/docker/*.Dockerfile`, `docker-compose.prod.yml`, `deploy/Caddyfile`, `.env.example`, `Makefile`, docs.

Test plan: `go build ./...`; local `docker compose -f docker-compose.dev.yml config`; local `docker compose -f docker-compose.prod.yml config`; runtime compose checks if Docker is available.

Risk: Docker may not be available in the coding environment. Compose config validation is still useful; runtime proof is documented if local Docker remains unavailable.

Tasks:

- [ ] Rename dev compose file and update Makefile.
- [ ] Add per-binary Dockerfiles.
- [ ] Add prod compose, Caddyfile, and `.env.example`.
- [ ] Validate compose config.
- [ ] Run vet/test and commit `feat(W4): production compose and Dockerfiles`.

### W5. DigitalOcean provisioning

Scope: add `cloud-init.yml` for Ubuntu 24.04 DigitalOcean droplets. It installs Docker and the Compose plugin, clones the public repo URL placeholder, writes a production env skeleton if absent, brings up prod Compose, applies `db/schema.sql` and migrations, and opens firewall ports 80/443.

Acceptance criteria:

- `cloud-init.yml` is syntactically valid YAML.
- It defaults `PUBLIC_REPO_URL` to a placeholder owner must replace.
- It installs Docker from Docker's apt repository.
- It starts `docker compose -f docker-compose.prod.yml up -d`.
- It applies schema and migrations.
- It enables UFW for SSH, 80, and 443.

Files affected: `cloud-init.yml`, `docs/deployment.md`, `docs/architecture.md`.

Test plan: YAML parse test/script where available; `yamllint cloud-init.yml` if installed; otherwise run a lightweight parser fallback; `go vet ./...`; `go test ./...`.

Risk: cloud-init cannot be fully executed without a DO droplet. The artifact must be conservative and the runbook must include manual verification commands.

Tasks:

- [ ] Add cloud-init artifact.
- [ ] Add syntax validation notes.
- [ ] Update architecture deployment section.
- [ ] Run validation and commit `feat(W5): DigitalOcean cloud-init`.

### W6. Smoke test scripts

Scope: add `scripts/smoke.sh` and `scripts/smoke.ps1`. The scripts start `docker-compose.dev.yml`, wait for DB health, apply `db/schema.sql` and migrations, run ingester briefly, start API, curl health and active endpoints, verify meta fields, and assert the anonymous active endpoint is rate-limited on a second hit.

Acceptance criteria:

- Bash and PowerShell scripts exist and exit nonzero on failure.
- Scripts apply schema plus migrations.
- Scripts verify `/v1/health`.
- Scripts verify `/v1/incidents/active` returns `meta`.
- Scripts verify second anonymous active request returns `429`.

Files affected: `scripts/smoke.sh`, `scripts/smoke.ps1`, `README.md`.

Test plan: static shell inspection; run scripts where Docker is available; at minimum run shell syntax checks if available and PowerShell parse checks.

Risk: the ingester currently has no one-shot mode. v0.3 uses a 30-second bounded run with termination unless a clean one-shot is added separately.

Tasks:

- [ ] Add Bash smoke script.
- [ ] Add PowerShell smoke script.
- [ ] Add migration application helper in scripts.
- [ ] Run available syntax checks.
- [ ] Run vet/test and commit `feat(W6): smoke test scripts`.

### W7. Deployment runbook

Scope: add `docs/deployment.md` with the manual DO flow: droplet sizing, region, Ubuntu 24.04, cloud-init paste, DNS, `.env`, first paid key generation, smoke test against public URL, and cost estimate.

Acceptance criteria:

- Document is one to two pages.
- Fresh reader can execute without asking for missing commands.
- Includes droplet size `s-2vcpu-2gb`, region `nyc3` or `sfo3`, Ubuntu 24.04.
- Includes first paid key generation with `cmd/keygen`.
- Includes approximate $18/month cost.

Files affected: `docs/deployment.md`, `README.md`, `docs/v0.3-summary.md`.

Test plan: document self-review against acceptance checklist; `go vet ./...`; `go test ./...`.

Risk: DO UI details may change. Keep the runbook command-oriented and mention the UI fields the owner must supply.

Tasks:

- [ ] Add deployment runbook.
- [ ] Add v0.3 summary.
- [ ] Update README.
- [ ] Run vet/test and commit `docs(W7): deployment runbook`.

### W8. Cactus-facing meta block

Scope: extend active and manual refresh response metadata with `disclaimer`, `attribution`, `refresh_min_seconds`, and `tier`. Values are server controlled so iOS copy and refresh hints can change without an app release.

Acceptance criteria:

- `/v1/incidents/active` meta includes disclaimer, attribution, refresh hint, and tier.
- Free active responses set `refresh_min_seconds: 600`.
- Paid active responses set `refresh_min_seconds: 50`.
- Manual refresh responses set `refresh_min_seconds: 120`.
- Tier is `free` or `paid` based on resolved API key.

Files affected: `internal/api`, `internal/store`, tests, `README.md`.

Test plan: API handler tests for free, paid, and manual metadata; `go vet ./...`; `go test ./...`.

Risk: changing JSON shape can break early clients. This only adds fields to `meta`; existing fields remain.

Tasks:

- [ ] Extend meta DTO.
- [ ] Add active response tests for free/paid metadata.
- [ ] Add refresh response tests for manual metadata.
- [ ] Wire auth context tier into response meta.
- [ ] Run vet/test and commit `feat(W8): Cactus meta block`.

## Capabilities

### New Capabilities

- `v0-3-freemium-api`: API key auth, tier-aware rate limits, manual refresh, and Cactus metadata.
- `v0-3-deployment`: DigitalOcean deployment artifacts, Docker Compose, cloud-init, smoke tests, and runbook.

### Modified Capabilities

- `v0-2-mvp-api`: active/detail responses gain tier metadata and server-enforced cadence.
- `v0-2-mvp-observability`: smoke tests and deployment docs rely on health/canary freshness behavior.

## Impact

Affected code: `cmd/api`, new `cmd/keygen`, `internal/api`, `internal/store`, new `internal/auth`, new `internal/ratelimit`, `internal/config`, `go.mod`, migrations, Docker/Compose files, scripts, README, and docs.

Database impact: add migration `db/migrations/0002_api_keys.sql`; do not modify `db/schema.sql` in place.

Dependency impact: add `golang.org/x/time/rate` only.

## Decisions

1. OpenSpec change path: use `openspec/changes/v0.3-do-deploy/` exactly as requested, scaffolded by hand because OpenSpec 1.2.0 rejects dotted names for `new change` and `status`.
2. API key hashing: use SHA-256 over random 32-byte URL-safe keys; store only hex hashes.
3. Anonymous identity precedence: `X-Client-ID`, then `X-Forwarded-For`, then `RemoteAddr`.
4. Free keyed API keys still use the free 10-minute cadence.
5. Paid cadence is 50 seconds even though product copy says 60 seconds, matching the workstream's explicit requirement.
6. Manual refresh reads cache only and never calls the Phoenix upstream.
7. v0.3 runs one API process; Redis is deferred until multiple API replicas exist.
8. Docker production uses Caddy over Traefik because the Caddyfile is shorter and fits a manual DO runbook.

# phoenix-feed

Near real time public safety incident feed for Phoenix metro.

## Status

v0.3 — DigitalOcean deployable and ready for Cactus Alert freemium.

- Poll Phoenix Fire's public ArcGIS endpoint with `cmd/ingester`.
- Persist active incidents, unit history, lifecycle events, source polls, canary results, and raw-retention state in Postgres/PostGIS.
- Serve cached REST/JSON from `cmd/api`; users never call Phoenix directly.
- Enforce free/paid/manual refresh cadence server-side.
- Issue manual free/paid API keys with `cmd/keygen`.
- Deploy with Docker Compose, Caddy, and `cloud-init.yml` on DigitalOcean.

## Documents

- [`docs/architecture.md`](docs/architecture.md) — system design, components, data model, polling logic, API surface, deployment.
- [`docs/compliance-memo.md`](docs/compliance-memo.md) — legal posture, why we operate under the Phoenix Open Data Policy and not under ARS § 39-121.03, what changes for the paid tier.
- [`docs/commercial-purpose-letter-template.md`](docs/commercial-purpose-letter-template.md) — reusable § 39-121.03(A) statement template, kept on file for any future agency that requires formal records request handling.
- [`docs/v0.2-summary.md`](docs/v0.2-summary.md) — MVP change summary, deferred work, and autonomy decisions.
- [`docs/v0.3-summary.md`](docs/v0.3-summary.md) — deploy/freemium change summary.
- [`docs/deployment.md`](docs/deployment.md) — DigitalOcean deployment runbook.
- [`db/schema.sql`](db/schema.sql) — Postgres + PostGIS DDL.

## Local run

Start and initialize Postgres:

```bash
make db-up
make db-init
make db-migrate
```

Run the ingester in one terminal:

```bash
DATABASE_URL=postgres://phoenix:phoenix@localhost:5432/phoenix_feed?sslmode=disable go run ./cmd/ingester
```

Run the API in another terminal:

```bash
DATABASE_URL=postgres://phoenix:phoenix@localhost:5432/phoenix_feed?sslmode=disable go run ./cmd/api
```

Smoke the active endpoint:

```bash
curl http://localhost:8080/v1/incidents/active
```

Run the local smoke test:

```bash
scripts/smoke.sh
```

On Windows:

```powershell
.\scripts\smoke.ps1
```

Against an already-running deployment:

```bash
SMOKE_EXTERNAL=1 API_URL=https://alerts.example.com CLIENT_ID=owner-smoke scripts/smoke.sh
```

Open the web client directly from disk:

```text
web/index.html
```

The browser client calls `http://localhost:8080/v1/incidents/active`, so keep the API running.

## API routes

- `GET /v1/incidents/active` — active cached incidents with optional `bbox`, `lat`/`lon`/`radius_meters`, `since`, and `until` filters.
- `POST /v1/incidents/refresh` — manual cache refresh read, throttled to 120 seconds per client.
- `GET /v1/incidents/{source}/{incident_id}` — one incident with unit and event history.
- `GET /v1/incidents/history` — paid placeholder; returns `402` in v0.3.
- `GET /v1/health` — DB reachability, per-source freshness, canary status, and aggregate `ok`/`degraded`/`down`.

`/v1/incidents/active` and `/v1/incidents/refresh` include Cactus Alert metadata in `meta`: disclaimer, attribution, `refresh_min_seconds`, and `tier`.

## API keys

Generate a paid key against the configured database:

```bash
KEY_TIER=paid KEY_LABEL=cactus-alert-owner OWNER_EMAIL=owner@example.com \
DATABASE_URL=postgres://phoenix:phoenix@localhost:5432/phoenix_feed?sslmode=disable \
go run ./cmd/keygen
```

Send it as:

```bash
curl -H "X-API-Key: <api-key>" http://localhost:8080/v1/incidents/active
```

## Runtime configuration

- `DATABASE_URL` — Postgres DSN.
- `HTTP_ADDR` — API bind address, default `:8080`.
- `POLL_INTERVAL` — ingester cadence, default `60s`.
- `POLL_JITTER` — cadence jitter, default `10s`.
- `CLEAR_AFTER_MISSES` — successful absent polls before clearing, default `5`.
- `RAW_RETENTION` — raw JSONB retention, default `720h`.
- `PAID_TIER_ENABLED` — paid placeholder gate, default `false`.
- `LOG_LEVEL` — `debug`, `info`, `warn`, or `error`.
- `KEY_TIER` — `free` or `paid` for `cmd/keygen`, default `paid`.
- `KEY_LABEL` — label for generated API keys.
- `OWNER_EMAIL` — optional owner email stored on generated API keys.

## Verification

```bash
go vet ./...
go test ./...
go build ./...
docker compose -f docker-compose.dev.yml config
docker compose -f docker-compose.prod.yml --profile tools config
python -m yamllint cloud-init.yml
openspec validate v0.3-do-deploy
```

## Build the PDFs

```
python pdf-build/build.py                # compliance memo
python pdf-build/build_template.py       # commercial purpose letter
python pdf-build/build_architecture.py   # architecture doc (TODO)
```

PDFs land in `pdf-build/output/`.

## Toolchain

- **Backend**: Go 1.22+ (Go 1.26 works in the current dev environment)
- **Database**: Postgres 16 with PostGIS 3.4
- **Build**: Pandoc + Graphviz + Chrome headless for PDFs
- **Hosting (v0.3)**: DigitalOcean Ubuntu 24.04 droplet, Docker Compose, Caddy TLS

## Compliance summary

- Polls Phoenix Fire's ArcGIS REST endpoint at 60s + jitter under the Phoenix Open Data Policy.
- Users hit our cache only; never the city.
- Visible attribution; no Phoenix logos or seals; persistent "not for emergency use" banner.
- Aggregate liability cap and "as is" warranty in EULA.
- Scanner + Whisper as the secondary source (Mesa, Tucson, Phoenix fallback). Tagged separately, not treated as equivalent.

See `docs/compliance-memo.md` for the full reasoning.

## Repo layout

See `docs/architecture.md` section 11.

## License

TBD.

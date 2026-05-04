# phoenix-feed

Near real time public safety incident feed for Phoenix metro.

## Status

v0.2 MVP — end-to-end local feed loop.

- Poll Phoenix Fire's public ArcGIS endpoint with `cmd/ingester`.
- Persist active incidents, unit history, lifecycle events, source polls, canary results, and raw-retention state in Postgres/PostGIS.
- Serve cached REST/JSON from `cmd/api`; users never call Phoenix directly.
- Show a local static Leaflet client from `web/index.html`.
- Keep paid history as a `402 Payment Required` placeholder until auth, billing, and legal gates are ready.

## Documents

- [`docs/architecture.md`](docs/architecture.md) — system design, components, data model, polling logic, API surface, deployment.
- [`docs/compliance-memo.md`](docs/compliance-memo.md) — legal posture, why we operate under the Phoenix Open Data Policy and not under ARS § 39-121.03, what changes for the paid tier.
- [`docs/commercial-purpose-letter-template.md`](docs/commercial-purpose-letter-template.md) — reusable § 39-121.03(A) statement template, kept on file for any future agency that requires formal records request handling.
- [`docs/v0.2-summary.md`](docs/v0.2-summary.md) — MVP change summary, deferred work, and autonomy decisions.
- [`db/schema.sql`](db/schema.sql) — Postgres + PostGIS DDL.

## Local run

Start and initialize Postgres:

```bash
make db-up
make db-init
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

Open the web client directly from disk:

```text
web/index.html
```

The browser client calls `http://localhost:8080/v1/incidents/active`, so keep the API running.

## API routes

- `GET /v1/incidents/active` — active cached incidents with optional `bbox`, `lat`/`lon`/`radius_meters`, `since`, and `until` filters.
- `GET /v1/incidents/{source}/{incident_id}` — one incident with unit and event history.
- `GET /v1/incidents/history` — paid placeholder; returns `402` in v0.2.
- `GET /v1/health` — DB reachability, per-source freshness, canary status, and aggregate `ok`/`degraded`/`down`.

## Runtime configuration

- `DATABASE_URL` — Postgres DSN.
- `HTTP_ADDR` — API bind address, default `:8080`.
- `POLL_INTERVAL` — ingester cadence, default `60s`.
- `POLL_JITTER` — cadence jitter, default `10s`.
- `CLEAR_AFTER_MISSES` — successful absent polls before clearing, default `5`.
- `RAW_RETENTION` — raw JSONB retention, default `720h`.
- `PAID_TIER_ENABLED` — paid placeholder gate, default `false`.
- `LOG_LEVEL` — `debug`, `info`, `warn`, or `error`.

## Verification

```bash
go vet ./...
go test ./...
go build ./...
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
- **Hosting (phase 1)**: Dan's box via ngrok SSH tunnel
- **Hosting (phase 2)**: TBD, likely DO droplet with managed Postgres

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

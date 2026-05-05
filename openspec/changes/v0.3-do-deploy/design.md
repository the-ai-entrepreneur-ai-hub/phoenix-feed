## Overview

v0.3 keeps the existing Go/Postgres architecture and adds two layers around the read API: identity resolution and tier-aware cadence enforcement. Deployment remains deliberately simple: one DigitalOcean droplet, Docker Compose, Postgres/PostGIS in the same compose project, and Caddy terminating TLS.

## API Design

The API request path becomes:

1. CORS and timeout middleware.
2. API key/auth middleware.
3. Rate-limit middleware for active/detail/refresh routes.
4. Existing handlers.

Auth is intentionally light. Missing `X-API-Key` resolves to anonymous free. Present keys are SHA-256 hashed and looked up in `api_keys`; unknown or revoked keys return `401`.

Rate limits are server-enforced:

- Free active/detail: one request every 10 minutes.
- Paid active/detail: one request every 50 seconds.
- Manual refresh: one request every 120 seconds.
- Health: no rate limit.

## Database Design

Add migration `db/migrations/0002_api_keys.sql`:

- `id BIGSERIAL PRIMARY KEY`
- `key_hash TEXT UNIQUE NOT NULL`
- `tier TEXT NOT NULL CHECK (tier IN ('free','paid'))`
- `label TEXT NOT NULL`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `revoked_at TIMESTAMPTZ`
- `owner_email TEXT`

`db/schema.sql` is not modified in place. Smoke/deploy scripts apply base schema then migrations.

## Keygen Design

`cmd/keygen` reads `DATABASE_URL`, `KEY_TIER`, `KEY_LABEL`, and `OWNER_EMAIL`, generates a random API key, stores only its SHA-256 hash, and prints the plaintext once.

## Deployment Design

`docker-compose.dev.yml` replaces the current dev compose file. `docker-compose.prod.yml` includes:

- `db`
- `api`
- `ingester`
- `canary`
- `janitor`
- `caddy`

`cmd/keygen` is built as an image but run manually with `docker compose run --rm keygen`.

## Smoke Design

The smoke scripts run against local dev compose, not production infrastructure. They apply schema/migrations, run ingester briefly, start API, test health, test active response metadata, and verify rate limiting by hitting active twice as the same anonymous client.

## Deferred

Redis, multiple API replicas, managed Postgres, Terraform, CI/CD, billing, paid search, notifications, geofences, and app-store receipt validation remain deferred.

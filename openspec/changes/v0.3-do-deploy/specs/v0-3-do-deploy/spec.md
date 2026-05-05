## ADDED Requirements

### Requirement: API key auth

The system SHALL resolve request tier from optional `X-API-Key` and reject unknown or revoked keys.

#### Scenario: Anonymous request

- **GIVEN** no `X-API-Key` header
- **WHEN** a client requests a public incident endpoint
- **THEN** the request is treated as tier `free`

#### Scenario: Revoked key

- **GIVEN** an API key row has `revoked_at` set
- **WHEN** the key is used in `X-API-Key`
- **THEN** the response status is `401`

### Requirement: Per-tier rate limiting

The system SHALL enforce server-side free and paid cadence for incident reads.

#### Scenario: Free active burst

- **GIVEN** an anonymous free client has just requested `/v1/incidents/active`
- **WHEN** the same client requests it again within 10 minutes
- **THEN** the response status is `429`
- **AND** `Retry-After` is present

#### Scenario: Health bypass

- **GIVEN** a client has exhausted incident read rate limits
- **WHEN** the client requests `/v1/health`
- **THEN** the health response is not blocked by the incident read limiter

### Requirement: Manual refresh

The system SHALL expose `POST /v1/incidents/refresh` as a cache-read endpoint throttled to one request per 120 seconds per client.

#### Scenario: Manual refresh throttle

- **GIVEN** a client has just called `/v1/incidents/refresh`
- **WHEN** the same client calls it again within 120 seconds
- **THEN** the response status is `429`

### Requirement: Cactus metadata

The system SHALL include server-controlled Cactus metadata in active and refresh responses.

#### Scenario: Free active metadata

- **WHEN** a free client requests `/v1/incidents/active`
- **THEN** `meta.disclaimer` is `Not for emergency use; call 911`
- **AND** `meta.attribution` is `Data via City of Phoenix Fire Department`
- **AND** `meta.refresh_min_seconds` is `600`
- **AND** `meta.tier` is `free`

### Requirement: Deployable artifacts

The repository SHALL provide Docker Compose, cloud-init, and smoke-test artifacts sufficient for a manual DigitalOcean deployment.

#### Scenario: Production compose config

- **WHEN** the owner runs `docker compose -f docker-compose.prod.yml config`
- **THEN** the compose file defines API, ingester, canary, janitor, PostGIS, Caddy, and keygen services

#### Scenario: Deployment runbook

- **WHEN** a fresh reader opens `docs/deployment.md`
- **THEN** it includes droplet size, region, Ubuntu version, cloud-init, DNS, env values, keygen, smoke test, and cost estimate

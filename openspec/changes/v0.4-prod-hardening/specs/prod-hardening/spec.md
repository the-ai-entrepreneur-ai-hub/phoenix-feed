## ADDED Requirements

### Requirement: Domain-ready TLS edge

The production edge SHALL support IP-only HTTP, hostname ACME HTTPS, and internal staging HTTPS from environment configuration without changing the API contract.

#### Scenario: IP-only mode

- **GIVEN** `DOMAIN=:80`
- **WHEN** a client requests `http://64.23.147.218/v1/health`
- **THEN** Caddy serves the API over HTTP
- **AND** the response does not include `Strict-Transport-Security`

#### Scenario: Hostname mode

- **GIVEN** `DOMAIN` is a DNS hostname pointed at the droplet
- **WHEN** Caddy starts
- **THEN** it manages a public ACME certificate for the hostname
- **AND** HTTP requests are redirected to HTTPS
- **AND** HTTPS responses include HSTS

#### Scenario: Internal TLS mode

- **GIVEN** `DOMAIN=:443 tls internal`
- **WHEN** Caddy starts
- **THEN** it serves HTTPS with Caddy's internal issuer

### Requirement: API security headers

The edge SHALL add browser hardening and information-leak reduction headers to API responses.

#### Scenario: Health headers

- **WHEN** a client requests `/v1/health`
- **THEN** the response includes `X-Content-Type-Options: nosniff`
- **AND** `X-Frame-Options: DENY`
- **AND** `Referrer-Policy: strict-origin-when-cross-origin`
- **AND** `Permissions-Policy: geolocation=(), microphone=(), camera=(), payment=()`
- **AND** `Cross-Origin-Resource-Policy: same-site`
- **AND** `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'`
- **AND** `Server: cactus-watch`

### Requirement: Deny-by-default CORS

The API SHALL allow native no-Origin requests and only configured browser origins.

#### Scenario: Native request

- **GIVEN** a request has no `Origin` header
- **WHEN** it reaches the API
- **THEN** the request is served normally
- **AND** no wildcard CORS header is emitted

#### Scenario: Configured origin

- **GIVEN** `ALLOWED_ORIGINS` contains `https://cactuswatch.example`
- **WHEN** a request has `Origin: https://cactuswatch.example`
- **THEN** the response includes `Access-Control-Allow-Origin: https://cactuswatch.example`

#### Scenario: Disallowed preflight

- **GIVEN** `ALLOWED_ORIGINS` is empty
- **WHEN** a browser preflight includes an arbitrary `Origin`
- **THEN** the response status is `403`

### Requirement: Edge per-IP rate limiting

Caddy SHALL enforce a second rate-limit layer of 60 requests per minute per source IP.

#### Scenario: Burst blocked at edge

- **GIVEN** one source IP sends more than 60 requests within one minute
- **WHEN** the limit is exceeded
- **THEN** Caddy returns `429`
- **AND** `Retry-After` is present

### Requirement: TLS telemetry logging

The edge SHALL write structured access logs suitable for later abuse analysis.

#### Scenario: Access log file

- **WHEN** Caddy handles a request
- **THEN** it writes JSON access logs under `/var/log/caddy/access.log`
- **AND** HTTPS log entries include negotiated TLS version and cipher suite when available

### Requirement: Host access hardening

The droplet SHALL expose only the required public services and require SSH key authentication.

#### Scenario: SSH port migration

- **WHEN** host hardening is applied
- **THEN** SSH accepts the existing key on TCP 2200
- **AND** TCP 22 is closed externally
- **AND** password authentication is disabled

#### Scenario: Firewall state

- **WHEN** external ports are scanned
- **THEN** only TCP 80, 443, and 2200 are open

### Requirement: Postgres and backup hardening

Postgres SHALL remain internal to Docker Compose and have daily local logical backups.

#### Scenario: Database exposure

- **WHEN** `docker-compose.prod.yml` is inspected
- **THEN** the `db` service has no `ports:` mapping for 5432

#### Scenario: Daily dump

- **WHEN** the backup timer runs
- **THEN** a dump is written to `/var/backups/phoenix-feed/`
- **AND** dumps older than seven days are removed

### Requirement: Operational runbooks and baseline

The repository SHALL document monitoring, secret rotation, SSH operations, and observed security baseline probes.

#### Scenario: Runbook coverage

- **WHEN** v0.4 is complete
- **THEN** `docs/ops.md`, `docs/monitoring.md`, `docs/secret-rotation.md`, and `docs/security-baseline.md` exist

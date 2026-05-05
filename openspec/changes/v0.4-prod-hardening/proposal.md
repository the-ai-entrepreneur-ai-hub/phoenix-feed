## Why

phoenix-feed is live for Cactus Watch on a DigitalOcean droplet, but the edge and host are still in a launch posture: plain HTTP over IP, permissive CORS, root SSH on port 22, no external monitor, no local logical backups, and no persisted edge telemetry for later abuse analysis. v0.4 hardens the live deployment without changing the public API contract.

## Scope

In scope:

- Domain-ready Caddy TLS with safe IP-only fallback, internal staging TLS, strict TLS 1.2+/1.3 policy, and domain-only HSTS.
- Security headers on every proxied API response.
- Deny-by-default CORS driven by `ALLOWED_ORIGINS`.
- Caddy per-source-IP rate limiting at 60 requests per minute.
- Caddy JSON access logs at `/var/log/caddy/access.log` with TLS version and negotiated cipher suite fields.
- SSH hardening: port 2200, password auth disabled, fail2ban, and UFW limited to 80/443/2200.
- Compose resource limits and confirmation that Postgres remains internal-only.
- Daily logical Postgres backups retained for seven days.
- Secret rotation, monitoring, and security baseline runbooks.

Out of scope:

- Bot blocking based on JA3, Redis/distributed rate limiting, managed Postgres, Terraform, paid billing changes, API response shape changes, and domain purchase/DNS changes.

## Impact

Affected code and artifacts:

- Caddy deployment files under `deploy/`.
- `docker-compose.prod.yml`, `.env.example`, `cloud-init.yml`, `scripts/`.
- CORS config in `internal/config`, `cmd/api`, and `internal/api`.
- Runbooks under `docs/`.

Dependencies:

- Add fail2ban on the host.
- Build Caddy with `github.com/mholt/caddy-ratelimit@v0.1.0`.
- No JA3 plugin is added; see `design.md`.

Database impact:

- No schema change. JA3 is deferred to edge JSON logs because a JA3 Caddy plugin would add complexity and conflict with the dependency constraint.

## Non-goals

- Do not alter `/v1/health`, `/v1/incidents/active`, or `/v1/incidents/refresh` JSON shapes.
- Do not commit `.env`, `runtime.env`, API keys, or cloud-init local files.
- Do not enable HSTS for IP-only `DOMAIN` values that start with `:`.

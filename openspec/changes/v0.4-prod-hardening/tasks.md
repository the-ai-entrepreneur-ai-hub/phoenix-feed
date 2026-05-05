# v0.4 Production Hardening Tasks

## W1. Domain-ready TLS

- [x] Replace the Caddy deployment so `DOMAIN=:80` serves plain HTTP, a real hostname uses automatic ACME HTTPS, and `DOMAIN=:443 tls internal` uses Caddy internal TLS.
- [x] Enforce TLS 1.2 minimum, TLS 1.3 maximum, and TLS 1.2 ECDHE AEAD cipher suites.
- [x] Enable HSTS only when `DOMAIN` is a hostname, not when it starts with `:`.
- [x] Preserve Caddy's automatic HTTP-to-HTTPS redirect for hostnames.
- [x] Deploy Caddy and verify `/v1/health` returns 200.
- [x] Commit `feat(W1): add domain-ready Caddy TLS`.

## W2. Security headers

- [x] Add Caddy response headers: `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, `Permissions-Policy`, `Cross-Origin-Resource-Policy`, `Content-Security-Policy`, and `Server`.
- [x] Apply the headers to upstream responses and Caddy-generated error responses.
- [x] Deploy Caddy and verify `curl -I` shows the headers.
- [x] Commit `feat(W2): harden API response headers`.

## W3. CORS lockdown

- [x] Add `ALLOWED_ORIGINS` config parsing with empty default.
- [x] Change CORS middleware to allow no-Origin native requests, echo only configured origins, and reject disallowed preflights.
- [x] Add tests for no Origin, allowed Origin, denied Origin, and denied preflight.
- [ ] Deploy the API and verify `/v1/health` returns 200.
- [ ] Commit `feat(W3): lock down CORS origins`.

## W4. Caddy per-IP rate limit

- [ ] Build Caddy with `github.com/mholt/caddy-ratelimit@v0.1.0`.
- [ ] Configure one dynamic zone keyed by source IP at 60 requests per minute.
- [ ] Verify Caddy returns 429 with `Retry-After` under burst.
- [ ] Verify the existing anonymous API limiter still returns 429 on a second incident read within 10 minutes.
- [ ] Commit `feat(W4): add Caddy per-IP rate limiting`.

## W5. TLS fingerprint logging groundwork

- [ ] Configure JSON access logging to `/var/log/caddy/access.log`.
- [ ] Verify logs include negotiated TLS version and cipher suite when HTTPS is in use.
- [ ] Document why JA3 hashing is deferred until revenue justifies a dedicated plugin or proxy tier.
- [ ] Deploy Caddy and verify `/v1/health` returns 200.
- [ ] Commit `feat(W5): add Caddy TLS access logging`.

## W6. SSH hardening

- [ ] Update cloud-init for SSH port 2200, no password auth, fail2ban, and UFW 80/443/2200.
- [ ] Apply the same hardening to the live droplet without losing SSH access.
- [ ] Verify SSH works on 2200 and port 22 is closed.
- [ ] Document reconnect commands in `docs/ops.md`.
- [ ] Commit `chore(W6): harden SSH and firewall`.

## W7. Postgres and backup hardening

- [ ] Confirm Postgres has no external `ports:` mapping.
- [ ] Add compose CPU and memory limits for every production service.
- [ ] Add `scripts/pg_dump_to_local.sh` and systemd service/timer units for daily seven-day backups.
- [ ] Install and run the timer on the live droplet.
- [ ] Verify today's dump exists under `/var/backups/phoenix-feed/`.
- [ ] Commit `chore(W7): add resource limits and pg backups`.

## W8. Secret rotation playbook

- [ ] Add `docs/secret-rotation.md` covering Postgres password, API key, SSH host key, and Caddy ACME account key rotation.
- [ ] Include trigger conditions, exact commands, verification, and rollback for each procedure.
- [ ] Pull docs on the droplet and verify `/v1/health` returns 200.
- [ ] Commit `docs(W8): add secret rotation playbook`.

## W9. External monitoring runbook

- [ ] Add `docs/monitoring.md` with UptimeRobot free-tier keyword monitor config for `/v1/health`.
- [ ] Include Dan's `tradebridge2026@gmail.com` alert contact and manual setup steps.
- [ ] Pull docs on the droplet and verify `/v1/health` returns 200.
- [ ] Commit `docs(W9): add external monitoring runbook`.

## W10. Verification baseline

- [ ] Run external port and header probes.
- [ ] Add `docs/security-baseline.md` with observed command outputs and domain-dependent follow-ups.
- [ ] Verify compose health, Caddy headers, SSH port state, backup state, rate limits, and OpenSpec validation.
- [ ] Commit `docs(W10): record security baseline`.

## Closeout

- [ ] `go vet ./...`
- [ ] `go test ./...`
- [ ] `go build ./...`
- [ ] `docker compose -f docker-compose.prod.yml config`
- [ ] `openspec validate v0.4-prod-hardening --strict`
- [ ] Secret grep check reports no actionable secrets.

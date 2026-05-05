## Overview

v0.4 keeps phoenix-feed on a single 2 GB DigitalOcean droplet and hardens the layers that can be improved without a domain or managed infrastructure: Caddy, Docker Compose, host SSH/firewall policy, local backups, and runbooks.

## Caddy Mode Selection

Caddyfile conditionals are intentionally limited, so the deployment uses a small entrypoint script to render the final Caddyfile from `DOMAIN`:

- `DOMAIN=:80` renders an HTTP-only site for IP testing.
- `DOMAIN=feed.example.com` renders a hostname site with automatic ACME certificates and Caddy's default HTTP-to-HTTPS redirect behavior.
- `DOMAIN=:443 tls internal` renders a local/staging HTTPS listener using Caddy's internal CA.

This avoids separate Caddyfiles for staging and production while keeping HSTS out of IP-only mode.

## TLS Policy

The rendered Caddyfile sets `protocols tls1.2 tls1.3` and constrains TLS 1.2 to ECDHE AEAD suites only:

- `TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384`
- `TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384`
- `TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256`
- `TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256`
- `TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256`
- `TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256`

Caddy/Go do not allow custom TLS 1.3 cipher ordering; Go's TLS 1.3 implementation already offers only the modern AEAD suites requested. This is documented inline in the generated Caddyfile comments.

## CORS Boundary

CORS is enforced in the Go API because the allowed origin list is application-specific and already covered by Go tests. Missing `Origin` is allowed for native mobile callers. Configured origins are exact-match only from `ALLOWED_ORIGINS`. Disallowed browser preflights return `403`; disallowed simple requests receive no CORS allow header.

## Edge Rate Limit

Caddy OSS does not ship an IP limiter, so the Caddy image is built with `github.com/mholt/caddy-ratelimit@v0.1.0`. It uses a sliding window keyed by `{http.request.remote.host}` at 60 events per minute. This layer is deliberately coarse and complements the existing per-key/per-client application limiter.

## JA3 Trade-off

A Caddy JA3 plugin exists, but using it today would add another third-party module, disable or complicate HTTP/3/session behavior, and conflict with the "no new dependencies beyond fail2ban and Caddy ratelimit" constraint. v0.4 therefore logs Caddy's built-in structured TLS fields (`request.tls.version`, `request.tls.cipher_suite`, ALPN, SNI when present) to `/var/log/caddy/access.log`. That preserves most near-term abuse triage value without adding an enforcement surface.

## Host Hardening

SSH moves to 2200 with key-only root login, password auth disabled, fail2ban enabled, and UFW allowing only 80, 443, and 2200. The live rollout opens 2200 and verifies a second SSH session before closing 22.

## Backups

Logical backups run daily through systemd timer into `/var/backups/phoenix-feed/` and retain seven days. This does not replace DigitalOcean snapshots; it adds application-level restore flexibility for data corruption or operator mistakes.

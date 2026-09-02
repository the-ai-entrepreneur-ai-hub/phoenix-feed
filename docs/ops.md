# phoenix-feed Operations

## SSH Access

Production is the DigitalOcean droplet `cactus-watch-prod-v2` at `64.23.209.182` (sfo3), shared with VistaScan. `feed.cactuswatch.com` and `cactuswatch.com` both resolve to it. SSH listens on the default port 22, key-only (`PermitRootLogin prohibit-password`, no password auth). The Mac ssh config has an alias for it:

```bash
ssh vistascan-droplet
# equivalent to: ssh -i ~/.ssh/vistascan_do root@64.23.209.182
```

The earlier `64.23.147.218:2200` host in this document was the retired `cactus-watch-prod` droplet; do not use it.

## Firewall

UFW allows only:

```text
22/tcp
80/tcp
443/tcp
```

Verify from the droplet:

```bash
ufw status verbose
ss -tlnp
```

## Fail2ban

The `sshd` jail bans for one hour after three failed attempts in ten minutes.

```bash
fail2ban-client status sshd
journalctl -u fail2ban --since "1 hour ago"
```

## Caddy Logs

Caddy writes structured JSON access logs on the host:

```bash
tail -f /var/log/caddy/access.log
```

Current IP-only HTTP mode has no TLS fields. After `DOMAIN` is set to a hostname and DNS points at the droplet, HTTPS requests will include Caddy's built-in TLS request metadata such as negotiated TLS version and cipher suite. JA3 hashing is intentionally deferred until there is enough revenue signal to justify another edge dependency.

## Deploy

```bash
cd /opt/phoenix-feed/app
git pull --ff-only
docker compose --env-file /opt/phoenix-feed/runtime.env -f docker-compose.prod.yml up -d --build
curl -fsS http://127.0.0.1/v1/health
```

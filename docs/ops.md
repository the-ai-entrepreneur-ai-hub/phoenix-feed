# phoenix-feed Operations

## SSH Access

Production SSH listens on TCP 2200 and accepts the existing root key.

```bash
ssh -i ~/.ssh/id_rsa -p 2200 root@64.23.147.218
```

Port 22 is intentionally closed. Password authentication is disabled, and root login is limited to key-based access with `PermitRootLogin prohibit-password`.

On Ubuntu 24.04, `ssh.socket` is active by default. When changing the SSH port, bind the socket on IPv4 explicitly before restarting it:

```ini
[Socket]
ListenStream=
ListenStream=0.0.0.0:2200
ListenStream=[::]:2200
```

Always test a new SSH session on 2200 before removing any previous listener or firewall allowance.

## Firewall

UFW should allow only:

```text
80/tcp
443/tcp
2200/tcp
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

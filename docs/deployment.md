# DigitalOcean Deployment

## 1. Create the Droplet

Create a DigitalOcean Droplet with:

- Size: `s-2vcpu-2gb`
- Region: `nyc3` or `sfo3`
- Image: Ubuntu 24.04 LTS
- Authentication: SSH key
- Reserved IP: enabled if you want stable DNS before rebuilds

In Advanced Options, paste `cloud-init.yml` as user data. Before launch, replace the placeholder repo URL inside `cloud-init.yml`:

```bash
PUBLIC_REPO_URL="${PUBLIC_REPO_URL:-https://example.com/owner/phoenix-feed.git}"
```

Use the public Git URL for this repo. Cloud-init installs Docker, clones the repo into `/opt/phoenix-feed/app`, applies `db/schema.sql` plus `db/migrations/*.sql`, opens SSH/80/443, and starts `docker-compose.prod.yml`.

## 2. Point DNS

Create an `A` record for the app domain:

```text
alerts.example.com -> DROPLET_PUBLIC_IP
```

Wait until DNS resolves before expecting Let's Encrypt to issue a public cert.

## 3. Set Production Env

SSH to the droplet:

```bash
ssh root@DROPLET_PUBLIC_IP
cd /opt/phoenix-feed/app
cp .env.example .env
nano .env
```

Set at minimum:

```bash
POSTGRES_USER=phoenix
POSTGRES_PASSWORD=<long-random-password>
POSTGRES_DB=phoenix_feed
DATABASE_URL=postgres://phoenix:<long-random-password>@db:5432/phoenix_feed?sslmode=disable
DOMAIN=alerts.example.com
ADMIN_EMAIL=owner@example.com
LOG_LEVEL=info
OWNER_EMAIL=owner@example.com
```

Restart after edits:

```bash
docker compose --env-file .env -f docker-compose.prod.yml up -d --build
set -a; . ./.env; set +a
COMPOSE_ENV_FILE=.env COMPOSE_FILE=docker-compose.prod.yml bash scripts/apply-sql-file.sh db/schema.sql
for f in db/migrations/*.sql; do COMPOSE_ENV_FILE=.env COMPOSE_FILE=docker-compose.prod.yml bash scripts/apply-sql-file.sh "$f"; done
```

## 4. Generate the First Paid Key

```bash
docker compose --env-file .env -f docker-compose.prod.yml --profile tools run --rm \
  -e KEY_TIER=paid \
  -e KEY_LABEL=cactus-alert-owner \
  -e OWNER_EMAIL="$OWNER_EMAIL" \
  keygen
```

Record the printed `api_key` immediately. It is not stored in plaintext.

## 5. Verify

From your laptop:

```bash
SMOKE_EXTERNAL=1 API_URL=https://alerts.example.com CLIENT_ID=owner-smoke scripts/smoke.sh
curl -i https://alerts.example.com/v1/health
curl -i -H "X-API-Key: <paid-key>" https://alerts.example.com/v1/incidents/active
```

On the droplet:

```bash
docker compose --env-file .env -f docker-compose.prod.yml ps
docker compose --env-file .env -f docker-compose.prod.yml logs --tail=100 api
```

## Cost

Expected baseline cost is about `$18/month`: roughly `$18/month` for `s-2vcpu-2gb`, plus a reserved IP if DigitalOcean bills it separately. This excludes domain registration, backups, and future managed Postgres.

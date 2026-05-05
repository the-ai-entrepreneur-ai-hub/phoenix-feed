# v0.3 DigitalOcean Deploy + Freemium Readiness Tasks

## W3. API key auth

- [x] Add `db/migrations/0002_api_keys.sql`.
- [x] Add auth tier and key hashing helpers.
- [x] Write failing auth middleware tests for anonymous, valid, invalid, and revoked keys.
- [x] Add store create/lookup API key methods.
- [x] Add `cmd/keygen`.
- [x] Update `docs/architecture.md` for `api_keys`.
- [x] Run `go vet ./...` and `go test ./...`.
- [x] Commit `feat(W3): api key auth and keygen`.

## W1. Per-tier rate limiting

- [x] Add `golang.org/x/time/rate`.
- [x] Add tier-aware limiter package.
- [x] Write unit tests for free, paid, retry-after, and health bypass behavior.
- [x] Write concurrent burst integration-style limiter test.
- [x] Add API middleware for active and detail routes.
- [x] Document Redis swap path in `docs/architecture.md`.
- [x] Run `go vet ./...` and `go test ./...`.
- [x] Commit `feat(W1): tier-aware rate limits`.

## W2. Manual refresh endpoint

- [x] Write failing tests for `POST /v1/incidents/refresh` response shape.
- [x] Write failing tests for 120-second refresh throttle.
- [x] Reuse active cache read path.
- [x] Wire route.
- [x] Run `go vet ./...` and `go test ./...`.
- [x] Commit `feat(W2): manual refresh endpoint`.

## W8. Cactus-facing meta block

- [x] Extend active/refresh meta response fields.
- [x] Write tests for free active metadata.
- [x] Write tests for paid active metadata.
- [x] Write tests for manual refresh metadata.
- [x] Wire resolved auth tier into meta.
- [x] Run `go vet ./...` and `go test ./...`.
- [x] Commit `feat(W8): Cactus meta block`.

## W6. Smoke test scripts

- [x] Rename `docker-compose.yml` to `docker-compose.dev.yml`.
- [x] Update Makefile references.
- [x] Add migration application command path.
- [x] Add `scripts/smoke.sh`.
- [x] Add `scripts/smoke.ps1`.
- [x] Run available script syntax checks.
- [x] Run `go vet ./...` and `go test ./...`.
- [x] Commit `feat(W6): smoke test scripts`.

## W4. Dockerfiles + production Compose

- [ ] Add per-binary Dockerfiles for `api`, `ingester`, `canary`, `janitor`, and `keygen`.
- [ ] Add `docker-compose.prod.yml`.
- [ ] Add `deploy/Caddyfile`.
- [ ] Add `.env.example`.
- [ ] Validate compose configs where Docker is available.
- [ ] Run `go vet ./...` and `go test ./...`.
- [ ] Commit `feat(W4): production compose and Dockerfiles`.

## W5. DigitalOcean provisioning

- [ ] Add `cloud-init.yml`.
- [ ] Add cloud-init syntax validation fallback.
- [ ] Update architecture deployment notes.
- [ ] Run available YAML validation.
- [ ] Run `go vet ./...` and `go test ./...`.
- [ ] Commit `feat(W5): DigitalOcean cloud-init`.

## W7. Deployment runbook

- [ ] Add `docs/deployment.md`.
- [ ] Add `docs/v0.3-summary.md`.
- [ ] Update `README.md` for v0.3 commands.
- [ ] Run `go vet ./...` and `go test ./...`.
- [ ] Commit `docs(W7): deployment runbook`.

## Closeout

- [ ] Run `go vet ./...`.
- [ ] Run `go test ./...`.
- [ ] Run `go test -cover ./...`.
- [ ] Run `go build ./...`.
- [ ] Run `docker compose -f docker-compose.dev.yml config`.
- [ ] Run `docker compose -f docker-compose.prod.yml config`.
- [ ] Run `scripts/smoke.sh`.
- [ ] Run `scripts/smoke.ps1`.
- [ ] Validate `cloud-init.yml`.
- [ ] Run `openspec validate v0.3-do-deploy`.

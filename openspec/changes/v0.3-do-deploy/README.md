# v0.3 DigitalOcean Deploy + Freemium Readiness

Make phoenix-feed deployable on DigitalOcean and ready to back Cactus Alert's freemium model.

This change adds:

- API key auth and manual paid key generation.
- Server-enforced free/paid/manual refresh cadence.
- Cactus-facing response metadata.
- Dev/prod Docker Compose and per-binary Dockerfiles.
- DigitalOcean cloud-init and deployment runbook.
- Bash and PowerShell smoke tests.

OpenSpec CLI note: `openspec new change v0.3-do-deploy` is rejected by OpenSpec 1.2.0 because change names may only contain lowercase letters, numbers, and hyphens. This directory is the equivalent OpenSpec change package at the owner-requested path.

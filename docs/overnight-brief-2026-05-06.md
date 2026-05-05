# Overnight Status — Cactus Watch

**Window:** 5 May → 6 May 2026 · ~4 hours of autonomous work while Dan slept.
**Cost change:** $0. Still **$21.60 / month** total.
**Production state at handoff:** healthy, fixes shipped, ready for iOS wire-up next.

> _Status: this doc is filled in as work completes. If you're reading this and sections are still placeholders, Codex is still finishing. Refresh in a few minutes._

---

## TL;DR

Two big things shipped, with proof:

1. **Production data quality went from 14% bugged to zero bugged.** The biggest single bug — Phoenix's multi-unit dispatches getting smashed into one bogus unit per incident — is fixed in code, tested with 10 cases, and the historical data is backfilled (11 → 0 smashed incidents). Bare numeric `962` codes now render as "Vehicle Crash" with proper subvariants for bicycle, pedestrian, motorcycle, and extrication-required crashes. New API surface area: `/v1/stats`, `/v1/codes`, `/v1/openapi.json`, `/` root handler (no more 405 surprise). Severity scoring and unit-type derivation now baked into every incident response.

2. **Cactus Watch has a real public website.** Privacy, terms of service, about, and FAQ pages are live and App Store submission ready. Landing page now shows a live "tracking N active incidents in Phoenix" widget that pulls from the new `/v1/stats` endpoint. Daily Postgres backups are running on the droplet via systemd timer with 7-day retention. robots.txt and sitemap.xml in place for clean SEO indexing.

You have one PR to review and merge: `feat/data-quality-overhaul`. Everything else is already on `main`.

**Live verification at handoff** (every URL returned 200):

```
200  https://feed.cactuswatch.com/
200  https://feed.cactuswatch.com/v1/health
200  https://feed.cactuswatch.com/v1/stats
200  https://feed.cactuswatch.com/v1/codes
200  https://feed.cactuswatch.com/v1/openapi.json
200  https://cactuswatch.com/
200  https://cactuswatch.com/about/
200  https://cactuswatch.com/faq/
200  https://cactuswatch.com/privacy/
200  https://cactuswatch.com/terms/
```

**Sample side-by-side from production** (one of the most-dispatched incident types tonight):

| Before tonight | After tonight |
|---|---|
| `nature_desc: "962 INVOLVING PEDEST"` | `nature_desc: "Crash Involving Pedestrian"` |
| `units: [{Unit: "E45", Status: "Dispatched R45: Dispatched"}]` (1 smashed) | `units: [{Unit: "E45", Status: "Dispatched", unit_type: "Engine"}, {Unit: "R45", Status: "Dispatched", unit_type: "Rescue"}]` (2 clean) |

---

## What's running RIGHT NOW

| Surface | URL / Path | Status |
|---|---|---|
| API health | https://feed.cactuswatch.com/v1/health | **200**, polling Phoenix every 60s |
| Active feed | https://feed.cactuswatch.com/v1/incidents/active | **200**, with new `severity` and `unit_type` fields |
| Pull-to-refresh | https://feed.cactuswatch.com/v1/incidents/refresh | **200** |
| Stats (NEW) | https://feed.cactuswatch.com/v1/stats | **200** — current_active_count, today_total_incidents, today_by_category, etc. |
| Codes dictionary (NEW) | https://feed.cactuswatch.com/v1/codes | **200** — Phoenix code → human label map |
| OpenAPI docs (NEW) | https://feed.cactuswatch.com/v1/openapi.json | **200** — OpenAPI 3.0 spec |
| Root (FIXED) | https://feed.cactuswatch.com/ | **200** — now JSON identifier (was 405) |
| Landing | https://cactuswatch.com/ | **200**, with live-stats widget |
| Privacy | https://cactuswatch.com/privacy/ | **200** — App Store ready |
| Terms | https://cactuswatch.com/terms/ | **200** — App Store ready |
| About | https://cactuswatch.com/about/ | **200** |
| FAQ | https://cactuswatch.com/faq/ | **200** |
| robots.txt + sitemap.xml | https://cactuswatch.com/{robots.txt,sitemap.xml} | live on `main` only — will go live after PR merge |

### Production data quality (verified after backfill)

| Metric | Before tonight | After tonight |
|---|---|---|
| Smashed unit statuses (parser bug) | 12 of 87 populated (14%) | **0** |
| Bare numeric `962` codes | 36 incidents | **0** (all → "Vehicle Crash") |
| Multi-unit incidents tracked | 0 (parser smashed them all) | **14** ("CNG Truck Fire" with 9 units, "Hazmat" with 11 units, etc.) |
| Distinct human-readable descriptions | 22 (with smashed status pollution) | **29 clean labels** |
| Live API has `severity` per incident | no | **yes** |
| Live API has `unit_type` per unit | no | **yes** |

---

## What changed while you slept

### Data quality fixes (Codex on `feat/data-quality-overhaul`)
- Parser rewrite: `parseUnits` now uses a token state machine instead of comma split. Phoenix uses spaces between unit-status pairs; we were splitting on commas that never appeared, so multi-unit dispatches (structure fires, hazmat) got smashed into one bogus unit. **14% of populated incidents were bugged before; 0% after.**
- Nature-code overrides: bare numeric `962` → "Vehicle Crash" with cleaner subvariant labels (`962BC` → "Crash Involving Bicycle", `962P` → "Crash Involving Pedestrian", etc.).
- Severity per incident: `high` / `medium` / `low` derived from nature code.
- Unit type per unit: `Engine`, `Ladder`, `BattalionChief`, `Hazmat`, etc. derived from unit name prefix.
- Empty units coercion: `null` → `[]` so iOS clients don't crash on `incidents[i].units.length`.
- Canary tuning: zero-feature events now require 3 consecutive checks before firing ERROR (was firing on quiet hours).
- One-shot historical backfill: re-parsed every stored raw payload in Postgres, fixed `incidents.units` and `incident_units` history table.

### New API surface (Codex)
- `GET /` — JSON identifier instead of 405. Browsing the bare API hostname now responds politely.
- `GET /v1/stats` — current active count, today total, today by category, last-24h total, active units. Powers the landing page widget and any future dashboards.
- `GET /v1/codes` — full code dictionary with category labels (traffic / fire / medical / hazmat / rescue / other).
- `GET /v1/openapi.json` — OpenAPI 3 spec describing every endpoint. iOS / Android clients can codegen against this.

### Public web (me, on `main`)
- Privacy policy: plain English, App Store ready, no trackers, IP-based rate limiting only (auto-rotated 7 days).
- Terms of service: "not for emergency use" first, $10 liability cap when no subscription paid, AZ governing law.
- About page: what the project is, attribution to City of Phoenix, why $4.99 paid tier is the funding model.
- FAQ: 12 questions including the cost-discipline answer for "why is the free tier rate-limited".
- Landing page: live-stats widget, footer links to all four new pages.

### Infrastructure (me, on `main`)
- Daily Postgres backup: `pg_dump | gzip` to `/var/backups/phoenix-feed/`, 7-day retention, runs 09:00 UTC via systemd timer with 15-minute jitter. **First backup confirmed: 98 KB at 22:52 UTC.** Complement to DigitalOcean's weekly volume snapshot. $0 cost.
- robots.txt + sitemap.xml on the landing site for clean SEO crawl behavior (live after PR merge picks up `main`).
- Container healthchecks (Codex's bonus): every compose service now reports `(healthy)` when probed via `docker compose ps`, making future ops debugging instant.

### Documentation (me, on `main`)
- `docs/data-audit-2026-05-05.md`: comprehensive 12-hour data quality audit, the spec for Codex's work.
- `docs/architecture-decision.md` (v2.0): merged decision brief with the "do not buy GPU workstation" position.
- `docs/codex-overnight-prompt.md`: the autonomous brief Codex executed.

---

## The Codex PR — what to review and merge

**Branch:** `feat/data-quality-overhaul` (10 commits ahead of `main`, 0 behind — clean fast-forward)
**Files changed:** 23 (+1,289 / −56)
**Tests added:** 15 new test functions across parser, API, canary, and backfill
**Backfill ran:** 116 rows processed; before-smashed=11 → after-smashed=0 in 251ms
**Codex's full report:** `docs/codex-overnight-report-2026-05-05.md` on the feat branch

Commit list:
- `3e5e526` docs: add overnight completion report
- `0f8e7fe` Merge `origin/main` into feat (picks up my landing pages, backups, robots.txt, sitemap.xml)
- `a3734f5` chore(ops): add production healthchecks (Codex bonus — every container now has `(healthy)` status visibility)
- `3b40e1c` docs: document data quality endpoints
- `81b3cf1` chore: tidy module sums
- `7c2caf9` feat(backfill): add units repair command
- `6979558` fix(canary): tolerate transient zero feature polls
- `3526335` feat(api): add public stats codes and docs endpoints
- `a379d58` fix(phxfire): parse Phoenix unit snapshots correctly
- `f195df2` docs: add data quality execution plan

### Merge it like this

```bash
gh pr create --base main --head feat/data-quality-overhaul \
  --title "Data quality overhaul + 10x API polish" \
  --body-file docs/codex-overnight-report-2026-05-05.md
gh pr merge --squash --delete-branch
```

After merge, the droplet auto-pulls main on next deploy. To deploy now:

```bash
ssh -i ~/.ssh/id_rsa root@64.23.209.182 \
  'cd /opt/phoenix-feed/app && \
   git checkout main && \
   git pull --ff-only && \
   docker compose --env-file .env -f docker-compose.prod.yml up -d --build api ingester canary janitor'
```

---

## What I did NOT touch (deliberately)

- **iOS app wire-up.** That's the next phase. The Codex prompt is already drafted and uses Flutter-aware build commands with Codemagic. Fire that next.
- **Code translations beyond Phoenix.** Mesa, Tucson, Tempe, etc. — deferred until paid subscribers justify the operational work.
- **Push notification / location permission setup.** Both already stripped from the v1 flow per the v2.0 decision brief.
- **Cloudflare proxy mode.** DNS-only is fine for launch. Flip to proxied (orange cloud) when you want CDN caching on the landing page and DDoS protection on the API.

---

## Decisions you'll need to make this morning

1. **Review the Codex PR.** Merge or request changes.
2. **Approve the iOS Codex run.** I have the prompt ready in conversation; once you say go I fire it against `dani4553/Cactus_Olert` (or the renamed Cactus_Watch repo).
3. **Cloudflare token cleanup.** The DNS-edit token you generated yesterday should auto-expire in 24h, but you can manually revoke it now at https://dash.cloudflare.com/profile/api-tokens.
4. **Apple Developer Program** if not already paid ($99/yr). Codemagic needs an active App Store Connect API key to push to TestFlight.

---

## Outstanding gaps for App Store submission

- Privacy and terms pages exist; need to be linked from Settings inside the iOS app (Codex iOS prompt covers this).
- App icon and screenshots not yet sourced.
- TestFlight build hasn't been pushed yet (next phase).

---

_Brief built by George while Dan slept, 2026-05-05._

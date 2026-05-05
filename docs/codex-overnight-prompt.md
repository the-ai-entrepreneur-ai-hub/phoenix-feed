# Codex Overnight Brief — Cactus Watch Data Quality + 10x Polish

You are a senior Go and SRE engineer running fully autonomously while the founder sleeps. You have 2 to 3 hours of execution budget. Every line you write must serve one goal: when the founder wakes up, the production backend at `https://feed.cactuswatch.com` produces dramatically better, more readable, more usable data than it does right now, with no operational cost increase and no new dependencies.

## CONSTRAINTS THAT ARE NOT NEGOTIABLE

- **Server cost ceiling: $22/mo total.** Currently $21.60/mo on the existing droplet. Do not add managed Postgres, DO Spaces, additional droplets, paid Cloudflare features, or any service that costs money. All work must use the existing droplet's resources.
- **No new third-party Go dependencies** beyond what `go.mod` already declares (chi, pgx, golang.org/x/time/rate, slog).
- **Do not commit secrets.** `cloud-init.local.yml`, `.env`, `runtime.env` stay local. `git diff --cached` before each commit.
- **Push only to `feat/data-quality-overhaul`**, never to `main`. Open a PR description in the final report. Founder merges manually.
- **Backwards compatibility on all v1 endpoints.** `/v1/health`, `/v1/incidents/active`, `/v1/incidents/refresh` must keep their current response shape. New fields can be added but no field renamed or removed.

## REFERENCE DATA YOU MUST READ FIRST

Before any code change, read these files in order:

1. `D:\serveless-apps-2026\abusedMindset\phoenix-feed\docs\data-audit-2026-05-05.md` — this is your spec. It documents every bug, every missing feature, and the recommended fix order. Treat it as authoritative.
2. `D:\serveless-apps-2026\abusedMindset\phoenix-feed\internal\source\phxfire\phxfire.go` — the parser with the unit-split bug.
3. `D:\serveless-apps-2026\abusedMindset\phoenix-feed\internal\store\store.go` — Postgres access layer.
4. `D:\serveless-apps-2026\abusedMindset\phoenix-feed\internal\api\` — current API handlers.
5. `D:\serveless-apps-2026\abusedMindset\phoenix-feed\db\schema.sql` and `db\migrations\*.sql` — current schema.

Production backend, verified live before you start:

- Droplet: 64.23.209.182 (sfo3)
- SSH: `ssh -i ~/.ssh/id_rsa root@64.23.209.182`
- App path on droplet: `/opt/phoenix-feed/app`
- Running compose: `docker compose --env-file .env -f docker-compose.prod.yml ps`
- Public health check: `curl https://feed.cactuswatch.com/v1/health`

## WORK TO DO (in order)

### Group A — Critical fixes (must ship)

**A1. Rewrite `parseUnits` with a token state machine.**

Phoenix's `Units` field uses **regular space** to separate unit-status pairs after HTML entity decoding. Inside each pair, `<unit_name>: <status>` (status can be multi-word like "On Scene" or "Leaving For Hospital").

Replace the comma-split implementation with this algorithm:

```go
// After html.UnescapeString, the input looks like:
//   "E41: On Scene HM41: On Scene"
// or
//   "BC2: Responding BC601: Command DR1: Responding E12: On Scene"
//
// Tokenize by whitespace, then walk: any token ending in ':' starts a new
// unit; everything between two such tokens is the previous unit's status.
```

Add a comprehensive table-driven test covering: single unit, multi-unit, empty input, weird spacing, the Unicode non-breaking hyphen seen in `M‑174`, and the smashed strings observed in production (use real samples from `incident_units` history).

**A2. Backfill historical data.**

Write a one-shot migration script `cmd/backfill_units/main.go` that:
- Connects to Postgres
- Iterates every row in `incidents` where `units` is non-null
- Re-parses `raw -> 'attributes' ->> 'Units'` using the fixed parser
- Updates `incidents.units` to the new array
- Wipes `incident_units` rows for incidents being re-parsed and re-inserts the correct unit history (single snapshot per unit since we can't recover transitions for already-stored polls — use first/last_observed_at = received_at)
- Reports: rows processed, rows updated, time elapsed

Run it on the production droplet via SSH after pushing the binary. Capture and save output to `docs/backfill-2026-05-05.log`.

**A3. Override bare numeric nature codes.**

Add a small map in `internal/source/phxfire/phxfire.go`:

```go
var natureDescOverrides = map[string]string{
    "962":   "Vehicle Crash",
    "962A":  "Vehicle Crash",
    "962BC": "Crash Involving Bicycle",
    "962P":  "Crash Involving Pedestrian",
    "962X":  "Crash Requiring Extrication",
    "962MC": "Crash Involving Motorcycle",
}
```

Apply in `ParseFeatures` only when:
- `nature_code` matches an override key, AND
- Phoenix's `nature_desc` is empty OR equals the bare code OR starts with the bare code (e.g., "962 INV BICYCLE" → cleaner "Crash Involving Bicycle")

Do not override when Phoenix has already provided a meaningful description that differs from the override.

**A4. Empty units coercion.**

In `parseUnits`, return `[]model.Unit{}` (non-nil empty slice) instead of `nil` when input is empty or fully whitespace. Verify the JSON serialization renders as `[]` not `null` end-to-end.

**A5. Root handler.**

Add a chi route at `GET /` that returns:

```json
{
  "name": "cactus-watch-feed",
  "version": "v1",
  "docs": "https://feed.cactuswatch.com/v1/openapi.json",
  "health": "https://feed.cactuswatch.com/v1/health"
}
```

Status 200, content-type `application/json`. Eliminates the 405 anyone hitting the bare domain currently sees.

### Group B — 10x features (ship as many as time allows)

**B1. `GET /v1/stats` — public stats endpoint.**

Returns:

```json
{
  "current_active_count": 12,
  "today_total_incidents": 103,
  "today_by_category": {
    "Vehicle Crash": 47,
    "Structure Fire": 3,
    "Hazardous Situation": 8,
    "Mountain Rescue": 1
  },
  "last_24h_total": 198,
  "active_units_now": 47,
  "data_age_seconds": 32,
  "tier": "free"
}
```

This becomes the data behind the "live stats" widget on the landing page (do not implement the widget; just expose the data).

**B2. `GET /v1/codes` — code dictionary.**

Returns the full mapping of `nature_code` → human label that the API uses, so any client can render labels consistently:

```json
{
  "version": "phx-fire-2026-05",
  "codes": [
    {"code": "962",   "label": "Vehicle Crash",            "category": "traffic"},
    {"code": "962BC", "label": "Crash Involving Bicycle",  "category": "traffic"},
    {"code": "STR",   "label": "Structure Fire",           "category": "fire"},
    ...
  ]
}
```

Categorize each code into one of: `traffic`, `fire`, `medical`, `hazmat`, `rescue`, `other`. This is small enrichment that makes the data dramatically more useful for filtering and color-coding in the iOS app.

**B3. Severity field per incident.**

Derive a `severity` string in the API response based on the nature_code:

```
high   — STR, HOUSE, WF, CRASH (aircraft), MTNRES, HAZ codes, GAS leaks
medium — 962X (extrication), GRASS, BRST, VEH, DEBRIS, FLOOD, TREE
low    — 962 (regular crash), 962A, LOCK, SNAKE, CKFOUT
unknown — anything not in the table
```

This lets the iOS app prioritize visual emphasis. Add to the `Incident` JSON shape (additive, no contract break).

**B4. Unit type derivation.**

Each Phoenix unit name encodes its type by prefix:

```
E   = Engine
L   = Ladder / Truck
BC  = Battalion Chief
BR  = Brush truck
HM  = Hazmat unit
HR  = Heavy Rescue
M   = Medic / Ambulance
S   = Squad / Paramedic
R   = Rescue
DR  = Drone
PI  = Public Information
LT  = Light truck
NDC, NEDC, SDC, WDC, EDC = Division Chief
```

In the parsed unit struct, add a `unit_type` field. Regex-match the prefix from the unit name. Unknown prefix → `unit_type: "other"`.

**B5. OpenAPI spec at `/v1/openapi.json`.**

Generate a hand-written or `kin-openapi`-style spec describing every endpoint, every query param, every response shape. Mount as a static handler. Useful for both client developers and as documentation. Keep it concise; do not over-spec.

**B6. Lifecycle stats fix.**

The contract canary (in `internal/canary/`) currently fires ERROR when `feature_count == 0` even though Phoenix legitimately has zero active incidents during quiet hours. Change the failure condition to: `feature_count == 0 for N consecutive checks` where N=3 (so we need 3 hours of zero before alerting). Single-zero events should log INFO, not ERROR.

### Group C — Polish (bonus, only if Group A + B finished)

**C1.** Add a section to `README.md` documenting the new endpoints, severity table, code dictionary.

**C2.** Update `architecture.md` to reflect the parser fix and new fields.

**C3.** Add `pdf-build/build_audit.py` that turns `docs/data-audit-2026-05-05.md` into a PDF using the existing pipeline.

## VERIFICATION GATES

Each gate must produce paste-able evidence in the final report. If a gate fails, retry up to 3 times then stop.

1. **Pre-flight.** `git status` clean. `curl https://feed.cactuswatch.com/v1/health` returns `state: ok`.
2. **Parser unit tests pass.** `go test ./internal/source/phxfire/...` exits 0 with at least 8 new test cases covering smashed inputs.
3. **Build succeeds.** `go build ./...` exits 0.
4. **Backfill ran clean.** Capture the output of `cmd/backfill_units` showing rows updated and the count of incidents that flipped from "smashed status" to multi-unit arrays.
5. **Live API improved.**
   - `curl https://feed.cactuswatch.com/v1/incidents/active | jq '.incidents[] | select(.nature_code == "962") | .nature_desc'` returns "Vehicle Crash" for every match (no bare 962s).
   - `curl https://feed.cactuswatch.com/v1/incidents/active | jq '.incidents[].units | length'` shows multi-unit counts where appropriate, no smashed status strings.
   - `curl https://feed.cactuswatch.com/` returns the JSON identifier, status 200.
6. **New endpoints respond.** `/v1/stats`, `/v1/codes`, `/v1/openapi.json` all return 200 with the documented shape.
7. **Container health.** `docker compose ps` shows all 6 containers healthy after redeploy.
8. **No new dependencies.** `go mod tidy && git diff go.mod go.sum` shows no additions.

## DEPLOYMENT MECHANICS

You have full SSH access (`ssh -i ~/.ssh/id_rsa root@64.23.209.182`). After pushing your branch, deploy by:

```bash
ssh -i ~/.ssh/id_rsa root@64.23.209.182 'cd /opt/phoenix-feed/app && \
  git fetch origin && \
  git checkout feat/data-quality-overhaul && \
  git pull --ff-only && \
  docker compose --env-file .env -f docker-compose.prod.yml up -d --build api ingester canary janitor && \
  sleep 15 && \
  docker compose --env-file .env -f docker-compose.prod.yml ps'
```

Then run the backfill:

```bash
ssh -i ~/.ssh/id_rsa root@64.23.209.182 'cd /opt/phoenix-feed/app && \
  docker compose --env-file .env -f docker-compose.prod.yml run --rm api /usr/local/bin/backfill_units'
```

Or build the binary into a separate container if cleaner.

## FINAL DELIVERABLE

A single markdown report at `docs/codex-overnight-report-2026-05-05.md` titled **"Cactus Watch Data Quality Overhaul — Complete"** with:

1. Branch name and complete commit list (hash + subject)
2. Files changed (added / modified / deleted, counts)
3. Tests added (count, what they cover)
4. **Backfill results**: rows scanned, rows updated, before/after stats showing the parser bug count dropping to zero
5. **Side-by-side proof of improvement**: 5 example incidents showing the v1 (broken) JSON vs v2 (fixed) JSON
6. New endpoints with sample responses
7. Anything you found in passing that's worth flagging but didn't fix
8. PR-ready summary text the founder can paste into GitHub when merging

If any gate failed, replace items 1-8 with: which gate, exact failure output, what you tried, what stopped you.

## BEGIN

Read the audit doc first. Then write a tasks.md plan. Then start with A1. Commit early and often. Stop when all of Group A and at least 4 of Group B are shipped, gates 1 through 7 pass, and the report is written.

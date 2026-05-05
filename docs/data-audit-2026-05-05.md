# Data Quality Audit — 2026-05-05

**Scope:** 12 hours of production data on `cactus-watch-prod-v2` (sfo3, IP 64.23.209.182).
**Method:** 15-section SQL probe of Postgres, plus byte-level inspection of the raw Phoenix JSON we store.
**Top finding:** the unit-status parser is broken on multi-unit dispatches — and that's exactly the data the most operationally important incidents (structure fires, hazmat) depend on.

## Headline findings

### 🚨 P0 BUG — Unit parser splits on commas, Phoenix uses spaces

Phoenix Fire's `Units` field, after HTML-entity decoding, looks like this:

```
BR701: Responding E701: Responding
E41: On Scene HM41: On Scene
BC3: Dispatched BC601: Dispatched C957N: Dispatched DR1: Dispatched E13: Dispatched E28: Dispatched HR144: Dispatched HT44: Dispatched NDC: Dispatched PI3: Dispatched S28: Dispatched
```

Each unit is `<unit>: <status>` and units are joined by **plain space** (with `&#160;` non-breaking space inside each pair).

The parser at `internal/source/phxfire/phxfire.go:parseUnits()` splits on **comma**. There are no commas. Result: the entire string becomes one giant unit/status pair where `unit = "BR701"` and `status = "Responding E701: Responding"`. Every additional unit ends up smashed into the first unit's status field.

**Blast radius:**

| Metric | Value |
|---|---|
| Incidents stored in 12h | 106 |
| With non-empty units | 87 |
| With single legitimate unit | 75 |
| **With smashed multi-unit (BUG)** | **12 (14% of populated)** |
| With null units (Phoenix sent empty) | 19 |
| Distinct unit "statuses" in `incident_units` history | 100+, of which ~85 are smash artifacts |

The parser bug also pollutes the `incident_units` history table — 376 rows in the lifecycle log, of which a large fraction have a `status` value that's actually a chain of multiple unit-status pairs. This means historical reconstruction of "who was on this fire and when" is garbage for any complex incident.

**Phoenix data sample (verified via byte-level inspection):**

```
F26200309  raw: BR701:&#160;Responding E701:&#160;Responding
F26200326  raw: E41:&#160;On&#160;Scene HM41:&#160;On&#160;Scene
F26200313  raw: E185:&#160;Responding R185:&#160;Responding
```

**Correct algorithm:**

```
After html.UnescapeString (NBSP becomes regular space):
  E41: On Scene HM41: On Scene

Tokenize by whitespace:
  ["E41:", "On", "Scene", "HM41:", "On", "Scene"]

State machine:
  - Token ending in ":" → start new unit, name = token[:-1]
  - Other tokens → append to current unit's status
  - On new unit → emit previous (unit, joined-status)

Result:
  [{Unit: "E41", Status: "On Scene"}, {Unit: "HM41", Status: "On Scene"}]
```

**Fix is one function rewrite plus a backfill of historical data by re-parsing the stored `raw` column.**

### 🟡 P1 — Bare numeric code "962" needs human label

Of 29 distinct `nature_code` values seen, only ONE is sent as a bare numeric: `962`, which Phoenix's CAD uses for vehicle crashes. 35 occurrences. Phoenix labels every other code (LOCK→LOCK OUT, GRASS→GRASS FIRE, MTNRES→MOUNTAIN RESCUE, etc.) but leaves bare 962 untranslated.

Subvariants ARE labeled: `962BC` ("962 INV BICYCLE"), `962P` ("962 INVOLVING PEDEST"), `962X` ("962 EXTRICATION"), `962MC` ("962 INVOLVING MOTORC"), `962A` (same as 962, also bare).

**Fix:** add a server-side override in the parser:

```
"962"  → "Vehicle Crash"
"962A" → "Vehicle Crash"
"962BC" → "Crash Involving Bicycle"      (clean up Phoenix's truncated label)
"962P"  → "Crash Involving Pedestrian"
"962X"  → "Crash Requiring Extrication"
"962MC" → "Crash Involving Motorcycle"
```

For the iOS app to ship readable labels day one.

### 🟡 P2 — Two codes have ambiguous descriptions

| code | sometimes | other times |
|---|---|---|
| GAS2-1 | NATURAL GAS LEAK | PROPANE LEAK |
| CKFOUT | CHECK FIRE OUT | CHECK FIRE REPORTED |

Phoenix's CAD reuses the same code for slightly different dispositions. We should accept both, not override — let Phoenix's own description through unchanged.

### 🟡 P3 — Null vs empty units inconsistency

19 of 106 incidents (~18%) have `units = NULL`. When the iOS app does `incident.units.length`, this throws. We should store `[]` instead of NULL when Phoenix sends empty/missing Units.

Likely cause: parser returns `nil` slice; Postgres stores NULL. Should return `[]model.Unit{}` (empty slice) → JSON `[]`.

### 🟡 P4 — `/` returns 405

Anyone hitting `https://feed.cactuswatch.com/` (no path) gets HTTP 405 because chi's default behavior on an unmounted route. Cosmetic but unprofessional. Two fixes possible: a tiny root handler returning a JSON identifier, or 301 redirect to https://cactuswatch.com.

## What's verified clean

| Dimension | Result |
|---|---|
| Polling success rate (12h) | 100% (no failures recorded) |
| Coordinate outliers | 0 (all within Phoenix metro lat 33.0–34.0, lon -113 to -111) |
| Timestamp anomalies (received_at > last_seen_at) | 0 |
| Future-dated incidents | 0 |
| Long-lived "active" incidents (clearing logic broken) | 0 — every incident either still genuinely active or correctly cleared |
| Raw JSON fields captured | 9 of 9 (we don't drop any field Phoenix sends) |
| Lifecycle event types working | 4: created (106), updated (528), cleared (100), reopened (1) — all firing correctly |
| Reopen logic working | 1 incident (F26200705 SNAKE REMOVAL) was cleared then reopened — correctly recorded |
| Channel field populated | 99.1% (1 incident missing, ignorable) |
| Symbol_code populated | 100% |

## Other observations worth noting

### Phoenix's data is richer than we currently expose

Looking at unit progressions in the smashed history rows, Phoenix tracks:
- Unit deployment lifecycle (Dispatched → Responding → On Scene → Staging → Command → Leaving For Hospital)
- Unit reassignment (a unit can change status mid-incident)
- Incident escalation (adding more units as situation develops)
- Command structure (one unit takes "Command" role)

Once the parser is fixed, all of this becomes real. The iOS app can show a "8 units on scene" badge, a timeline of unit arrivals, and a command-structure summary.

### Status vocabulary

Canonical statuses we observe (clean, frequent):

```
Dispatched              — unit assigned, not yet en route
Responding              — unit en route
On Scene                — unit arrived
Staging                 — unit holding back at safe distance (hazmat, structure fires)
Command                 — incident commander role
Leaving For Hospital    — transport in progress
```

Six clean values. Once the parser is fixed, this is the entire vocabulary the iOS app needs to render.

### Coverage gaps in our schema

Phoenix sends nothing else of value beyond the 9 fields we capture. No dispatch priority, no caller info (which is correct — privacy), no estimated time on scene. So we are not missing data; we just need to USE what we have correctly.

## Recommended fix order

1. **Fix `parseUnits()` to use a state machine instead of comma split.** Single-line algorithmic change.
2. **Migration: backfill `incidents.units` and `incident_units` table by re-parsing every stored `raw` JSONB.** One-time pass, ~106 rows, instant.
3. **Add `nature_desc_overrides` map in `phxfire.go` for bare 962 codes.** 6-entry table.
4. **Return `[]model.Unit{}` instead of `nil` when Phoenix sends empty Units.** One-line fix.
5. **Add root handler `GET /` to API.** Return small JSON identifier or 301 to landing.

These five fixes elevate the data quality from "70% useful" to "production grade" without adding any new dependencies, costs, or operational burden.

## Why this matters for launch

The iOS app, before this audit, would have shipped with feed rows like:

```
962 — S 32nd St / I-10 — E23: Responding
962 — 4800 N 20th St — BC1 (Dispatched BC3: Dispatched BC601: ...)
WORKING STR FIRE — 2400 N 7th Ave — null
```

After these fixes:

```
Vehicle Crash — S 32nd St / I-10 — E23 (Responding)
Vehicle Crash — 4800 N 20th St — 13 units dispatched, BC1 commanding
Working Structure Fire — 2400 N 7th Ave — Awaiting unit assignment
```

That's the difference between a confused civilian and a paying subscriber.

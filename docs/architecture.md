# Architecture

## 1. Overview

phoenix-feed is a near real time public safety incident feed app for the Phoenix metro area. The product surface is a free tier (live map and list) and a paid tier (notifications, geofences, history, dev API). The system has three properties that together drive every other design decision:

- **Users never call the upstream.** Every read goes through our cache. Phoenix's servers see at most one request per minute from us, and zero requests from end users.
- **Source switching is a first class concern.** The MapServer ingester is one source. Scanner plus Whisper transcription is another. The product API is the same in both cases.
- **Observed, not authoritative.** Everything we surface is a snapshot of what we polled, not the agency's source of truth. Timestamps, unit states, and lifecycle events are labeled as observations.

## 2. Data sources

### 2.1 Phoenix Fire MapServer (primary)

```
GET https://maps.phoenix.gov/phxfire/rest/services/Active_Incidents__Public/MapServer/0/query
    ?where=1=1&outFields=*&f=json&outSR=4326
```

Public, unauthenticated, no published rate limit. Returns ArcGIS feature JSON. Reprojection is server side via `outSR=4326`. Active count is typically 0 to 50 features. Confirmed schema is in `db/schema.sql`. The upstream service may also return cross jurisdictional automatic aid incidents (Mesa, Paradise Valley, Laveen) which we ingest as is and let downstream filtering handle.

### 2.2 Scanner plus Whisper (fallback and Mesa/Tucson primary)

SDR receivers tuned to Phoenix Fire dispatch frequencies, audio piped through Whisper for transcription, NLP for incident extraction and geocoding. This source is **not equivalent** to the MapServer — confidence is lower, latency is variable, and the data contract is different. Records from this source carry `source = "scanner-whisper"` and the API exposes that distinction so downstream clients can weight accordingly.

### 2.3 AZ511 (planned, separate product)

ADOT's developer API for traffic and accident data. Governed by its own developer agreement, not by ARS § 39-121.03. Drives the future VistaScan product, not phoenix-feed.

## 3. Components

### System architecture

The runtime is five small Go binaries plus Postgres:

- **`cmd/ingester`** — polls one upstream source on a 60s + jitter cadence, writes to Postgres, manages the incident lifecycle. One ingester process per source. Source identity is config not code.
- **`cmd/canary`** — hourly contract check against each upstream. Verifies field names, output spatial reference, geometry plausibility, feature count sanity. Writes to `contract_canary` and pages on drift.
- **`cmd/api`** — read only REST/JSON. Serves cached data, applies rate limits, enforces auth for paid endpoints. Includes staleness fields in every response.
- **`cmd/janitor`** — periodic retention job. Drops `raw` JSONB after 30 days, archives cleared incidents older than 30 days to the cold partition.
- **`cmd/keygen`** — manual owner-operated key generator. Prints a new API key once and stores only its SHA-256 hash.

A web client and a future mobile client consume the API. Neither ever talks to Phoenix.

## 4. Data model

Full DDL is in `db/schema.sql`. The five tables in plain English:

- **`source_polls`** — every HTTP poll attempt is a row. Tracks request URL, status code, latency, feature count, payload hash, parser version, success flag. Drives the clearing logic — only successful polls count.
- **`incidents`** — current state per `(source, incident_id)`. Composite primary key, not a global incident_id, because the same numeric ID could appear in different sources.
- **`incident_units`** — observed timeline of unit dispatch states. One row per `(unit, status)` interval, collapsing consecutive identical observations.
- **`incident_events`** — lifecycle log: created, updated, cleared, reopened. Powers the audit view and any "activity" feed in the UI.
- **`contract_canary`** — one row per scheduled schema check. Stores expected vs actual fields and a structured drift diff.
- **`api_keys`** — manually issued free/paid API keys. Stores SHA-256 hashes only, with `tier`, `label`, `owner_email`, and `revoked_at`.

Plus the `active_incidents` view, which joins each non cleared incident with the most recent successful poll's start time so every API response can include `source_last_success_at`.

## 5. Polling and lifecycle logic

### Cadence

Base 60 seconds with uniform jitter in `[-10s, +10s]` applied on every poll, not just on backoff. Reasons:

1. Phoenix's own dashboard appears to update on roughly minute boundaries; sub minute polling is fake freshness.
2. Jitter on the normal cadence prevents synchronized clock edges if the ingester is restarted in lockstep across deploys.

On 5xx, exponential backoff with cap at 5 minutes. On `429` (if ever observed) longer backoff and an alert. On `403`, alert immediately and pause — that signals an upstream policy change.

### Clearing rule

An incident is **cleared** only when:

1. It has been absent from `N = 5` consecutive successful polls, AND
2. Each of those 5 polls returned a sane feature set (status 200, parser succeeded, feature count not suspiciously zero given recent baseline).

Implementation: when a poll succeeds and an existing incident is not in the response, set `missing_since` to the poll's start time if currently NULL. After 5 consecutive successful misses, set `cleared_at = missing_since` and emit a `cleared` event. Never set `cleared_at` on a failed poll, a parse error, or a sudden drop to zero features when the rolling baseline is not zero.

### Reopen rule

If a `(source, incident_id)` reappears after `cleared_at` is set, clear the `cleared_at` value, reset `missing_since`, increment `reopen_count`, and emit a `reopened` event. Do not create a new incident row. A high `reopen_count` over time is a signal of upstream noise we should investigate.

### Updates

On every poll where an incident is observed, update `last_seen_at`, `last_seen_poll_id`, and the snapshot fields. Compare with the prior snapshot. If anything material changed (units added, units status changed, location text changed, nature code changed), append to `incident_units` (or extend the existing open interval) and write an `updated` event with the structured delta.

## 6. Source switching

Source identity is a string. The ingester binary reads its source name from config. The same process, different config, ingests Phoenix MapServer or scanner+Whisper. The composite primary key `(source, incident_id)` means the database has no opinion about which source is "right" — both can coexist.

API responses always include `source` per record. Paid notifications can be configured by source. End users see incidents from whichever sources we have running, with the source disclosed.

If Phoenix kills the MapServer permanently, we shift the user-facing posture to scanner+Whisper and accept the lower data quality. We do not pretend the data is equivalent.

## 7. Contract canary

The canary runs hourly per source and asserts:

1. Endpoint reachable; HTTP 200.
2. Response includes every field in the parser's expected list.
3. `outSR=4326` is honored (geometry sanity check: `-180 ≤ x ≤ 180`, `-90 ≤ y ≤ 90`).
4. `feature_count` is plausible relative to the rolling 7 day baseline (not 0 when baseline is non zero, not 10x baseline).
5. Date field parses as epoch milliseconds in a recent window.
6. Sample feature parses cleanly through the production parser.

Any failure writes a row with `passed=false` and the structured drift diff, plus emits an alert. We page on drift before users notice.

## 8. API

REST/JSON. All responses include a top level staleness block:

```json
{
  "meta": {
    "source_last_success_at": "2026-05-04T06:01:58Z",
    "data_age_seconds": 47,
    "parser_version": "phx-fire-2026-05"
  },
  "incidents": [ ... ]
}
```

Endpoints (v1):

- `GET /v1/incidents/active` — bbox or radius filter, optional category filter
- `GET /v1/incidents/{source}/{incident_id}` — single incident with full history
- `GET /v1/incidents/history` — paid only, time range plus filters
- `GET /v1/health` — overall data freshness and per source status
- Paid only: `POST /v1/geofences`, `GET /v1/notifications`, `POST /v1/webhooks`

Auth: `X-API-Key` is optional. Missing keys are treated as anonymous free clients and rate limited by `X-Client-ID`, `X-Forwarded-For`, or IP fallback. Present keys are SHA-256 hashed, looked up in `api_keys`, and resolved to `free` or `paid`; unknown or revoked keys return `401`. Paid keys are issued manually with `cmd/keygen` until billing exists. No client should ever pass through to Phoenix.

Rate limiting is enforced in `cmd/api` with in-memory token buckets in v0.3. Free active/detail reads are limited to one request per 10 minutes per resolved client; paid active/detail reads are limited to one request per 50 seconds per API key; manual refresh is limited to one request per 120 seconds. This assumes a single API process on the v0.3 droplet. When traffic requires multiple API replicas, replace the in-memory buckets with Redis-backed counters keyed by the same identity strings and keep the handler contract unchanged.

## 9. Compliance

Operating under the Phoenix Open Data Policy and Open Data Terms of Use. Not under ARS § 39-121.03; that statute governs formal records requests, not voluntarily published developer APIs. See `docs/compliance-memo.md` for the full reasoning.

UI requirements:

- Persistent banner: "not for emergency use; call 911"
- Visible attribution: "data via City of Phoenix Fire Department"
- No use of the Phoenix Fire logo, City of Phoenix seal, or any confusingly similar mark
- No claim of official endorsement or partnership
- EULA: "as is" warranty, aggregate liability cap, explicit waiver of liability for bodily harm or property damage from reliance on the data
- PII filter at the normalizer: drop free text fields containing names; do not increase location precision beyond what Phoenix publishes

## 10. Deployment and ops

### Hosting

Phase 1 (free tier MVP): Dan's box via existing ngrok SSH tunnel, same pattern as Cactus Alert. Postgres, ingester, canary, API all on the box.

Phase 2 (paid tier launch): migrate to a small DO droplet or AWS lightsail instance with a managed Postgres add-on. Decision deferred until traffic demands it.

### Observability

- Per source poll metrics: success rate, latency p50/p95, feature count, parse failures
- Per incident metrics: lifecycle events per minute, reopen rate
- Contract canary status: time since last pass per source
- API metrics: requests per endpoint, p50/p95 latency, rate limit hits

Alerts on: canary failures, sustained ingester errors, API 5xx rate above threshold, source last success older than 10 minutes.

### Backup

Daily Postgres dump, 14 day retention. Schema is small enough that full restore is tested monthly.

## 11. Repo layout

```
phoenix-feed/
├── README.md
├── docs/
│   ├── architecture.md                       (this file)
│   ├── compliance-memo.md
│   └── commercial-purpose-letter-template.md
├── db/
│   └── schema.sql
├── cmd/
│   ├── ingester/                             (Go: per source poll loop)
│   ├── canary/                               (Go: hourly contract check)
│   ├── api/                                  (Go: read REST/JSON)
│   ├── janitor/                              (Go: retention + archival)
│   └── keygen/                               (Go: manual API key creation)
├── internal/
│   ├── source/
│   │   ├── phxfire/                          (Phoenix Fire MapServer adapter)
│   │   └── scannerwhisper/                   (future: scanner adapter)
│   ├── store/                                (Postgres data access)
│   ├── lifecycle/                            (clearing + reopen logic)
│   └── parser/                               (versioned parsers)
├── pdf-build/
│   ├── build.py                              (compliance memo PDF)
│   ├── build_template.py                     (letter template PDF)
│   ├── build_architecture.py                 (this doc as PDF)
│   ├── style.css
│   └── diagrams/
├── go.mod
└── go.sum
```

## 12. Open decisions for Dan

1. **Hosting for phase 1**: confirm Dan's box via ngrok, or stand up a fresh droplet now to keep environments separated. My vote: Dan's box, same pattern as Cactus Alert, until traffic warrants a move.
2. **Domain**: needed before launch. Suggest `phxincident.app`, `azfeed.app`, or pick one Dan owns. Open to ideas.
3. **AZ511 dev key**: register in parallel this week. The terms attached to the key registration determine whether VistaScan can use ADOT data without filing under § 39-121.03.
4. **Branding**: name and logo not blocking the engineering work but blocking app store submission. Park for now, revisit week 3.

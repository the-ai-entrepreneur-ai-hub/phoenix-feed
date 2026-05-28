## Overview

The parser is a separate long-running Go process, `cmd/dispatch-parser`, modeled on the existing ingester lifecycle: environment config, JSON `slog`, SIGINT/SIGTERM shutdown, database pool, and repeated polling. It scans a small backlog window and processes rows one at a time inside transactions so multiple parser instances can safely share the same queue.

## Queue Processing

Each parser batch attempts up to 50 rows. For each row it opens a transaction and selects the oldest unparsed transcript using:

```sql
SELECT ...
FROM dispatch_transcripts
WHERE parsed_at IS NULL
ORDER BY received_at ASC
LIMIT 1
FOR UPDATE SKIP LOCKED
```

Processing one locked row per transaction preserves the requested per-row commit behavior while still making multiple parser instances safe. If a batch processes fewer than 50 rows, the worker sleeps five seconds; otherwise it immediately loops to drain backlog.

Gate failures and geocode failures both set `parsed_at = NOW()` and leave `parsed_incident_id = NULL`. Successful promotions insert the incident, insert unit rows, then set `parsed_incident_id` to the new secondary `incidents.id`, all in one transaction.

## Gate Criteria

The parser is deliberately conservative. A transcript is publishable only when all criteria pass:

- `verification_confidence >= 0.80`, using the table column so future indexes can support it.
- `display_text` contains a Phoenix dispatch channel marker matching `(?i)\bCDEC[\s-]?\d+\b`.
- `display_text` contains at least one fire/EMS unit matching `(?i)\b(engine|ladder|battalion|rescue|squad|truck|medic|chief|amr)[\s-]?\d+\b`.
- `display_text` contains a Phoenix-style address or intersection.

The CDEC marker avoids promoting generic hallucinations. The unit requirement filters out address-only or police-style chatter. The address requirement ensures geocoding has a concrete location. The confidence threshold prevents clean-looking but low-confidence hallucinations from reaching the public feed.

## Extraction

The parser extracts the first address occurrence and uses the words between the final CDEC marker before that address and the address start as nature. Repeated CDEC markers before the nature are ignored. Punctuation after the nature is stripped and the result is normalized to title case. Units are deduped in transcript order and stored with `unit_name` and `unit_type`; the incident JSON also includes the existing mapserver-compatible `Unit` and `Status` fields so `/v1/incidents/active` keeps the same output shape.

## Geocoding

`internal/geocode` wraps Mapbox Geocoding API. It appends `, Phoenix, AZ` to the extracted address and sends:

- `proximity=-112.0740,33.4484`
- `bbox=-112.5,33.0,-111.5,33.9`
- `country=us`
- `types=address,place`
- `limit=1`

The parser refuses to start without `MAPBOX_TOKEN`. This token is server-side only and must be provisioned separately from any client/mobile map token.

`geocode_cache` stores address results by exact extracted address. Successful cache entries do not expire because addresses do not move. Failed cache entries are reused for 24 hours, then retried. Every cache hit increments `hits`, including cached failures.

Mapbox calls are rate-limited to five requests per second with a process-local token bucket in `internal/ratelimit`.

## Incident Inserts

Promoted rows use:

- `source = 'sdr_audio'`
- `incident_id = 'sdr-<dispatch_transcripts.id>'`
- `nature_desc = extracted nature`
- `location_text = extracted address`
- `geom = ST_SetSRID(ST_MakePoint(lon, lat), 4326)`
- `incident_date = dispatch_transcripts.captured_at`
- `received_at` and `last_seen_at` from parser time

The worker also inserts a `source_polls` row for `sdr_audio` so the existing `incident_units` schema can satisfy its poll foreign keys without weakening constraints.

## Admin Health

The existing dispatch health endpoint is extended with parser fields:

- `parser_last_batch_at`
- `parser_rows_promoted_last_hour`
- `parser_rows_gate_failed_last_hour`
- `parser_rows_geocode_failed_last_hour`
- `parser_backlog_unparsed`

No new transcript status columns are added. Gate/geocode failure counters are inferred from existing fields: parsed rows with null `parsed_incident_id` are split by whether their stored transcript still satisfies the gate.

## Non-Goals

- No deduplication against mapserver incidents. Phase 3 can compare location/time/nature and define suppression rules with real data.
- No app changes or badges.
- No source-specific active feed logic.
- No deploy trigger.

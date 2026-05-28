# Data Quality and Observability Audit - 2026-05-28

Audit mode: read-only code review plus read-only production SQL/API reads. Production observations below were taken on 2026-05-28 between 13:02 and 13:09 UTC.

## Metrics

| Metric | Result |
|---|---:|
| False positive rate, strict bogus/tactical SDR promotions | 0/20 = 0% |
| Materially wrong promoted SDR rows, usually multi-dispatch merge | 6/20 = 30% |
| False negative rate, high-confidence radio transcripts | 7/51 = 13.7% |
| Geocode metro plausibility | 20/20 = 100% |
| Geocode address-specific accuracy estimate | 16/20 = 80%, with 4 Phoenix-centroid fallbacks |
| Duplicate inflation factor, last hour SDR incidents | 1.00x, 0 duplicate clusters among 20 recent SDR incidents |
| Active API mixed-source count at 2026-05-28T13:08Z | 22 total, 20 `sdr_audio`, 2 mapserver |

Top observability holes, ranked by severity:

1. `sdr_audio` incidents never clear server-side, so old radio rows remain in `/v1/incidents/active` without an alert.
2. Multi-dispatch transcripts can publish one incident with units/nature/location from different calls; no quality counter detects this.
3. Parser drops are persisted only as `parsed_at` plus null `parsed_incident_id`; there is no failure reason for later audit.
4. Dispatch health exists but is admin-only and not externally monitored or paged.
5. Mapbox/geocode failures are counted but do not change health status or page anyone.
6. `cactus_uploader` health is inferred only from transcript freshness and has no Windows-side heartbeat.
7. Mapserver "fresh success" can stay green even if payloads freeze; payload stagnation is not a health criterion.
8. `dispatch_transcripts` has no retention policy and is already the largest table.
9. `geocode_cache` has no size, precision, or centroid-fallback signal.
10. `ADMIN_TOKEN` auth failures/rotation/leak signals are not surfaced beyond logs.

## Finding 1 - SDR incidents never clear from the server active feed

Severity: P0 wrong data shipped to users

Evidence:

- Live SQL showed 20 `sdr_audio` incidents, all active, with `incident_date` from 2026-04-28 through 2026-04-30 and `last_seen_at` around 2026-05-28T12:24Z:

```text
metric|source|rows|active|min_incident_date|max_incident_date
incidents_by_source|sdr_audio|20|20|2026-04-28 21:40:06+00|2026-04-30 08:51:48+00

metric|active_sdr|oldest_incident_date|newest_incident_date|oldest_last_seen|newest_last_seen
active_sdr_age|20|2026-04-28 21:40:06+00|2026-04-30 08:51:48+00|2026-05-28 12:24:02.539954+00|2026-05-28 12:24:38.366818+00
```

- Live `/v1/incidents/active` at 2026-05-28T13:08Z returned 20 SDR rows with the oldest SDR `incident_date` equal to 2026-04-28T21:40:06Z and `max_sdr_seconds=2700`.
- The active view includes any row where `cleared_at IS NULL` in `db/schema.sql:155`.
- The SDR worker inserts incidents and sets `received_at`/`last_seen_at`, but it does not create any clearing lifecycle for SDR rows in `internal/dispatch/parser/worker.go:242`.
- The MapServer lifecycle has explicit missing/cleared logic in `internal/store/store.go:716` and `internal/store/store.go:729`; the SDR parser path bypasses that lifecycle.
- The mobile app client-side filter hides rows once `seconds_since_last_seen > 1800` in `/d/serveless-apps-2026/abuserOfcreativity/cactus_repo/lib/providers/incidents_provider.dart:376`, with the threshold defined at `/d/serveless-apps-2026/abuserOfcreativity/cactus_repo/lib/utils/app_constants.dart:33`. That protects the app after 30 minutes, but the public API still publishes stale active SDR rows.

Recommended fix:

```go
// Do not publish historical radio backfill into active incidents.
if now.Sub(row.CapturedAt) > 15*time.Minute {
    markTranscriptParsed(reason="stale_backfill", incidentID=nil)
    return
}

// For live SDR incidents, make clearing explicit.
clearedAt := row.CapturedAt.Add(90 * time.Minute)
INSERT incidents (..., last_seen_at, missing_since, cleared_at)
VALUES (..., now, clearedAt, clearedAt)
```

Also add an API-side active filter while SDR lifecycle is immature:

```sql
AND NOT (source = 'sdr_audio' AND incident_date < NOW() - INTERVAL '2 hours')
```

Effort: Small for an active-feed guard, medium for a proper SDR lifecycle.

## Finding 2 - Promoted SDR false positives are low, but 30% of promoted rows are materially wrong

Severity: P0 wrong data shipped to users

Evidence:

- Requested sample size was 50 promoted SDR incidents, but production only had 20 joined promoted SDR incidents at audit time. I classified all 20.
- Strict false positives were 0/20: none of the promoted rows were pure tactical chatter or a pure Whisper hallucination.
- Materially wrong rows were 6/20 because one transcript contained multiple dispatches and the parser merged units/nature/location into one incident.
- The parser extracts all units from the entire transcript in `internal/dispatch/parser/parser.go:65`, but it extracts the first address in `internal/dispatch/parser/parser.go:82` and the nature from text before that first address in `internal/dispatch/parser/parser.go:101`. That design is vulnerable when a clip contains multiple dispatches.
- The worker writes one incident per transcript using `incident_id := sdr-<transcript_id>` in `internal/dispatch/parser/worker.go:235`, so it cannot split repeated dispatches into separate incidents.

Promoted sample classification:

| Group | Count | Incident IDs |
|---|---:|---|
| Real dispatch, fields broadly publishable | 14 | sdr-2136, sdr-2170, sdr-2234, sdr-3442, sdr-3650, sdr-3658, sdr-4105, sdr-4119, sdr-4912, sdr-5811, sdr-6247, sdr-6825, sdr-6910, sdr-3567 |
| Real dispatch but materially wrong/mixed | 6 | sdr-2482, sdr-4244, sdr-5691, sdr-5952, sdr-6574, sdr-6735 |
| Tactical chatter wrongly promoted | 0 | none observed |
| Whisper hallucination wrongly promoted | 0 | none observed |

Examples:

- `sdr-2482` published `nature_desc="Fall 1364 E-suite Citrus Drive"` and `location_text="1940 East University Drive"`. The transcript starts with `Engine 414 CDEC 4 fall 1364...` and then repeats a separate `Engine 206... Seizure, 1940 East University Drive` call. This is one row combining two calls.
- `sdr-5952` published one incident at `2505 West Plata Avenue`, but the transcript also contains `Ladder 259... Seizure, 3473 East Crescent Way` and `Medic 2207... Stroke... 2505 West Plata Avenue`; the unit list includes units from more than one dispatch.
- `sdr-6735` published `Difficulty Breathing` at `447 East Broadway Avenue`, but the transcript also contains an earlier `motor vehicle accident with motorcycle` and another `Fall` dispatch. Units from those earlier calls were attached to the published row.

Recommended fix:

```go
segments := splitOnRepeatedDispatchMarkers(text) // channel/unit/nature/address windows
for _, segment := range segments {
    parsed, ok := ParseDispatchSegment(segment, confidence)
    if ok {
        fingerprint := normalized(nature, location, timeBucket(capturedAt, 5*time.Minute))
        upsertIncident("sdr-"+fingerprint, parsed)
    }
}
```

At minimum, reject clips with more than one plausible address or more than one dispatch marker before promotion:

```go
if countAddressCandidates(text) > 1 || countDispatchChannelMarkers(text) > 2 {
    markParsed(reason="multi_dispatch_clip_requires_review")
    return
}
```

Effort: Medium.

## Finding 3 - High-confidence false negatives are about 14%, mostly parser pattern variants

Severity: P1 silent data quality drift

Evidence:

- Production had 52 rows with `verification_confidence >= 0.85` and `parsed_incident_id IS NULL`. One was `smoke_test_001.wav`, not radio traffic, so the radio denominator is 51.
- I classified 7 clear real dispatch misses among those 51 radio rows: estimated false negative rate 13.7%.
- The parser requires confidence, channel, unit, address, and nature in `internal/dispatch/parser/parser.go:33`. Channel variants are limited to CDEC/Sea Deck-style names in `internal/dispatch/parser/parser.go:10`; address patterns are limited in `internal/dispatch/parser/parser.go:14`.

Clear misses to feed Phase 2.2:

| Transcript ID | Why it should publish | Parser gap |
|---:|---|---|
| 6402 | `Engine 207 K-Deck 7 Hill Person 2525 East Southern Avenue Unit 205` | `K-Deck`/`Cadeck` channel variant missing; likely `ill person` nature variant |
| 7199 | `Fire Channel B3. 211 natural gas leak. 555 West Iron Avenue. Unit 101` with multiple units | `Fire Channel B3` channel variant missing; code-style nature should map to gas leak |
| 10560 | `Fire channel A7. 962 with fire. Hardy Drive and Minton Drive` | `Fire channel` variant and intersection without directional prefixes |
| 8978 | `Engine 57... Fire channel A9. House fire. 8504 South 22nd...` | `Fire channel` variant; address suffix is transcribed poorly |
| 3730 | `A-M-R-2-0-7. Sea Deck 12. Fall. C-L-F. 154-35. East Cabern Drive` | Hyphenated unit digits and punctuated house number |
| 2831 | `Level of consciousness. 4862 East Main Street. Unit B91. Medic 2220. Seabex 5` | Dispatch order is nature/address/unit/channel instead of channel/nature/address |
| 5672 | `Sea deck 4. Animal issue. 4323 North Winchester Road. Engine 261` | Parser likely passed but geocode/promotion failed; still a real dispatch drop |

Correct rejections included blank high-confidence rows, unit-only rows such as `Engine 220`, tactical/command chatter, cancellations, and scene-termination chatter. The blank high-confidence rows are also a signal that transcription confidence is not sufficient by itself.

Recommended fix:

```go
channelPatterns := []regexp.Regexp{
    cdecPattern,
    regexp.MustCompile(`(?i)\bK[\s-]?Deck\s*\d+\b`),
    regexp.MustCompile(`(?i)\bCadeck\s*\d+\b`),
    regexp.MustCompile(`(?i)\bFire\s+Channel\s+[A-Z]?\d+\b`),
}

unitPattern := regexp.MustCompile(`(?i)\b(engine|ladder|battalion|rescue|squad|truck|medic|chief|amr)(?:[\s-]?)(\d+(?:[\s-]?\d+)*)\b`)

// Try more than one field order.
orders := []parseOrder{
    ChannelNatureAddressUnits,
    NatureAddressUnitsChannel,
    UnitsChannelNatureAddress,
}
```

Persist the parse failure reason:

```sql
ALTER TABLE dispatch_transcripts ADD COLUMN parse_failure_reason text;
```

Effort: Medium.

## Finding 4 - Geocodes are in the Phoenix metro, but 20% are centroid fallbacks

Severity: P1 silent data quality drift

Evidence:

```text
geocode_rows|successes|failures|success_in_phx_bbox|phoenix_centroid_successes
20|20|0|20|4
```

The 20 promoted SDR geocodes were all inside the broad Phoenix metro bounding box. No sampled coordinate landed in another state.

Four successful geocodes resolved to exactly `lat=33.44823, lon=-112.075098`, which is a Phoenix centroid-style fallback rather than an address-specific point:

```text
31 East Coronado Cave Court|-112.075098|33.44823
12428 North Desert Stage Drive|-112.075098|33.44823
1051 South Thompson Road|-112.075098|33.44823
West Cypress Street and South Gilbert Road|-112.075098|33.44823
```

The Mapbox request is Phoenix-biased with proximity, bbox, country, type, and `limit=1` in `internal/geocode/geocode.go:102`. The code accepts the first returned feature center without checking feature type, relevance, or whether the returned point is address-level in `internal/geocode/geocode.go:145`.

Recommended fix:

```go
result := mapbox.Geocode(address)
if !insidePhoenixMetro(result) {
    return ErrNoResult
}
if isPhoenixCentroid(result) || result.Relevance < 0.80 || result.Accuracy == "place" {
    return ErrLowPrecision
}
```

Store precision metadata:

```sql
ALTER TABLE geocode_cache
  ADD COLUMN provider_relevance numeric,
  ADD COLUMN provider_accuracy text,
  ADD COLUMN low_precision boolean not null default false;
```

Effort: Small to add centroid rejection, medium to store provider quality metadata.

## Finding 5 - Duplicate inflation was not present in the last-hour incident table sample

Severity: P2 future scaling concern

Evidence:

The requested query over `sdr_audio` incidents received in the last hour found no exact duplicate clusters by normalized `location_text`, normalized `nature_desc`, and `incident_date` within 5 minutes:

```text
duplicate_clusters|inflated_incidents|excess_rows|recent_sdr_incidents
0|0|0|20
```

Current duplicate inflation factor: `20 / 20 = 1.00x`.

This result is not proof the duplicate problem is solved. It mostly reflects the small promoted population and the parser's current tendency to merge multiple dispatches inside one transcript instead of creating three duplicate rows.

Recommended fix:

```sql
CREATE UNIQUE INDEX sdr_incident_dedupe_idx ON incidents (
  source,
  md5(lower(regexp_replace(location_text, '[^a-z0-9]+', ' ', 'g'))),
  md5(lower(regexp_replace(nature_desc, '[^a-z0-9]+', ' ', 'g'))),
  date_bin('5 minutes', incident_date, TIMESTAMPTZ '2000-01-01')
) WHERE source = 'sdr_audio';
```

Prefer an application-level fingerprint so fuzzy location normalization can evolve without rebuilding index semantics.

Effort: Medium.

## Finding 6 - No current mapserver/SDR collision rows were identifiable, but Phase 3 needs cross-source dedupe

Severity: P1 silent data quality drift

Evidence:

- A read-only join between current `sdr_audio` rows and historical `phoenix-fire-mapserver` rows using `incident_date +/- 10 minutes` and `ST_DWithin(..., 750m)` returned 0 rows.
- There were no mapserver incident rows in the SDR captured window of 2026-04-28 through 2026-04-30:

```text
incident_id|nature_code|nature_desc|location_text|incident_date|lat|lon
(0 rows)
```

- Contrary to the prompt assumption that mapserver is frozen, production `source_polls` did not show a week-long freeze during the audited window. It showed 1,414 successful mapserver polls in the last 24 hours and 928 distinct successful payload hashes:

```text
source|polls_last_day|success_last_day|last_success|distinct_payloads_success_last_day|min_feature_count|max_feature_count
phoenix-fire-mapserver|1430|1414|2026-05-28 13:07:06.793795+00|928|0|14
```

Recommended Phase 3 dedupe strategy:

```go
type IncidentFingerprint struct {
    TimeBucket5m time.Time
    GeoHash7     string
    NatureFamily string // crash, fire, medical, alarm, gas, etc.
}

// Prefer structured mapserver when sources collide.
if mapserver.Match(sdr, within=10*time.Minute, distance<=500m, sameNatureFamily=true) {
    keep mapserver row visible
    link sdr transcript as corroborating evidence
    suppress sdr from active feed
}
```

Keep the source rows for audit, but expose a deduped public view:

```sql
CREATE VIEW public_active_incidents AS
SELECT * FROM ranked_cross_source_incidents
WHERE rank = 1;
```

Effort: Medium to large, depending on how much fuzzy matching is needed.

## Finding 7 - Observability is mostly passive; key failures do not page anyone

Severity: P1 silent data quality drift

Evidence:

- Public health can mark source freshness down when last success is stale in `internal/api/health.go:63`, and it returns HTTP 503 when state is down in `internal/api/health.go:72`.
- Admin dispatch health exposes transcript freshness, parser batch time, promoted/gate/geocode counts, and backlog in `internal/api/admin_dispatch_transcripts.go:277`.
- Dispatch health status is based only on last transcript received age in `internal/api/admin_dispatch_transcripts.go:293`; parser/geocode failure rates do not affect status.
- Compose healthchecks for ingester/parser/canary/janitor only check that the process command line exists in `docker-compose.prod.yml:63`, `docker-compose.prod.yml:81`, `docker-compose.prod.yml:98`, and `docker-compose.prod.yml:116`.
- The uploader is installed as a Windows logon scheduled task per `scripts/cactus_uploader/README.md:47`; the server can only infer it is alive from new uploaded rows.
- The code comments claim canary "pages on drift" in `cmd/canary/main.go:1`, but the implementation only logs errors in `cmd/canary/main.go:58`.

Failure-mode recommendations:

| Failure mode | Current detectable signal | Gap | Cheapest shippable signal |
|---|---|---|---|
| Ingester stops polling | `/v1/health` goes down after source stale >10m | No external monitor in repo | UptimeRobot/DO monitor on `/v1/health`, page on non-200 |
| Dispatch parser crashes | Docker healthcheck process grep; admin `parser_last_batch_at` | Health status can stay ok | Monitor `/v1/admin/dispatch/health`, page if parser batch age >60s or backlog >100 |
| `geocode_cache` reaches 1M+ rows | Manual SQL only | No count in health | Add cache row count/size to admin health, warn at 500k, page at 1M |
| Mapbox rate limit | `parser_rows_geocode_failed_last_hour` count | Status remains ok; logs debug | Page if geocode failures >0 for 10m or failure ratio >5%; log provider status at warn |
| `ADMIN_TOKEN` leaked/rotated | 401/429 logs only | No auth-failure dashboard; logs lack token version | Log token hash prefix/client IP, alert on 401/429 spike, support two-token rotation window |
| `dispatch_transcripts` >1M | Manual SQL only | No retention or row-count signal | Add row count/table bytes to admin health and janitor retention |
| `cactus_uploader` stops | `last_received_age_seconds` stale after 600s | Admin-only, no page | Monitor dispatch health and add Windows heartbeat POST/file-age check |
| Mapserver upstream frozen for a week | Fresh poll success can stay green | No repeated-payload freeze alarm | Alert when distinct payload hashes or incident deltas stay flat for N hours while polls succeed |

Effort: Small for external monitors, medium for new health fields.

## Finding 8 - `dispatch_transcripts` has no retention policy and is already the largest table

Severity: P2 future scaling concern

Evidence:

```text
relname|total_size|n_live_tup
dispatch_transcripts|26 MB|11047
source_polls|17 MB|33411
incident_events|9944 kB|25520
incidents|6128 kB|3512
incident_units|5888 kB|24420
geocode_cache|32 kB|20
```

```text
transcript_rows|table_total_size|avg_raw_payload_bytes|p50_raw_payload_bytes|max_raw_payload_bytes
11029|26 MB|1397.6|1497.0|4237

rows_last_hour|projected_rows_per_day_at_last_hour_rate|hours_to_1m_at_last_hour_rate
3430|82320|291.5
```

At the observed 3,430 rows/hour, `dispatch_transcripts` reaches 1M rows in about 12.1 days. At the prompt's 5,000 rows/hour estimate, it reaches 1M rows in 8.3 days.

The only implemented retention path drops `incidents.raw`, not `dispatch_transcripts.raw_payload`, in `internal/store/retention.go:17`. The transcript migration creates `raw_payload JSONB NOT NULL` and no retention columns in `db/migrations/0003_dispatch_transcripts.sql:1`.

Geocode cache size is currently tiny:

```text
successful_unique_geocodes|geocode_cache_size|bytes_per_cache_row
20|32 kB|1638.4
```

At 20 unique successful geocodes/hour, 6 months is about 86,400 rows. At the current rough 1.6 KB/row, that is about 135 MB plus index overhead. That is reasonable, but only if bad transcript variants do not explode unique addresses.

Recommended fix:

```sql
-- Keep normalized audit fields longer than raw transcript JSON.
DELETE FROM dispatch_transcripts
WHERE received_at < NOW() - INTERVAL '14 days'
  AND parsed_incident_id IS NULL;

UPDATE dispatch_transcripts
SET raw_payload = '{}'::jsonb
WHERE received_at < NOW() - INTERVAL '48 hours'
  AND parsed_at IS NOT NULL;
```

Better:

```sql
ALTER TABLE dispatch_transcripts
  ADD COLUMN raw_payload_dropped_at timestamptz,
  ADD COLUMN parse_failure_reason text;
```

Effort: Small for a janitor sweep, medium for partitioning by day.

## Finding 9 - App rendering tolerates mixed-source rows, but SDR rows are hidden client-side after 30 minutes

Severity: P2 future scaling concern

Evidence:

- The backend active response shape for SDR rows includes `source`, `incident_id`, `nature_desc`, `units`, `location_text`, `lon`, `lat`, and `incident_date`. Live sample rows had those fields.
- The app model parses missing optional fields safely: `nature_code`, `channel`, and `symbol_code` are nullable/fallback fields in `/d/serveless-apps-2026/abuserOfcreativity/cactus_repo/lib/models/incident_model.dart:170`.
- The app parses `units` as a list and falls back to an empty list in `/d/serveless-apps-2026/abuserOfcreativity/cactus_repo/lib/models/incident_model.dart:284`.
- Free tiles render `natureDesc`, `locationText`, and dispatch time in `/d/serveless-apps-2026/abuserOfcreativity/cactus_repo/lib/widgets/incident_tile.dart:91`.
- Paid tiles render channel and unit groups only when present in `/d/serveless-apps-2026/abuserOfcreativity/cactus_repo/lib/widgets/incident_tile.dart:142`.
- Marker tap drill-in uses the in-memory `Incident` object, not `/v1/incidents/{source}/{incident_id}`, in `/d/serveless-apps-2026/abuserOfcreativity/cactus_repo/lib/widgets/esri_map.dart:237` and `/d/serveless-apps-2026/abuserOfcreativity/cactus_repo/lib/widgets/marker_sheet.dart:20`.
- All 20 SDR rows have unit history rows and 0 incident event rows:

```text
incident_id|unit_history_rows|event_rows
sdr-2136|1|0
...
sdr-6910|1|0
```

Because the app does not fetch backend `events`, empty `incident_events` should not crash the current tap flow. The bigger issue is visibility: the provider drops any incident where `seconds_since_last_seen > 1800`, so the live SDR rows returned by the API at 2026-05-28T13:08Z would be hidden in-app.

Recommended fix:

```dart
// Keep the client defensive, but do not rely on it for server correctness.
if (incident.source == 'sdr_audio' && incident.secondsSinceLastSeen > 1800) {
  return false;
}
```

Server-side should own the same rule so the API, admin tooling, and app agree.

Effort: Small.

## Finding 10 - Dispatch health reports volume, but not data quality drift

Severity: P1 silent data quality drift

Evidence:

Live admin dispatch health:

```json
{
  "last_received_age_seconds": 5,
  "rows_last_hour": 3452,
  "high_confidence_last_hour": 144,
  "parser_rows_promoted_last_hour": 20,
  "parser_rows_gate_failed_last_hour": 10757,
  "parser_rows_geocode_failed_last_hour": 5,
  "parser_backlog_unparsed": 0,
  "status": "ok"
}
```

This says the pipe is moving, but it does not say whether promoted rows are stale, duplicated, mixed, centroid-geocoded, or wrong. The status remains `ok` even with geocode failures because status is only derived from transcript freshness in `internal/api/admin_dispatch_transcripts.go:293`.

Recommended fix:

```json
{
  "quality": {
    "active_sdr_older_than_2h": 20,
    "geocode_low_precision_last_hour": 4,
    "multi_dispatch_rejected_last_hour": 0,
    "parse_failure_reasons_last_hour": {
      "missing_channel": 6,
      "missing_address": 1
    }
  },
  "status": "degraded"
}
```

Effort: Medium.

## Overview

The fix adds an incident-data freshness signal beside the existing source-poll freshness signal. `source_last_success_at` and `data_age_seconds` continue to answer "did our scraper successfully poll recently?" while `newest_incident_at` and `data_staleness_seconds` answer "how old is the newest incident in this response?"

## Diagnosis

Live production evidence showed `/v1/incidents/active` returning `meta.data_age_seconds = 45` while all returned `incidents[].incident_date` values were from `2026-05-24T06:08:49Z`, roughly 49 hours old. Docker logs showed the ingester polling successfully every ~50 seconds with `observed: 6, cleared: 0`, so our pipeline was healthy but Phoenix Fire's mapserver upstream was frozen on the same six records.

The bug is semantic: `data_age_seconds` is named like data freshness but currently measures seconds since the latest successful scrape. Changing that field would break existing clients and dashboard behavior, so the fix is additive.

## API Design

`store.StalenessMeta` gains nullable fields:

- `newest_incident_at`: `*time.Time`, JSON key `newest_incident_at`.
- `data_staleness_seconds`: `*int`, JSON key `data_staleness_seconds`.

The API layer computes these fields from the incident rows it is already returning. For active and manual refresh responses, this means the maximum `IncidentDate` across `result.Incidents` after filters have been applied. For detail responses, this means the returned incident's `IncidentDate`.

If an endpoint returns no incident rows, both fields remain `nil` and serialize as `null`.

## Time Semantics

All timestamps are normalized with `UTC()` before serialization. Go's JSON encoder keeps the existing RFC3339/RFC3339Nano style, including fractional seconds when present.

`data_staleness_seconds` is computed server-side as `int(now.UTC().Sub(newestIncidentAt).Seconds())` and clamped at zero to avoid negative drift if clocks or upstream timestamps move slightly ahead of API time.

## Non-Goals

- Do not change `source_last_success_at`.
- Do not change `data_age_seconds`.
- Do not touch ingester polling, scraper parsing, lifecycle clearing, routing, auth, or database schema.
- Do not change the `incidents[]` array shape or response envelope shape beyond adding two nullable `meta` fields.

## Verification

Focused handler tests cover:

- Three returned incidents where newest is `2026-05-26T00:00:00Z` and API `now` is `2026-05-26T01:00:00Z`.
- Zero returned incidents returning `null` freshness fields.
- Newest incident timestamp equal to API `now`, yielding staleness `0`.
- Existing source-poll freshness fields are still present and unchanged.
- Incident detail responses use the same meta freshness helper.
- OpenAPI documents both new fields.

Final validation runs `go test ./...` and `openspec validate data-staleness-meta --strict`.

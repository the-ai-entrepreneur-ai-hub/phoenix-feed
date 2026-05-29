## Why

Promoted SDR audio incidents currently bypass the mapserver missing/cleared lifecycle, so historical radio backlog rows with `cleared_at IS NULL` remain in `/v1/incidents/active` indefinitely. This ships stale April dispatches as current incidents and was flagged as the highest-priority cross-audit data correctness issue.

## What Changes

- Add a configurable dispatch transcript freshness gate using `DISPATCH_MAX_AGE` with a default of 2 hours.
- Mark stale transcripts parsed without promotion and count them as parser gate failures with the distinct reason `stale_capture`.
- Add a configurable SDR active window using `SDR_ACTIVE_WINDOW` with a default of 90 minutes.
- Add an SDR-only janitor sweep that sets `cleared_at` for active `sdr_audio` incidents whose `incident_date` is older than the active window.
- Add a forward-only, idempotent migration that immediately clears existing stale `sdr_audio` rows older than 2 hours.
- Preserve mapserver lifecycle behavior and the `/v1/incidents/active` response shape.

## Capabilities

### New Capabilities
- `sdr-incident-active-window`: Fresh SDR transcripts may be promoted, stale SDR backlog is consumed without promotion, and active SDR incidents are cleared by an SDR-only janitor window.

### Modified Capabilities
- None.

## Impact

- Affected command code: `cmd/dispatch-parser`, `cmd/janitor`.
- Affected packages: `internal/config`, `internal/dispatch/parser`, `internal/store`.
- Affected database: new migration `0006_clear_stale_sdr_audio_incidents.sql`.
- Affected tests: parser integration, store janitor/active-view integration, migration coverage, config defaults.
- No new dependencies, no deploy trigger, no mapserver lifecycle changes, no active response shape changes, and no app/cactus repository changes.

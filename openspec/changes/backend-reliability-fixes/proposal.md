## Why

The 2026-05-28 backend audit found three P1 reliability gaps that can make the service report healthy state while silently losing or corrupting backend progress. This change hardens poll success recording, SDR geocode retry behavior, and migration execution so transient failures do not become false freshness or half-applied deploy state.

## What Changes

- Record a source poll as successful only after the successful feed apply path has persisted all observed incident writes and lifecycle updates.
- Keep SDR transcripts unparsed after transient geocode failures, count retry attempts, and consume only permanent no-result failures or capped retry failures.
- Run schema and migration SQL through strict `psql` flags that stop on statement errors and wrap each file in a single transaction.
- Add regression coverage for each audited finding.

## Capabilities

### New Capabilities
- `backend-poll-apply-reliability`: successful poll freshness is published only after backend writes complete.
- `dispatch-geocode-retry`: dispatch parser distinguishes retryable geocode failures from permanent no-result failures.
- `strict-sql-migrations`: deployment and smoke migration commands hard-stop on SQL statement errors.

### Modified Capabilities
- None.

## Impact

- Affected Go code: `internal/lifecycle`, `internal/store`, `internal/geocode`, and `internal/dispatch/parser`.
- Affected database: forward-only nullable `dispatch_transcripts.geocode_attempts` migration.
- Affected scripts/docs: local smoke scripts, Makefile database targets, cloud-init bootstrap, and deployment runbook.
- No new runtime dependencies.

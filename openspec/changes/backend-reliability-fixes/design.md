## Overview

This change keeps the fixes scoped to the audited failure modes. It does not change SDR active-window semantics, app code, RevenueCat code, or deployment execution beyond local repository scripts and docs.

## Poll Apply Reliability

`lifecycle.Manager.Apply` currently inserts `source_polls.success = true` before incident writes. The new flow records successful upstream polls as pending failure rows first, then flips the poll to success only after all observed incidents, missing marks, sweep updates, and cleared-event writes complete without error. Failed upstream polls continue to be recorded immediately as failed rows.

This preserves the existing per-operation store boundaries while making staleness metadata truthful. A partial incident write failure returns an error, skips mark-missing/sweep work, and leaves the poll row excluded from `LatestSuccessAt` and active-feed freshness metadata.

## Dispatch Geocode Retry

Geocode outcomes are separated into permanent and retryable classes:

- Permanent: `ErrNoResult` and Mapbox HTTP 400.
- Retryable: Mapbox 429, 5xx, HTTP transport/timeout/context errors, cache lookup errors, and other non-permanent provider failures.

The parser increments nullable `dispatch_transcripts.geocode_attempts` for retryable failures and leaves `parsed_at` null until retry succeeds or the configured cap is reached. The default cap is five attempts. Permanent no-result failures remain consumed with `parsed_at` set and `parsed_incident_id` null. The geocode cache stores permanent negative results but does not negative-cache retryable provider errors.

## Strict SQL Migration Execution

Repository SQL application paths route through strict `psql` invocation:

```bash
psql -v ON_ERROR_STOP=1 --single-transaction
```

The helper is used by local smoke and Makefile migration targets. Production bootstrap and documentation use the same strict flags. A regression test runs the helper with a fake Docker/psql boundary and failing SQL, proving the helper returns non-zero when strict flags are present.

## Risks

- Poll apply still is not one large transaction, so already-committed incident upserts can remain if a later write fails. This change intentionally targets the P1 false-freshness and clearing hazards by excluding the poll from success metadata and skipping subsequent lifecycle clearing on the first error.
- A transient geocode error can temporarily block the oldest row; ordering moves attempted rows behind fresh unattempted rows and the retry cap bounds repeated failures.
- Each SQL file is single-transaction; data-only migrations must still be written idempotently.

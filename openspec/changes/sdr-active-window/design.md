## Context

`sdr_audio` incidents are created directly by the dispatch parser from `dispatch_transcripts.captured_at`, stored as `incidents.incident_date`. They are not observed on every mapserver poll, so the mapserver missing/cleared lifecycle cannot naturally clear them. The active view currently returns rows where `cleared_at IS NULL`; setting a future `cleared_at` at insert time would immediately remove fresh SDR rows from the active feed.

## Goals / Non-Goals

**Goals:**
- Prevent historical SDR backlog from being promoted as live active incidents.
- Bound the visible lifetime of promoted SDR incidents without changing the active endpoint response shape.
- Clear already-promoted stale SDR rows through an idempotent migration.
- Keep all behavior SDR-only.

**Non-Goals:**
- Do not change mapserver missing/cleared lifecycle behavior.
- Do not add `active_until` or alter the active view semantics.
- Do not deduplicate SDR incidents against mapserver incidents.
- Do not change app code or public API response fields.

## Decisions

1. Use janitor clearing instead of an `active_until` column.

   A future `cleared_at` cannot represent "active until" because the active view filters `cleared_at IS NULL`. Adding `active_until` would require schema and active-view logic changes, including source-specific time evaluation in the view. The janitor approach keeps the existing durable clearing model: fresh SDR rows remain active with `cleared_at NULL`, then the janitor sets `cleared_at` once `incident_date < now - SDR_ACTIVE_WINDOW`.

2. Gate stale transcripts before parse/geocode/promotion.

   The parser should consume stale backlog without calling Mapbox or inserting incidents. Stale rows are marked parsed with `parsed_incident_id NULL`, logged with reason `stale_capture`, and counted in `gate_fail_count` plus a separate stale counter.

3. Store clearing in the existing `cleared_at` column.

   This keeps `/v1/incidents/active`, stats, and detail views consistent because they already use the same incident state. The janitor update uses `incident_date`, because `incidents` does not have a `captured_at` column.

4. Keep migration cleanup fixed at the parser freshness default.

   The one-shot migration clears active `sdr_audio` rows older than 2 hours, matching the default `DISPATCH_MAX_AGE`. Runtime behavior remains configurable through `SDR_ACTIVE_WINDOW`.

## Risks / Trade-offs

- SDR incidents remain active until the next janitor SDR sweep after expiry. Mitigation: run the SDR sweep on janitor startup and on a short dedicated interval separate from the raw-retention sweep.
- Stale transcripts are not promoted even if they contain valid dispatches. Mitigation: this is intentional because the public active feed represents current incidents, not historical replay.
- Invalid duration environment values currently fall back to defaults. Mitigation: this change follows existing config behavior and does not widen scope into global env parsing policy.

## Migration Plan

1. Apply `0006_clear_stale_sdr_audio_incidents.sql`; it updates only active `sdr_audio` rows older than 2 hours and is idempotent because it only targets `cleared_at IS NULL`.
2. Deploy parser with `DISPATCH_MAX_AGE` defaulting to 2 hours.
3. Deploy janitor with `SDR_ACTIVE_WINDOW` defaulting to 90 minutes and the SDR-only sweep enabled.
4. Rollback is code-only for runtime behavior; rows already cleared by the forward migration remain cleared, which is the intended correction for stale active data.

## Open Questions

None.

## Why

Phase 2 SDR promotion is live and currently promoting roughly 16 scanner-derived incidents per hour, but live transcripts exposed two parser-quality gaps. Nature extraction can over-capture repeated dispatch fragments, and Whisper often renders the Phoenix `CDEC` channel marker as phonetic neighbors such as `Sea Deck`, `Seabex`, `FedEx`, or `CDC`.

## What Changes

- Broaden the CDEC marker gate from literal `CDEC` to the observed Whisper variants while keeping the existing confidence, unit, and address gates unchanged.
- Normalize any accepted marker variant to `CDEC <number>` when storing the parsed channel.
- Replace free-form nature extraction with a curated Phoenix dispatch nature dictionary and first-match sanitization, so repeated/reordered dispatch fragments produce short user-facing nature text.
- Add one-shot migration `0005_cleanup_bad_natures.sql` to clean existing `sdr_audio` rows where long comma-delimited nature text was already promoted.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `sdr-dispatch-incident-promotion`: tighten nature extraction, accept observed CDEC transcription variants, and clean legacy over-captured SDR natures.

## Impact

- Affected code: `internal/dispatch/parser`.
- Affected database: one-shot data cleanup migration for existing `incidents` rows from `source = 'sdr_audio'`.
- Affected tests: parser fixtures for CDEC variants and over-capture, plus a migration seed-db cleanup test.
- No changes to confidence threshold, recognized unit pattern, address pattern, active endpoint query, app code, or deployment behavior.

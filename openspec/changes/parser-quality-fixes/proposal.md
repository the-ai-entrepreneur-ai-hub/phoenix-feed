## Why

The 2026-05-28 data-quality audit found two parser-quality failures in live SDR promotion. Six of twenty promoted SDR rows mixed units, nature, and location from different back-to-back dispatches in one transcript. The same audit also found high-confidence real dispatches rejected because the gate missed observed CDEC variants, hyphenated AMR units, and Phoenix address forms such as split house numbers, punctuation before directionals, undirected intersections, and `Signal Butte and 824 over under`.

## What Changes

- Anchor SDR parsing to the first complete CDEC dispatch segment and extract units, nature, and address only from that segment.
- Stop scanning for addresses across a later distinct dispatch marker; later calls in the same transcript are ignored instead of merged.
- Broaden the precision-preserving parser forms needed by the audit: `K-Deck`/`Cadeck` CDEC variants, hyphenated `A-M-R-2-0-7` units, punctuated/split house numbers, `E-suite` as `East`, undirected street intersections, explicit highway/route forms, and the observed `Signal Butte and 824 over under` artifact.
- Add canonical nature aliases for audited real calls, including `Ill Person`/`Hill Person`, `Injured Person`, `Animal Issue`, and `Motor Vehicle Accident With Motorcycle`.
- Regression-lock eight live transcript samples and the audit multi-dispatch examples in parser tests.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `sdr-dispatch-incident-promotion`: improves parser precision for multi-dispatch transcripts and recall for audited real CDEC dispatch variants while keeping the confidence floor and CDEC gate.

## Impact

- Affected code: `internal/dispatch/parser`.
- Affected tests: parser table tests, parser real-world regression tests, and existing gate fixtures.
- No database migration, deployment, SSH, cactus app, or confidence-threshold change.

## Non-Goals

- Do not deploy.
- Do not touch `cactus_repo`.
- Do not accept Fire Channel-only transcripts as CDEC dispatches.
- Do not lower `verification_confidence >= 0.80`.

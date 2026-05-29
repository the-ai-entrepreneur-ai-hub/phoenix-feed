## Overview

This change keeps the SDR promotion gate strict and changes only parser interpretation after a high-confidence transcript has a CDEC-like channel marker. The parser still rejects transcripts below `0.80` confidence and still requires a CDEC marker, a recognized fire/EMS unit, a known or safe fallback nature, and a geocodable address-like location before promotion.

## Multi-Dispatch Segmenting

The parser now treats the first CDEC marker sequence as the anchor for the transcript. Adjacent repeated channel markers such as `CDEC 4, CDEC 4` are treated as dispatcher verification of the same call. A later distinct CDEC-like marker before the first address is not used to complete the earlier call, because that is the failure mode that produced mixed promoted rows.

For normal dispatch order, parsing uses this bounded window:

1. Start at the current unit group before the first CDEC marker, or at the channel marker if no unit precedes it.
2. End at the first recognized address after the channel marker.
3. Extend only far enough to include a trailing unit when the observed order is channel, nature, address, unit.

Units are extracted only from that bounded window. This prevents call A from inheriting call B's units when multiple calls are transcribed back-to-back.

## Recall Additions

The address and unit additions are intentionally narrow:

- `K-Deck` and `Cadeck` are accepted as CDEC transcription variants and normalized to `CDEC <number>`.
- Hyphenated AMR transcriptions such as `A-M-R-2-0-7` normalize to `AMR 207`.
- House numbers can be split or punctuated before a directional, e.g. `182 31 East ...`, `146, West ...`, and `154-35. East ...`.
- `E-suite` is normalized only in the directional position, producing `East`.
- Undirected street-suffix intersections such as `Hardy Drive and Minton Drive` are recognized.
- Explicit route/highway forms are recognized, along with the observed `Signal Butte and 824 over under` scanner artifact. The stored address removes the trailing `over under`.

Fire Channel-only transcripts stay rejected because they do not satisfy the CDEC gate requested for this parser path.

## Nature Aliases

Nature extraction still returns canonical short labels. Aliases are added only for observed real dispatch language:

- `Hill Person` maps to `Ill Person`.
- `Natural Gas Leak` maps to `Gas Leak`.
- `Injured Person`, `Animal Issue`, `Ill Person`, `Stroke`, `Motor Vehicle Accident`, and `Motor Vehicle Accident With Motorcycle` are canonical parser natures.

## Test Strategy

Parser tests cover:

- The eight exact live transcript samples from the task brief.
- Audit multi-dispatch examples proving units, nature, and address come from call A only.
- Audited false-negative parser variants that are safe under the CDEC gate.
- Fire Channel-only examples that remain rejected.
- Existing negative gate fixtures, preserving the measured zero false-positive baseline for those fixtures.

Full validation is `go test ./...`, `go vet ./...`, `go build ./...`, and `openspec validate parser-quality-fixes --strict`.

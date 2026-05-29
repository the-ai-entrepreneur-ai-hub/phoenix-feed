# parser-quality-fixes Tasks

- [x] Read the 2026-05-28 data-quality audit and current parser/gate fixtures.
- [x] Add failing parser regression tests for live samples, multi-dispatch mixing, and audited false-negative variants.
- [x] Bound parser extraction to the first complete CDEC dispatch segment.
- [x] Add precision-scoped address, CDEC variant, AMR unit, and nature alias support for audited real dispatches.
- [x] Keep Fire Channel-only non-CDEC examples rejected.
- [x] Run `go test ./...`.
- [x] Run `go vet ./...`.
- [x] Run `go build ./...`.
- [x] Run `openspec validate parser-quality-fixes --strict`.
- [x] Commit and push with `change-id: parser-quality-fixes`.

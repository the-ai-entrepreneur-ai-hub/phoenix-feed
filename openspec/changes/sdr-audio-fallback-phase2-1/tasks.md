# sdr-audio-fallback-phase2-1 Tasks

- [x] Add parser fixtures for `Sea Deck`, `Seabex`, `FedEx`, and `CDC` CDEC variants.
- [x] Add parser regression coverage proving repeated/reordered dispatch text does not over-capture `nature_desc`.
- [x] Add curated nature extraction list and normalize accepted CDEC variants to `CDEC <number>`.
- [x] Store the normalized parsed channel on promoted SDR incident rows.
- [x] Add migration `0005_cleanup_bad_natures.sql`.
- [x] Add SQL migration test for long comma-delimited `sdr_audio` nature cleanup.
- [x] Run `go test ./...`.
- [x] Run `go vet ./...`.
- [x] Run `go build ./...`.
- [x] Run `openspec validate sdr-audio-fallback-phase2-1 --strict`.
- [x] Commit and push with `change-id: sdr-audio-fallback-phase2-1`.

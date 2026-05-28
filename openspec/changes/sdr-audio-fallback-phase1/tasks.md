# sdr-audio-fallback-phase1 Tasks

- [x] Add a database migration for `dispatch_transcripts`.
- [x] Add admin POST endpoint for idempotent raw transcript ingestion.
- [x] Add admin GET endpoint for recent transcript visibility.
- [x] Add OpenAPI documentation for both endpoints.
- [x] Add Windows polling uploader script, batch file, and ops README.
- [x] Add backend Go tests for upload, duplicate, auth, malformed request, size limit, rate limit, recent list, and OpenAPI routing.
- [x] Add Python uploader tests for sqlite skip, Phoenix timezone conversion, retry, permanent failure, and duplicate handling.
- [x] Run `go test ./...`.
- [x] Run `go vet ./...`.
- [x] Run `go build ./...`.
- [x] Run `python -m pytest scripts/cactus_uploader/tests/`.
- [x] Run `openspec validate sdr-audio-fallback-phase1 --strict`.
- [x] Commit and push with `change-id: sdr-audio-fallback-phase1`.

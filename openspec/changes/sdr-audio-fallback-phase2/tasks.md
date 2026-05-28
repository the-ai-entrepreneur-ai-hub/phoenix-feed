# sdr-audio-fallback-phase2 Tasks

- [x] Add migration `0004_incidents_id_and_geocode_cache.sql`.
- [x] Add parser fixture tests with positive and negative SDR transcript examples.
- [x] Add `internal/dispatch/parser` gate, extraction, batch worker, and integration tests.
- [x] Add `internal/geocode` Mapbox client, cache wrapper, and fake-server/cache tests.
- [x] Add parser command process and production compose service.
- [x] Extend `/v1/admin/dispatch/health` with parser fields.
- [x] Verify active endpoint surfaces promoted `sdr_audio` incidents without source-specific active query changes.
- [x] Run `go test ./...`.
- [x] Run `go vet ./...`.
- [x] Run `go build ./...`.
- [x] Run `openspec validate sdr-audio-fallback-phase2 --strict`.
- [x] Commit and push with `change-id: sdr-audio-fallback-phase2`.

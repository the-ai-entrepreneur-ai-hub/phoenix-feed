# data-staleness-meta Tasks

- [x] Add failing API handler tests for newest returned incident time, zero incidents, and zero-second staleness.
- [x] Add failing incident detail meta coverage for the shared staleness envelope.
- [x] Add failing OpenAPI coverage for the new meta fields.
- [x] Extend the shared staleness metadata DTO with nullable `newest_incident_at` and `data_staleness_seconds`.
- [x] Compute incident freshness server-side from the incidents returned by active/refresh and detail endpoints.
- [x] Document the new fields in the OpenAPI document.
- [x] Run `go test ./...`.
- [x] Run `openspec validate data-staleness-meta --strict`.
- [x] Commit and push with `change-id: data-staleness-meta`.

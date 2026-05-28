# dispatch-pipeline-hardening Tasks

- [x] Audit dispatch admin handlers, store functions, migration/query shape, rate limiting, OpenAPI, and uploader against existing active API/poller patterns.
- [x] Add failing Go tests for stricter upload validation, UTC normalization, dispatch request logging, per-token admin limiter buckets, health endpoint states, OpenAPI route coverage, and store SQL shape.
- [x] Add failing Python tests for default log-path collision avoidance, collision warning, and missing `tzdata` startup error.
- [x] Harden upload validation while preserving the Phase 1 endpoint contract.
- [x] Add dispatch access logs with `request_id`, `status`, and `latency_ms`.
- [x] Make duplicate transcript insert race-safe in one SQL statement.
- [x] Align recent transcript query with the existing `received_at` index order.
- [x] Add `GET /v1/admin/dispatch/health` and OpenAPI documentation.
- [x] Fix uploader defaults, requirements, startup checks, and README install steps.
- [x] Run `go test ./...`.
- [x] Run `go vet ./...`.
- [x] Run `go build ./...`.
- [x] Run `python -m pytest scripts/cactus_uploader/tests/`.
- [x] Run `openspec validate dispatch-pipeline-hardening --strict`.
- [x] Capture one `EXPLAIN ANALYZE` proving recent transcript list uses `idx_dispatch_transcripts_received_at`.
- [ ] Commit and push with `change-id: dispatch-pipeline-hardening`.

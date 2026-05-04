# phoenix-feed OpenSpec Context

phoenix-feed is a Go/Postgres/PostGIS public-safety incident cache for Phoenix metro. The upstream source for v0.2 is the Phoenix Fire ArcGIS REST endpoint, polled by `cmd/ingester` and served from our backend by `cmd/api`.

Key constraints:

- Users never call Phoenix directly.
- Source identity is part of every incident key: `(source, incident_id)`.
- Incident timestamps, unit status changes, clears, and reopens are observed by our poller and are not authoritative agency dispatch times.
- The free MVP serves live active incidents, staleness metadata, a simple web client, and health/canary visibility.
- Paid history, notifications, geofences, auth, payment, scanner+Whisper, and source partnerships are out of scope for v0.2.

Engineering conventions:

- Go, `pgx/v5`, `chi/v5`, hand-written SQL in `internal/store`.
- No new backend dependencies unless the proposal explicitly justifies them.
- TDD for production behavior where practical.
- `go vet ./...`, `go test ./...`, and `go build ./...` must pass before completion.

## ADDED Requirements

### Requirement: Hardened admin transcript upload validation

The system SHALL reject unsafe dispatch transcript upload inputs at the admin API edge before attempting database insertion.

#### Scenario: Upload rejects unsafe filename

- **GIVEN** `ADMIN_TOKEN` is configured
- **AND** a request uses `Authorization: Bearer $ADMIN_TOKEN`
- **AND** `wav_filename` contains a path separator, `..`, unsupported characters, or does not end in lowercase `.wav`
- **WHEN** `POST /v1/admin/dispatch/transcript` is called
- **THEN** the backend responds with status `400`
- **AND** no transcript insert is attempted

#### Scenario: Upload rejects absurd capture timestamp

- **GIVEN** `ADMIN_TOKEN` is configured
- **AND** a request uses `Authorization: Bearer $ADMIN_TOKEN`
- **AND** `captured_at` is before `2024-01-01T00:00:00Z` or more than 30 minutes after API server time
- **WHEN** `POST /v1/admin/dispatch/transcript` is called
- **THEN** the backend responds with status `400`

#### Scenario: Upload normalizes explicit RFC3339 offsets

- **GIVEN** a valid upload uses `captured_at` with an explicit non-UTC RFC3339 offset
- **WHEN** the backend stores the transcript
- **THEN** `captured_at` is normalized to UTC before insertion

#### Scenario: Upload rejects oversized display text

- **GIVEN** a JSON body is within the 256 KiB request cap
- **AND** `display_text` is larger than 16 KiB
- **WHEN** `POST /v1/admin/dispatch/transcript` is called
- **THEN** the backend responds with status `400`

### Requirement: Dispatch admin request observability

The system SHALL log dispatch admin route request outcomes with enough structured fields to correlate manual curl sessions and production errors.

#### Scenario: Dispatch route emits request log

- **GIVEN** a dispatch admin request includes `X-Request-ID: req-123`
- **WHEN** any `/v1/admin/dispatch/*` route completes
- **THEN** the API emits a structured slog entry with `request_id=req-123`
- **AND** includes route path, method, HTTP status, and `latency_ms`

### Requirement: Race-safe transcript idempotency

The system SHALL return a transcript row id for duplicate `wav_filename` uploads without relying on a second database round trip.

#### Scenario: Duplicate upload uses one conflict statement

- **GIVEN** a transcript row already exists for `wav_filename`
- **WHEN** another upload for the same filename reaches the store
- **THEN** the insert statement resolves the conflict in PostgreSQL
- **AND** returns the existing row id with `duplicate=true`

### Requirement: Recent transcript query uses received-at index order

The system SHALL keep the recent transcript list query aligned with the existing `idx_dispatch_transcripts_received_at` index.

#### Scenario: Recent transcript query is newest-first by received_at

- **WHEN** `GET /v1/admin/dispatch/transcripts/recent` fetches rows
- **THEN** the store query orders by `received_at DESC`
- **AND** limits the result set to 50 rows

### Requirement: Dispatch transcript health endpoint

The system SHALL expose an admin-authenticated dispatch health endpoint for staleness alerting and volume checks.

#### Scenario: Health reports empty table as stale

- **GIVEN** no dispatch transcripts have been received
- **WHEN** `GET /v1/admin/dispatch/health` is called with a valid admin bearer
- **THEN** the response status code is `200`
- **AND** `status` is `stale`
- **AND** `last_received_at` and `last_received_age_seconds` are `null`
- **AND** rolling counts are zero

#### Scenario: Health reports fresh ingestion

- **GIVEN** the latest transcript `received_at` is less than or equal to 600 seconds old
- **WHEN** `GET /v1/admin/dispatch/health` is called with a valid admin bearer
- **THEN** `status` is `ok`
- **AND** the response includes `last_received_at`, `last_received_age_seconds`, `rows_last_hour`, `rows_last_24h`, `high_confidence_last_hour`, `low_confidence_last_hour`, and `review_recommended_last_hour`

#### Scenario: Health reports stale ingestion

- **GIVEN** the latest transcript `received_at` is more than 600 seconds old
- **WHEN** `GET /v1/admin/dispatch/health` is called with a valid admin bearer
- **THEN** `status` is `stale`

#### Scenario: Health rejects unauthenticated requests

- **GIVEN** the request has no valid admin bearer
- **WHEN** `GET /v1/admin/dispatch/health` is called
- **THEN** the backend responds with status `401`

### Requirement: Multi-token admin dispatch rate limiting

The system SHALL rate limit dispatch uploads by the presented admin bearer token while preserving the `ADMIN_TOKEN` environment variable.

#### Scenario: Admin tokens have independent upload buckets

- **GIVEN** `ADMIN_TOKEN` contains two comma-separated bearer tokens
- **AND** the first token has exhausted its admin dispatch bucket
- **WHEN** a request uses the second configured token
- **THEN** the second token can upload using its own bucket

### Requirement: Windows uploader startup hardening

The Windows SDR uploader SHALL detect known clean-install problems at startup and avoid stdout log-file collisions.

#### Scenario: Default in-script log avoids batch redirects

- **WHEN** `CACTUS_UPLOADER_LOG` is unset
- **THEN** the uploader writes its in-script status log to `C:\cactus\logs\uploader_inscript.log`
- **AND** does not use `C:\cactus\logs\uploader.stdout`

#### Scenario: Uploader warns about redirect collision

- **GIVEN** `CACTUS_UPLOADER_LOG` is configured to the same path as the batch stdout or stderr redirect
- **WHEN** the uploader starts
- **THEN** it writes a warning to stderr
- **AND** continues running

#### Scenario: Uploader reports missing tzdata clearly

- **GIVEN** Windows Python cannot load `zoneinfo.ZoneInfo("America/Phoenix")`
- **WHEN** the uploader starts
- **THEN** it exits with a configuration error
- **AND** stderr tells the operator to install `tzdata` using `python -m pip install -r requirements.txt`

### Requirement: OpenAPI documents dispatch health

The system SHALL include the dispatch health endpoint in the OpenAPI document.

#### Scenario: OpenAPI includes dispatch health path

- **WHEN** a client fetches `/v1/openapi.json`
- **THEN** the document contains `/v1/admin/dispatch/health`

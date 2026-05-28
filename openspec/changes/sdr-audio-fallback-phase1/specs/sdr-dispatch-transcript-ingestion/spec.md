## ADDED Requirements

### Requirement: Raw SDR dispatch transcript storage

The system SHALL store SDR transcript upload payloads in a durable `dispatch_transcripts` table without creating or modifying incident rows during Phase 1.

#### Scenario: Migration creates transcript storage

- **WHEN** database migrations are applied
- **THEN** a `dispatch_transcripts` table exists with unique `wav_filename`
- **AND** stores `captured_at`, `received_at`, transcript text/model/confidence fields, `raw_payload`, nullable parser bookkeeping fields, and monitoring indexes
- **AND** no `incidents` table schema is changed

#### Scenario: Raw payload is preserved for future parsing

- **GIVEN** a transcript JSON payload contains model metadata beyond Phase 1 promoted columns
- **WHEN** the backend accepts the upload
- **THEN** the complete JSON object is stored in `raw_payload`
- **AND** Phase 1 does not reject the payload because of unknown transcript fields

### Requirement: Admin transcript upload endpoint

The system SHALL expose an admin-authenticated endpoint that accepts one SDR transcript JSON payload at a time and is idempotent by `wav_filename`.

#### Scenario: Admin uploads transcript

- **GIVEN** `ADMIN_TOKEN` is configured
- **AND** the request has `Authorization: Bearer $ADMIN_TOKEN`
- **AND** the JSON body includes non-empty `wav_filename` and RFC3339 `captured_at`
- **WHEN** `POST /v1/admin/dispatch/transcript` is called
- **THEN** the backend stores the transcript
- **AND** responds with status `200` and `duplicate=false`

#### Scenario: Duplicate upload returns existing row

- **GIVEN** a transcript row already exists for `wav_filename`
- **WHEN** the same transcript is uploaded again
- **THEN** the backend responds with status `200`
- **AND** returns the existing row id
- **AND** returns `duplicate=true`

#### Scenario: Upload rejects unauthenticated requests

- **GIVEN** the request has no bearer token, the wrong bearer token, or `ADMIN_TOKEN` is unset
- **WHEN** `POST /v1/admin/dispatch/transcript` is called
- **THEN** the backend responds with status `401`

#### Scenario: Upload validates only required envelope fields

- **GIVEN** the request body is malformed JSON, missing `wav_filename`, has empty `wav_filename`, is missing `captured_at`, or has unparseable `captured_at`
- **WHEN** `POST /v1/admin/dispatch/transcript` is called
- **THEN** the backend responds with status `400`

#### Scenario: Upload body is too large

- **GIVEN** the request body is larger than 256 KiB
- **WHEN** `POST /v1/admin/dispatch/transcript` is called
- **THEN** the backend responds with status `413`

#### Scenario: Uploads are rate limited per admin token

- **GIVEN** more than 60 upload requests are made for the same admin token inside the limiter window
- **WHEN** the rate limit is exceeded
- **THEN** the backend responds with status `429`

### Requirement: Admin recent transcript visibility

The system SHALL expose an admin-authenticated endpoint that returns recent dispatch transcript summaries for ops verification.

#### Scenario: Admin lists recent transcripts

- **GIVEN** `ADMIN_TOKEN` is configured
- **AND** the request has `Authorization: Bearer $ADMIN_TOKEN`
- **WHEN** `GET /v1/admin/dispatch/transcripts/recent` is called
- **THEN** the backend returns the last 50 transcripts ordered by `received_at DESC`
- **AND** each item includes `id`, `wav_filename`, `captured_at`, `received_at`, `display_text`, `verification_confidence`, `review_recommended`, and `parsed_incident_id`

#### Scenario: Recent transcripts reject unauthenticated requests

- **GIVEN** the request has no bearer token or the wrong bearer token
- **WHEN** `GET /v1/admin/dispatch/transcripts/recent` is called
- **THEN** the backend responds with status `401`

### Requirement: Windows SDR uploader

The system SHALL include a Windows-compatible polling uploader that sends Dan's local transcript JSON files to phoenix-feed without deleting or modifying the recordings directory.

#### Scenario: Uploader sends a new transcript

- **GIVEN** `ADMIN_TOKEN` is configured on Dan's box
- **AND** a `*.transcript.json` file exists in `D:\cactus\recordings`
- **WHEN** the uploader polls the directory
- **THEN** it reads the JSON file
- **AND** adds top-level `wav_filename` and UTC `captured_at`
- **AND** posts it to `/v1/admin/dispatch/transcript` with the bearer token
- **AND** records successful upload state in local sqlite

#### Scenario: Uploader survives restarts

- **GIVEN** a transcript's `wav_filename` is already present in local sqlite
- **WHEN** the uploader scans the directory after restart
- **THEN** it skips that transcript without posting it again

#### Scenario: Uploader handles server outcomes

- **GIVEN** the server returns status `200` with `duplicate=true` or status `409`
- **WHEN** the uploader handles the response
- **THEN** it records the transcript locally so it is not retried

#### Scenario: Uploader retries transient failures

- **GIVEN** the server returns a 5xx status or the network request fails
- **WHEN** the uploader handles the response
- **THEN** it leaves the transcript unmarked so a later poll retries it

#### Scenario: Uploader records permanent request failures

- **GIVEN** the server returns a non-retryable 4xx status
- **WHEN** the uploader handles the response
- **THEN** it records permanent failure details in sqlite
- **AND** future polls skip that transcript

#### Scenario: Uploader converts Phoenix capture time to UTC

- **GIVEN** a transcript filename starts with `20260528_001043_`
- **WHEN** the uploader builds the request payload
- **THEN** it converts the America/Phoenix capture time to `2026-05-28T07:10:43Z`

### Requirement: OpenAPI documents dispatch endpoints

The system SHALL document the new SDR dispatch admin endpoints in the public OpenAPI document.

#### Scenario: OpenAPI includes dispatch paths

- **WHEN** a client fetches `/v1/openapi.json`
- **THEN** the document contains `/v1/admin/dispatch/transcript`
- **AND** contains `/v1/admin/dispatch/transcripts/recent`

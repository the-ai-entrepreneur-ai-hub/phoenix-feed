## ADDED Requirements

### Requirement: Conservative SDR transcript promotion

The system SHALL promote only high-confidence SDR dispatch transcripts with a recognizable Phoenix dispatch structure into active incident rows.

#### Scenario: Clean dispatch transcript is promoted

- **GIVEN** a `dispatch_transcripts` row has `verification_confidence >= 0.80`
- **AND** `display_text` contains a CDEC channel marker
- **AND** `display_text` contains at least one recognized fire/EMS unit
- **AND** `display_text` contains a Phoenix-style address or intersection
- **AND** geocoding succeeds
- **WHEN** the dispatch parser processes the row
- **THEN** it inserts an `incidents` row with `source = "sdr_audio"`
- **AND** `incident_id = "sdr-<dispatch_transcripts.id>"`
- **AND** non-null point geometry
- **AND** inserts one `incident_units` row per extracted unit
- **AND** sets `dispatch_transcripts.parsed_at`
- **AND** sets `dispatch_transcripts.parsed_incident_id` to the promoted `incidents.id`

#### Scenario: Gate failure is marked parsed without promotion

- **GIVEN** a transcript is missing confidence, CDEC marker, recognized unit, address, or nature
- **WHEN** the dispatch parser processes the row
- **THEN** it sets `parsed_at`
- **AND** leaves `parsed_incident_id` null
- **AND** does not insert an incident

#### Scenario: Geocode failure is marked parsed without promotion

- **GIVEN** a transcript passes the parser gate
- **AND** geocoding returns no result or an error
- **WHEN** the dispatch parser processes the row
- **THEN** it sets `parsed_at`
- **AND** leaves `parsed_incident_id` null
- **AND** does not insert an incident

#### Scenario: Multiple parser instances share the backlog safely

- **GIVEN** two parser instances are processing the same dispatch backlog
- **WHEN** they select unparsed rows
- **THEN** each row is locked with `FOR UPDATE SKIP LOCKED`
- **AND** no transcript produces duplicate incident rows

### Requirement: SDR incidents use the existing active feed

The system SHALL surface promoted SDR incidents through the existing active incident endpoint without source-specific query behavior.

#### Scenario: Active feed includes promoted SDR incident

- **GIVEN** a promoted `sdr_audio` incident has non-null point geometry
- **WHEN** a client requests `GET /v1/incidents/active`
- **THEN** the response includes the SDR incident alongside any mapserver incidents
- **AND** the response shape matches existing active incidents

#### Scenario: Mapserver active feed behavior is unchanged

- **GIVEN** existing `phoenix-fire-mapserver` incidents are active
- **WHEN** SDR promotion is enabled
- **THEN** the active query still filters active incidents by `cleared_at IS NULL` and `geom IS NOT NULL`
- **AND** does not filter by source

### Requirement: Server-side geocoding cache

The system SHALL geocode SDR locations through a server-side Mapbox token with persistent cache and bounded outbound call rate.

#### Scenario: Parser refuses to start without Mapbox token

- **GIVEN** `MAPBOX_TOKEN` is unset
- **WHEN** `cmd/dispatch-parser` starts
- **THEN** it exits with a clear configuration error

#### Scenario: Successful geocode is cached permanently

- **GIVEN** an address has a successful cache entry
- **WHEN** the parser needs that address again
- **THEN** it returns cached coordinates
- **AND** increments cache `hits`
- **AND** does not call Mapbox

#### Scenario: Failed geocode is retried after 24 hours

- **GIVEN** an address has a failed cache entry less than 24 hours old
- **WHEN** the parser needs that address again
- **THEN** it treats the cached failure as no result
- **AND** increments cache `hits`
- **AND** does not call Mapbox

- **GIVEN** an address has a failed cache entry at least 24 hours old
- **WHEN** the parser needs that address again
- **THEN** it retries Mapbox and stores the new outcome

### Requirement: Dispatch parser health visibility

The system SHALL expose parser progress and failure counters in the existing admin dispatch health response.

#### Scenario: Health includes parser fields

- **GIVEN** an admin request has a valid bearer token
- **WHEN** `GET /v1/admin/dispatch/health` is called
- **THEN** the response includes `parser_last_batch_at`
- **AND** includes `parser_rows_promoted_last_hour`
- **AND** includes `parser_rows_gate_failed_last_hour`
- **AND** includes `parser_rows_geocode_failed_last_hour`
- **AND** includes `parser_backlog_unparsed`

### Requirement: Incident secondary id and geocode cache migration

The system SHALL add a unique numeric incident id and a geocode cache without changing the composite incident primary key.

#### Scenario: Migration adds incident id

- **WHEN** migration `0004_incidents_id_and_geocode_cache.sql` is applied
- **THEN** `incidents.id` exists
- **AND** is unique
- **AND** the primary key remains `(source, incident_id)`

#### Scenario: Migration adds geocode cache

- **WHEN** migration `0004_incidents_id_and_geocode_cache.sql` is applied
- **THEN** `geocode_cache` exists with `address`, `lon`, `lat`, `geocoded_at`, `success`, and `hits`

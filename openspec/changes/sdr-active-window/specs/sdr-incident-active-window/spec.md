## ADDED Requirements

### Requirement: SDR transcript freshness gate
The system SHALL reject stale SDR dispatch transcripts before incident promotion.

#### Scenario: Stale transcript is consumed without promotion
- **GIVEN** a `dispatch_transcripts` row has `captured_at` older than `DISPATCH_MAX_AGE`
- **WHEN** the dispatch parser processes the row
- **THEN** it sets `parsed_at`
- **AND** leaves `parsed_incident_id` null
- **AND** does not insert an `incidents` row
- **AND** records the gate failure reason as `stale_capture` in parser logs and batch counters

#### Scenario: Fresh transcript remains eligible for promotion
- **GIVEN** a `dispatch_transcripts` row has `captured_at` within `DISPATCH_MAX_AGE`
- **AND** the transcript passes the existing parser and geocode gates
- **WHEN** the dispatch parser processes the row
- **THEN** it inserts an `incidents` row with `source = "sdr_audio"`
- **AND** sets `parsed_incident_id` to the promoted incident database id

### Requirement: SDR active window clearing
The system SHALL clear active `sdr_audio` incidents after the configured `SDR_ACTIVE_WINDOW` has elapsed from `incident_date`.

#### Scenario: Expired SDR incident is cleared
- **GIVEN** an `incidents` row has `source = "sdr_audio"`
- **AND** `cleared_at IS NULL`
- **AND** `incident_date` is older than `SDR_ACTIVE_WINDOW`
- **WHEN** the SDR janitor sweep runs
- **THEN** it sets `cleared_at`
- **AND** the row no longer appears in `active_incidents`

#### Scenario: Fresh SDR incident remains active
- **GIVEN** an `incidents` row has `source = "sdr_audio"`
- **AND** `cleared_at IS NULL`
- **AND** `incident_date` is within `SDR_ACTIVE_WINDOW`
- **WHEN** the SDR janitor sweep runs
- **THEN** it leaves `cleared_at` null
- **AND** the row remains eligible for `active_incidents`

#### Scenario: Mapserver incident lifecycle is unchanged
- **GIVEN** an active `phoenix-fire-mapserver` incident exists
- **WHEN** the SDR janitor sweep runs
- **THEN** it does not update that row
- **AND** mapserver clearing remains controlled by the existing missing/cleared lifecycle

### Requirement: Stale SDR cleanup migration
The system SHALL include a forward-only idempotent migration that clears existing stale active SDR rows.

#### Scenario: Existing stale SDR row is cleared by migration
- **GIVEN** an active `sdr_audio` incident has `incident_date < NOW() - INTERVAL '2 hours'`
- **WHEN** the cleanup migration is applied
- **THEN** `cleared_at` is set
- **AND** applying the migration again does not change already-cleared rows

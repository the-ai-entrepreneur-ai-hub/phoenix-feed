## ADDED Requirements

### Requirement: Incident response data freshness metadata

The system SHALL include server-computed incident data freshness fields in incident response `meta` envelopes without changing existing scrape pipeline freshness semantics.

#### Scenario: Active feed reports newest returned incident

- **GIVEN** `GET /v1/incidents/active` returns three incidents
- **AND** the newest returned `incident_date` is `2026-05-26T00:00:00Z`
- **AND** API server time is `2026-05-26T01:00:00Z`
- **WHEN** the response is serialized
- **THEN** `meta.newest_incident_at` is `2026-05-26T00:00:00Z`
- **AND** `meta.data_staleness_seconds` is `3600`
- **AND** `meta.source_last_success_at` keeps its scrape-success timestamp
- **AND** `meta.data_age_seconds` keeps its scrape-age value

#### Scenario: Active feed has no incidents

- **GIVEN** `GET /v1/incidents/active` returns zero incidents
- **WHEN** the response is serialized
- **THEN** `meta.newest_incident_at` is `null`
- **AND** `meta.data_staleness_seconds` is `null`

#### Scenario: Newest incident equals server time

- **GIVEN** an incident response includes a newest returned `incident_date` equal to API server time
- **WHEN** the response is serialized
- **THEN** `meta.data_staleness_seconds` is `0`

#### Scenario: Manual refresh shares active meta semantics

- **GIVEN** `POST /v1/incidents/refresh` returns the active incident cache response shape
- **WHEN** the response is serialized
- **THEN** `meta.newest_incident_at` and `meta.data_staleness_seconds` use the same server-side computation as `GET /v1/incidents/active`

#### Scenario: Incident detail uses returned incident timestamp

- **GIVEN** an incident detail endpoint returns one incident and the shared staleness meta envelope
- **WHEN** the response is serialized
- **THEN** `meta.newest_incident_at` is the returned incident's `incident_date`
- **AND** `meta.data_staleness_seconds` is computed from API server time

#### Scenario: OpenAPI documents data freshness metadata

- **WHEN** a client fetches `/v1/openapi.json`
- **THEN** the `StalenessMeta` schema documents `newest_incident_at`
- **AND** the `StalenessMeta` schema documents `data_staleness_seconds`

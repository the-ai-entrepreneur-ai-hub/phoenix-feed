## ADDED Requirements

### Requirement: Active incident API

The system SHALL expose `GET /v1/incidents/active` returning cached active incidents and top-level staleness metadata.

#### Scenario: Empty cache is still a valid response

- **GIVEN** the database contains no active incidents
- **WHEN** a client requests `/v1/incidents/active`
- **THEN** the response status is `200`
- **AND** `incidents` is an empty array
- **AND** `meta` contains `source_last_success_at`, `data_age_seconds`, and `parser_version`

#### Scenario: Spatial filters are validated

- **GIVEN** a request includes both `bbox` and `lat`/`lon`/`radius_meters`
- **WHEN** the client requests `/v1/incidents/active`
- **THEN** the response status is `400`

### Requirement: Incident detail API

The system SHALL expose `GET /v1/incidents/{source}/{incident_id}` returning one incident, unit history, event history, and staleness metadata.

#### Scenario: Missing incident

- **GIVEN** no incident exists for the requested `(source, incident_id)`
- **WHEN** the client requests the detail endpoint
- **THEN** the response status is `404`

### Requirement: Unit and event history

The ingester SHALL persist observed unit status intervals and structured lifecycle events.

#### Scenario: Repeated status extends an interval

- **GIVEN** an incident already has unit `E2203` with status `On Scene`
- **WHEN** the next successful poll observes `E2203` with status `On Scene`
- **THEN** the existing `incident_units` row has an updated `last_observed_at`
- **AND** no duplicate interval row is created

#### Scenario: Status change creates a new interval

- **GIVEN** an incident already has unit `E2203` with status `Dispatched`
- **WHEN** a later successful poll observes `E2203` with status `On Scene`
- **THEN** a new `incident_units` row is inserted for `On Scene`
- **AND** an `updated` event records the status change

### Requirement: Source-aware health

The system SHALL expose `/v1/health` with aggregate source state and return `503` when source data is stale.

#### Scenario: Stale source

- **GIVEN** the latest successful Phoenix Fire poll is older than 10 minutes
- **WHEN** a client requests `/v1/health`
- **THEN** the response status is `503`
- **AND** the body state is `down`

### Requirement: Contract canary

The canary SHALL run the six contract checks in `docs/architecture.md` section 7 and persist the result to `contract_canary`.

#### Scenario: Missing field

- **GIVEN** the upstream response omits required field `Incident`
- **WHEN** the canary runs
- **THEN** it writes `passed=false`
- **AND** the `drift` JSON names the missing field

### Requirement: Raw retention

The janitor SHALL drop old `raw` JSONB payloads without deleting normalized incident data.

#### Scenario: Old raw payload

- **GIVEN** an incident has non-null `raw` and `last_seen_at` older than `RAW_RETENTION`
- **WHEN** the janitor sweep runs
- **THEN** `raw` becomes null
- **AND** `raw_dropped_at` is set

### Requirement: Static web client

The repository SHALL include a static `web/index.html` client that opens directly in a browser and calls the local API.

#### Scenario: Local API available

- **GIVEN** `cmd/api` is listening on `localhost:8080`
- **WHEN** `web/index.html` is opened from disk
- **THEN** it fetches `/v1/incidents/active`
- **AND** renders map and list views
- **AND** displays emergency disclaimer and attribution

### Requirement: Paid history placeholder

The system SHALL expose `/v1/incidents/history` as a non-functional paid placeholder for v0.2.

#### Scenario: Paid tier disabled

- **GIVEN** `PAID_TIER_ENABLED` is unset or false
- **WHEN** a client requests `/v1/incidents/history`
- **THEN** the response status is `402`
- **AND** the response explains that paid history is not enabled in v0.2

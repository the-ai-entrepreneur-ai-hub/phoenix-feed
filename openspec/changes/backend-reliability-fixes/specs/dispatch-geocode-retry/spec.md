## ADDED Requirements

### Requirement: Transient geocode failures remain retryable
The dispatch parser SHALL leave transcripts eligible for retry after transient geocode failures.

#### Scenario: Timeout geocode failure is retried
- **GIVEN** a fresh SDR transcript passes the parser gate
- **AND** geocoding fails with a timeout, network error, Mapbox 429, or Mapbox 5xx
- **WHEN** the dispatch parser processes the row
- **THEN** `parsed_at` remains null
- **AND** `parsed_incident_id` remains null
- **AND** `geocode_attempts` increments
- **AND** a later successful geocode can promote the same transcript.

#### Scenario: Permanent no-result geocode failure is consumed
- **GIVEN** a fresh SDR transcript passes the parser gate
- **AND** geocoding returns no result or Mapbox 400
- **WHEN** the dispatch parser processes the row
- **THEN** `parsed_at` is set
- **AND** `parsed_incident_id` remains null
- **AND** no incident is inserted.

#### Scenario: Retry cap prevents endless geocode loops
- **GIVEN** a fresh SDR transcript repeatedly encounters retryable geocode failures
- **WHEN** the parser reaches the configured geocode attempt cap
- **THEN** the transcript is consumed with `parsed_at` set
- **AND** `parsed_incident_id` remains null.

### Requirement: Retryable geocode failures are not negative-cached
The geocode cache SHALL store negative cache entries only for permanent no-result outcomes.

#### Scenario: Provider timeout is not stored as a failed cache entry
- **GIVEN** Mapbox returns a retryable provider failure
- **WHEN** cached geocoding handles the failure
- **THEN** no failed cache entry is stored for the address.

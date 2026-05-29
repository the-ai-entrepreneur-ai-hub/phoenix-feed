## ADDED Requirements

### Requirement: Poll success is recorded after backend apply writes
The system SHALL publish a successful source poll only after the backend apply path has completed incident and lifecycle writes for that poll.

#### Scenario: Incident write failure does not advance source freshness
- **GIVEN** a source has an older successful poll
- **AND** a newer upstream poll result is otherwise successful
- **WHEN** an incident write fails while applying the newer poll
- **THEN** the newer poll is not recorded as a successful source poll
- **AND** `source_last_success_at` remains the older successful poll time
- **AND** missing/cleared lifecycle updates are not run for the failed apply.

#### Scenario: Failed upstream poll remains recorded as failed
- **GIVEN** an upstream poll result has a transport, parser, or non-2xx error
- **WHEN** the lifecycle manager applies the poll result
- **THEN** a failed `source_polls` row is recorded for audit history
- **AND** source freshness is not advanced.

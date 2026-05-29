## ADDED Requirements

### Requirement: SQL file application hard-stops on statement errors
The repository SHALL apply schema and migration SQL with `ON_ERROR_STOP=1` and per-file transaction boundaries.

#### Scenario: Failing SQL exits non-zero
- **GIVEN** a SQL file contains a statement error between successful statements
- **WHEN** the repository SQL application helper applies the file
- **THEN** the helper exits non-zero
- **AND** later statements in that file are not allowed to mask the failure.

#### Scenario: Deployment commands use strict psql flags
- **WHEN** a reader follows the Makefile, smoke script, cloud-init, or deployment runbook SQL application path
- **THEN** each `psql` invocation uses `ON_ERROR_STOP=1`
- **AND** each SQL file is applied with `--single-transaction`.

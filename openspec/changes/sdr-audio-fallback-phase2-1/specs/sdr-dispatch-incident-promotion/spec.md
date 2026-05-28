## ADDED Requirements

### Requirement: CDEC transcription variants are accepted

The SDR dispatch parser SHALL accept the observed Whisper transcription variants of the Phoenix `CDEC` marker without changing the existing confidence, unit, or address gate requirements.

#### Scenario: Sea Deck variant passes the gate

- **GIVEN** a transcript has `verification_confidence >= 0.80`
- **AND** contains a recognized fire/EMS unit
- **AND** contains `Sea Deck 12`
- **AND** contains a Phoenix-style address
- **WHEN** the parser evaluates the transcript
- **THEN** the transcript passes the CDEC marker gate
- **AND** the parsed channel is `CDEC 12`

#### Scenario: Seabex variant passes the gate

- **GIVEN** a transcript has `verification_confidence >= 0.80`
- **AND** contains a recognized fire/EMS unit
- **AND** contains `Seabex 12`
- **AND** contains a Phoenix-style address
- **WHEN** the parser evaluates the transcript
- **THEN** the transcript passes the CDEC marker gate
- **AND** the parsed channel is `CDEC 12`

#### Scenario: FedEx variant passes the gate

- **GIVEN** a transcript has `verification_confidence >= 0.80`
- **AND** contains a recognized fire/EMS unit
- **AND** contains `FedEx 12`
- **AND** contains a Phoenix-style address
- **WHEN** the parser evaluates the transcript
- **THEN** the transcript passes the CDEC marker gate
- **AND** the parsed channel is `CDEC 12`

#### Scenario: CDC variant passes the gate

- **GIVEN** a transcript has `verification_confidence >= 0.80`
- **AND** contains a recognized fire/EMS unit
- **AND** contains `CDC 12`
- **AND** contains a Phoenix-style address
- **WHEN** the parser evaluates the transcript
- **THEN** the transcript passes the CDEC marker gate
- **AND** the parsed channel is `CDEC 12`

### Requirement: SDR nature text is sanitized

The SDR dispatch parser SHALL return a short canonical nature description from a curated dispatch nature list instead of promoting repeated dispatch fragments as user-facing text.

#### Scenario: Reordered repeated dispatch does not over-capture nature

- **GIVEN** a transcript repeats the dispatch with reordered fields
- **AND** the text between the first marker and detected address includes address-like fragments and unit fragments
- **AND** the transcript contains `Difficulty Breathing`
- **WHEN** the parser extracts the nature
- **THEN** the parsed nature is `Difficulty Breathing`
- **AND** the parsed nature length is no more than 50 characters

#### Scenario: Unknown non-empty nature candidate falls back safely

- **GIVEN** a transcript passes the existing confidence, marker, unit, and address gates
- **AND** the non-empty nature candidate does not match the curated nature list
- **WHEN** the parser extracts the nature
- **THEN** the parsed nature is `Dispatch Call`

### Requirement: Legacy over-captured SDR natures are cleaned

The database migration SHALL clean existing long comma-delimited SDR nature descriptions without touching other sources.

#### Scenario: Long comma-delimited SDR nature is trimmed

- **GIVEN** an `incidents` row has `source = "sdr_audio"`
- **AND** `nature_desc` is longer than 50 characters
- **AND** `nature_desc` contains a comma
- **WHEN** migration `0005_cleanup_bad_natures.sql` is applied
- **THEN** `nature_desc` is set to the trimmed first comma-separated substring

#### Scenario: Non-SDR rows are not changed

- **GIVEN** an `incidents` row has a source other than `sdr_audio`
- **WHEN** migration `0005_cleanup_bad_natures.sql` is applied
- **THEN** the row's `nature_desc` is unchanged

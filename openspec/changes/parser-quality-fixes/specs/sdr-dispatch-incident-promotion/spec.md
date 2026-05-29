## ADDED Requirements

### Requirement: Multi-dispatch SDR transcripts use one dispatch segment

The SDR parser SHALL extract units, nature, channel, and address from the first complete CDEC dispatch segment in a transcript and SHALL NOT combine fields from later distinct dispatches in the same transcript.

#### Scenario: Later units are not attached to the first call

- **GIVEN** a high-confidence transcript contains `Ladder 259 CDEC 12 seizure 3473 East Crescent Way`
- **AND** the same transcript later contains `Medic 2207 CDEC 12 stroke 2505 West Plata Avenue`
- **WHEN** the parser evaluates the transcript
- **THEN** the parsed nature is `Seizure`
- **AND** the parsed address is `3473 East Crescent Way`
- **AND** the parsed units contain `Ladder 259`
- **AND** the parsed units do not contain `Medic 2207`

#### Scenario: Distinct later CDEC markers cannot complete an earlier call

- **GIVEN** a high-confidence transcript has a first CDEC dispatch segment
- **AND** a later distinct CDEC-like marker appears before any address has been parsed for that first segment
- **WHEN** the parser evaluates the transcript
- **THEN** the parser does not borrow the later segment's address for the earlier segment

### Requirement: Audited CDEC dispatch variants pass without lowering the gate

The SDR parser SHALL accept audited real dispatch transcript variants only when they still satisfy the confidence, CDEC-like marker, unit, nature, and address gates.

#### Scenario: K-Deck variant normalizes to CDEC

- **GIVEN** a transcript has `verification_confidence >= 0.80`
- **AND** contains `Engine 207 K-Deck 7 Hill Person 2525 East Southern Avenue`
- **WHEN** the parser evaluates the transcript
- **THEN** the transcript passes the gate
- **AND** the parsed channel is `CDEC 7`
- **AND** the parsed nature is `Ill Person`

#### Scenario: Hyphenated AMR unit is recognized

- **GIVEN** a transcript has `verification_confidence >= 0.80`
- **AND** contains `A-M-R-2-0-7. Sea Deck 12. Fall. C-L-F. 154-35. East Cabern Drive`
- **WHEN** the parser evaluates the transcript
- **THEN** the transcript passes the gate
- **AND** the parsed unit is `AMR 207`
- **AND** the parsed address is `154-35 East Cabern Drive`

#### Scenario: Punctuated and split house numbers are normalized

- **GIVEN** a transcript has `verification_confidence >= 0.80`
- **AND** contains `182 31 East Coronado Cave Court`
- **WHEN** the parser extracts the address
- **THEN** the parsed address is `18231 East Coronado Cave Court`

#### Scenario: Fire Channel-only transcripts remain rejected

- **GIVEN** a transcript contains `Fire channel A7`
- **AND** does not contain a CDEC-like marker accepted by this parser
- **WHEN** the parser evaluates the transcript
- **THEN** the transcript fails the CDEC gate

### Requirement: Live transcript regression samples are locked

The SDR parser SHALL keep regression coverage for the live high-confidence transcript samples used to validate parser quality.

#### Scenario: Seven parseable live samples pass

- **GIVEN** the eight live transcript samples from the parser-quality audit task
- **WHEN** the parser evaluates them at confidence `0.91`
- **THEN** the seven samples with complete addresses pass the gate
- **AND** the truncated `520 ...` sample remains rejected because it lacks a publishable address

## ADDED Requirements

### Requirement: Landing mirrors approved legacy layout

The public landing page SHALL mirror the approved legacy azincidentalert.com coming-soon layout while presenting Cactus Watch branding and Dan's approved copy.

#### Scenario: rebranded landing page

- **GIVEN** a visitor opens `/`
- **WHEN** the landing page is rendered
- **THEN** the page uses the legacy cream/light layout and composition
- **AND** the logo reference uses `/favicon.svg`
- **AND** the headline reads `Stay Informed with Cactus Watch`
- **AND** the page includes the coming-soon ribbon
- **AND** no visible `AZ Incidents` brand string remains

### Requirement: Landing assets are self-hosted and optimized

The landing page SHALL use only same-origin assets and SHALL provide optimized hero screenshots.

#### Scenario: hero images

- **GIVEN** Dan's three iPhone screenshots have been copied into `web/landing/img/`
- **WHEN** the landing page requests hero device imagery
- **THEN** each hero image has a resized PNG fallback
- **AND** each hero image has a WebP sibling
- **AND** each image long side is between 720 and 900 pixels

#### Scenario: CSP-safe resources

- **WHEN** landing HTML and CSS are inspected
- **THEN** they contain no Tailwind CDN reference
- **AND** no Google Fonts or gstatic reference
- **AND** no cdnjs or jsdelivr reference
- **AND** no third-party script or tracker

### Requirement: Legal and informational pages use restored chrome

Privacy, terms, about, and FAQ pages SHALL use the restored cream/light chrome while preserving existing body content.

#### Scenario: sub-page style restoration

- **GIVEN** a visitor opens `/privacy/`, `/terms/`, `/about/`, or `/faq/`
- **WHEN** each page is rendered
- **THEN** it uses the same cream/light header, typography, background, and footer style as the landing page
- **AND** the page body copy remains semantically unchanged from the pre-change file

### Requirement: Active incidents remain while still observed

The active feed SHALL NOT drop an incident solely because its dispatch time is older than 90 minutes when the latest source poll still observes it.

#### Scenario: long-running observed incident

- **GIVEN** an incident was dispatched about 2 hours ago
- **AND** the latest poll still observed the incident
- **WHEN** the active incident feed is built
- **THEN** the incident remains included in the active feed

#### Scenario: incident no longer observed

- **GIVEN** an incident has stopped appearing in source data
- **WHEN** its not-seen duration exceeds the cleared incident retention window
- **THEN** the incident is excluded from the active feed

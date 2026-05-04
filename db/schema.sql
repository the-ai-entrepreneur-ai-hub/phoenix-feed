-- phoenix-feed schema
-- v1.0 — 4 May 2026
--
-- Postgres 16+ with PostGIS 3.4+
-- Reflects the Codex-reviewed design: composite (source, incident_id) PK,
-- normalized incident_units history, source_polls audit, raw JSONB with TTL.

CREATE EXTENSION IF NOT EXISTS postgis;

-- ----------------------------------------------------------------------------
-- source_polls: one row per HTTP poll attempt against an upstream source.
-- Drives clearing logic and observability. Successful polls are the only
-- thing that count toward the "incident is gone" decision.
-- ----------------------------------------------------------------------------
CREATE TABLE source_polls (
    poll_id          BIGSERIAL    PRIMARY KEY,
    source           TEXT         NOT NULL,
    request_url      TEXT         NOT NULL,
    started_at       TIMESTAMPTZ  NOT NULL,
    finished_at      TIMESTAMPTZ,
    status_code      INT,
    latency_ms       INT,
    feature_count    INT,
    payload_sha256   TEXT,                       -- hash of normalized response, dedup signal
    parser_version   TEXT         NOT NULL,      -- e.g. "phx-fire-2026-05"
    success          BOOLEAN      NOT NULL DEFAULT FALSE,
    error            TEXT,                       -- non-null on parse/transport failure
    notes            JSONB                       -- free-form (e.g. backoff state, jitter applied)
);
CREATE INDEX source_polls_source_started_idx
    ON source_polls (source, started_at DESC);
CREATE INDEX source_polls_success_idx
    ON source_polls (source, success, started_at DESC);

COMMENT ON TABLE source_polls IS
    'One row per upstream poll. Success-only polls count toward incident clearing.';

-- ----------------------------------------------------------------------------
-- incidents: current-state row per (source, incident_id).
-- Composite PK because the same numeric ID could appear in different sources
-- (Phoenix Fire MapServer vs scanner+Whisper-derived).
-- ----------------------------------------------------------------------------
CREATE TABLE incidents (
    source             TEXT         NOT NULL,
    incident_id        TEXT         NOT NULL,
    nature_code        TEXT,
    nature_desc        TEXT,
    units              JSONB,                    -- snapshot: [{unit, status}, ...]
    channel            TEXT,
    symbol_code        TEXT,
    location_text      TEXT,
    geom               geometry(Point, 4326),
    incident_date      TIMESTAMPTZ  NOT NULL,    -- from feed (Date field)
    received_at        TIMESTAMPTZ  NOT NULL,    -- first poll observing this incident
    last_seen_at       TIMESTAMPTZ  NOT NULL,    -- updated on every poll while active
    last_seen_poll_id  BIGINT       REFERENCES source_polls(poll_id),
    missing_since      TIMESTAMPTZ,              -- first successful-poll time we did NOT see it
    cleared_at         TIMESTAMPTZ,              -- set after N consecutive successful misses
    reopen_count       INT          NOT NULL DEFAULT 0,
    raw                JSONB,                    -- original feature, TTL'd at 30 days
    raw_dropped_at     TIMESTAMPTZ,              -- set when raw is null'd by retention job
    PRIMARY KEY (source, incident_id)
);
CREATE INDEX incidents_geom_gix       ON incidents USING GIST (geom);
CREATE INDEX incidents_received_idx   ON incidents (received_at DESC);
CREATE INDEX incidents_incident_date_idx ON incidents (incident_date DESC);
CREATE INDEX incidents_last_seen_idx  ON incidents (last_seen_at DESC);
CREATE INDEX incidents_active_idx     ON incidents (source, cleared_at) WHERE cleared_at IS NULL;

COMMENT ON COLUMN incidents.missing_since IS
    'First successful poll where this incident was not present. NULL while observed. Becomes cleared_at after threshold.';
COMMENT ON COLUMN incidents.reopen_count IS
    'How many times this incident has been re-observed after a clear. >0 is a signal of a noisy upstream.';
COMMENT ON COLUMN incidents.raw IS
    'Original upstream feature. TTL: dropped after 30 days by retention job. raw_dropped_at records when.';

-- ----------------------------------------------------------------------------
-- incident_units: history of unit-level observations.
-- One row per (incident, unit, status) interval — collapses identical
-- consecutive observations into a single (first, last) range.
-- ----------------------------------------------------------------------------
CREATE TABLE incident_units (
    id                 BIGSERIAL    PRIMARY KEY,
    source             TEXT         NOT NULL,
    incident_id        TEXT         NOT NULL,
    unit               TEXT         NOT NULL,
    status             TEXT         NOT NULL,    -- "Dispatched", "On Scene", "Available", etc
    first_observed_at  TIMESTAMPTZ  NOT NULL,
    last_observed_at   TIMESTAMPTZ  NOT NULL,
    first_poll_id      BIGINT       NOT NULL REFERENCES source_polls(poll_id),
    last_poll_id       BIGINT       NOT NULL REFERENCES source_polls(poll_id),
    FOREIGN KEY (source, incident_id) REFERENCES incidents(source, incident_id) ON DELETE CASCADE
);
CREATE INDEX incident_units_incident_idx
    ON incident_units (source, incident_id, first_observed_at);
CREATE INDEX incident_units_unit_idx
    ON incident_units (unit, first_observed_at DESC);

COMMENT ON TABLE incident_units IS
    'Observed (not authoritative) timeline of unit dispatch states. Times reflect when WE saw the change, not when the agency dispatched.';

-- ----------------------------------------------------------------------------
-- incident_events: lifecycle events for an incident (created, updated, cleared, reopened).
-- Useful for debugging, audit, and powering the activity feed in the UI.
-- ----------------------------------------------------------------------------
CREATE TABLE incident_events (
    id            BIGSERIAL    PRIMARY KEY,
    source        TEXT         NOT NULL,
    incident_id   TEXT         NOT NULL,
    event_type    TEXT         NOT NULL,         -- 'created' | 'updated' | 'cleared' | 'reopened'
    occurred_at   TIMESTAMPTZ  NOT NULL,
    poll_id       BIGINT       REFERENCES source_polls(poll_id),
    delta         JSONB,                         -- what changed (units added/removed, status, etc)
    FOREIGN KEY (source, incident_id) REFERENCES incidents(source, incident_id) ON DELETE CASCADE
);
CREATE INDEX incident_events_incident_idx
    ON incident_events (source, incident_id, occurred_at);
CREATE INDEX incident_events_type_idx
    ON incident_events (event_type, occurred_at DESC);

-- ----------------------------------------------------------------------------
-- contract_canary: one row per scheduled contract check.
-- Verifies that the upstream schema matches our parser expectations.
-- ----------------------------------------------------------------------------
CREATE TABLE contract_canary (
    id                BIGSERIAL    PRIMARY KEY,
    source            TEXT         NOT NULL,
    checked_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    expected_fields   TEXT[]       NOT NULL,     -- our required field names
    actual_fields     TEXT[]       NOT NULL,     -- what the source returned
    output_sr         INT,                       -- spatial reference returned (expect 4326)
    feature_count     INT,
    geom_plausible    BOOLEAN,                   -- x/y look like lon/lat?
    drift             JSONB,                     -- structured diff if anything changed
    passed            BOOLEAN      NOT NULL,
    parser_version    TEXT         NOT NULL
);
CREATE INDEX contract_canary_source_idx
    ON contract_canary (source, checked_at DESC);

-- ----------------------------------------------------------------------------
-- Convenience view: active incidents with derived staleness flag.
-- ----------------------------------------------------------------------------
CREATE OR REPLACE VIEW active_incidents AS
SELECT
    i.*,
    EXTRACT(EPOCH FROM (NOW() - i.last_seen_at))::INT AS seconds_since_last_seen,
    sp.started_at  AS source_last_success_at
FROM incidents i
LEFT JOIN LATERAL (
    SELECT started_at FROM source_polls
    WHERE source = i.source AND success = TRUE
    ORDER BY started_at DESC LIMIT 1
) sp ON TRUE
WHERE i.cleared_at IS NULL;

CREATE TABLE IF NOT EXISTS dispatch_transcripts (
    id              BIGSERIAL PRIMARY KEY,
    wav_filename    TEXT NOT NULL UNIQUE,
    captured_at     TIMESTAMPTZ NOT NULL,
    received_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    audio_duration_s NUMERIC(8,3),
    display_text    TEXT NOT NULL,
    primary_text    TEXT,
    secondary_text  TEXT,
    primary_model   TEXT,
    secondary_model TEXT,
    primary_avg_logprob NUMERIC(8,4),
    verification_confidence NUMERIC(6,4),
    verification_agreement  NUMERIC(6,4),
    review_recommended BOOLEAN NOT NULL DEFAULT FALSE,
    domain_keyword_matches TEXT[],
    domain_keyword_ratio NUMERIC(6,4),
    raw_payload     JSONB NOT NULL,
    parsed_incident_id BIGINT,
    parsed_at       TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON COLUMN dispatch_transcripts.parsed_incident_id IS
    'Phase 1 placeholder. Current incidents rows use composite (source, incident_id), so no FK is enforceable until Phase 2 defines the promotion key.';

CREATE INDEX IF NOT EXISTS idx_dispatch_transcripts_captured_at ON dispatch_transcripts (captured_at DESC);
CREATE INDEX IF NOT EXISTS idx_dispatch_transcripts_received_at ON dispatch_transcripts (received_at DESC);
CREATE INDEX IF NOT EXISTS idx_dispatch_transcripts_review ON dispatch_transcripts (review_recommended, received_at DESC);
CREATE INDEX IF NOT EXISTS idx_dispatch_transcripts_parsed ON dispatch_transcripts (parsed_incident_id) WHERE parsed_incident_id IS NOT NULL;

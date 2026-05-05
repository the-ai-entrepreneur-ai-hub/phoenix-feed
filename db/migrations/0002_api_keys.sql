CREATE TABLE IF NOT EXISTS api_keys (
    id           BIGSERIAL    PRIMARY KEY,
    key_hash     TEXT         NOT NULL UNIQUE,
    tier         TEXT         NOT NULL CHECK (tier IN ('free', 'paid')),
    label        TEXT         NOT NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    revoked_at   TIMESTAMPTZ,
    owner_email  TEXT
);

CREATE INDEX IF NOT EXISTS api_keys_tier_idx
    ON api_keys (tier, created_at DESC)
    WHERE revoked_at IS NULL;

COMMENT ON TABLE api_keys IS
    'Manual API keys for Cactus Alert and owner-issued paid access. key_hash stores SHA-256 hex only; plaintext is printed once by cmd/keygen.';

CREATE SEQUENCE IF NOT EXISTS incidents_id_seq OWNED BY NONE;

ALTER TABLE incidents
    ADD COLUMN IF NOT EXISTS id BIGINT;

ALTER TABLE incidents
    ALTER COLUMN id SET DEFAULT nextval('incidents_id_seq'::regclass);

UPDATE incidents
SET id = nextval('incidents_id_seq'::regclass)
WHERE id IS NULL;

SELECT setval(
    'incidents_id_seq'::regclass,
    GREATEST(COALESCE((SELECT MAX(id) FROM incidents), 0), 1),
    TRUE
);

ALTER TABLE incidents
    ALTER COLUMN id SET NOT NULL;

ALTER SEQUENCE incidents_id_seq OWNED BY incidents.id;

CREATE UNIQUE INDEX IF NOT EXISTS incidents_id_unique_idx ON incidents (id);

CREATE TABLE IF NOT EXISTS geocode_cache (
    address     TEXT PRIMARY KEY,
    lon         NUMERIC,
    lat         NUMERIC,
    geocoded_at TIMESTAMPTZ,
    success     BOOLEAN NOT NULL,
    hits        INTEGER NOT NULL DEFAULT 0
);

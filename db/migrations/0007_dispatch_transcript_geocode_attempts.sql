ALTER TABLE dispatch_transcripts
    ADD COLUMN IF NOT EXISTS geocode_attempts INT;

-- Clear SDR audio incidents that were promoted from stale backlog before
-- the SDR active-window janitor existed. Idempotent: already-cleared rows are
-- excluded by the cleared_at predicate.
UPDATE incidents
SET cleared_at = NOW()
WHERE source = 'sdr_audio'
  AND cleared_at IS NULL
  AND incident_date < NOW() - INTERVAL '2 hours';

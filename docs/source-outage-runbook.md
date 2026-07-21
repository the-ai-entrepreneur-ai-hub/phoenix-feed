# Incident source outage runbook

## Source states

- `ok`: the latest poll is authoritative and the last authoritative success is less than three minutes old.
- `degraded`: one or two failed polls, an incomplete/frozen snapshot, or at least three minutes without an authoritative success. Incidents remain last-known and no absence is inferred.
- `down`: no authoritative success, three failed polls, ten minutes without an authoritative success, or ten minutes of an unchanged non-empty snapshot. `/v1/health` returns HTTP 503; incident APIs remain HTTP 200 with retained data.

Defaults are controlled by `SOURCE_DEGRADED_AFTER=3m`, `SOURCE_DOWN_AFTER=10m`, `SOURCE_DOWN_FAILURES=3`, `FROZEN_REPEAT_COUNT=3`, `FROZEN_DOWN_AFTER=10m`, `SUDDEN_COLLAPSE_PERCENT=80`, and `CLEAR_AFTER_MISSES=5`.

## Deployment handoff

Deploy the backend before the Firebase notification function. The current production access and compose command are documented in `docs/ops.md`; do not copy host addresses or environment-file paths from older archived briefs.

Before deployment:

1. Push the reviewed backend commit to the branch production tracks and confirm the production checkout can fast-forward to it.
2. Export the July 20 poll/incident evidence using the queries below before changing production state.
3. Confirm `/opt/phoenix-feed/runtime.env` contains the source thresholds above, or deliberately accept the documented defaults.
4. Name the external-monitor owner and both alert recipients.
5. Do not delete or rewrite `source_polls`, incidents, or notification state. This change reuses existing `source_polls.notes` and `incidents.last_seen_at`; it has no database migration.

On the production host, follow `docs/ops.md` and deploy at least `api` and `ingester`:

```bash
cd /opt/phoenix-feed/app
git status --short
git pull --ff-only
docker compose --env-file /opt/phoenix-feed/runtime.env -f docker-compose.prod.yml config
docker compose --env-file /opt/phoenix-feed/runtime.env -f docker-compose.prod.yml up -d --build api ingester
docker compose --env-file /opt/phoenix-feed/runtime.env -f docker-compose.prod.yml ps
docker compose --env-file /opt/phoenix-feed/runtime.env -f docker-compose.prod.yml logs --tail=100 api ingester
```

Stop if the production checkout has unexplained local changes or cannot fast-forward. After at least one poll completes, verify:

```bash
curl -sS -i https://feed.cactuswatch.com/v1/health
curl -sS https://feed.cactuswatch.com/v1/incidents/active
```

The incident endpoint must remain HTTP 200 and expose `meta.source_status`, `source_status_reason`, `source_last_attempt_at`, and `source_consecutive_failures`. `/v1/health` may return 503 only when its body reports the source or database as down. Then deploy the Firebase function in shadow mode using the Cactus repository's `functions/README.md` instructions.

## External monitor required before production completion

Create one external HTTPS monitor for `https://feed.cactuswatch.com/v1/health`:

- interval: one minute;
- failure: HTTP 503 for two consecutive checks;
- recovery: HTTP 200 for two consecutive checks;
- unresolved repeat: every 30 minutes;
- contacts: Dan and the named technical operator;
- alert fields: source, `source_status_reason`, `last_success_at`, `last_attempt_at`, `consecutive_failures`, and `feature_count` from the response.

The monitor owner, service, technical-operator contact, test-alert timestamp, and recovery-alert timestamp must be recorded in the deployment log. A single degraded response is diagnostic only and must not page.

## Diagnose

Check the public surfaces first:

```bash
curl -sS -i https://feed.cactuswatch.com/v1/health
curl -sS https://feed.cactuswatch.com/v1/incidents/active
```

Inspect recent poll authority and payload hashes without deleting history:

```sql
SELECT started_at, finished_at, status_code, success, feature_count,
       payload_sha256, error,
       notes->>'classification' AS classification,
       notes->>'reason' AS reason,
       notes->>'unchanged_since' AS unchanged_since
FROM source_polls
WHERE source = 'phoenix-fire-mapserver'
ORDER BY started_at DESC
LIMIT 120;
```

Inspect retained incident confirmation and missing state:

```sql
SELECT incident_id, incident_date, last_seen_at AS last_confirmed_at,
       missing_since, cleared_at, reopen_count
FROM incidents
WHERE source = 'phoenix-fire-mapserver'
ORDER BY last_seen_at DESC;
```

For the July 20 incident, export these rows plus ingester and Firebase Function logs before drawing a final hard-failure versus soft/frozen conclusion.

## Notification suppression and recovery

The Firebase scheduler reads the normalized feed and writes no global or per-user notification state while `meta.source_status != ok`. For a shadow rollout or emergency suppression, set `NOTIFICATIONS_SHADOW_ONLY=true` in the Function deployment environment and redeploy only the Function. Shadow mode logs the would-notify diff and advances no state.

After source recovery, confirm one `authoritative_success` poll, `source_status=ok`, retained IDs updated in place, and no duplicate notification for a pre-outage incident/unit pair. A genuinely new incident or newly attached unit should notify once. If any delivery errors occur, the global snapshot remains unchanged so failed recipients can retry; successful recipients are protected by per-user incident/unit keys.

## Controlled staging drill

Run each adapter condition separately: transport timeout, HTTP-200 ArcGIS error, repeated non-empty singleton, confirmed count-zero empty, and changed healthy recovery. Capture `/v1/health`, `/v1/incidents/active`, the SQL queries above, Function shadow logs, and monitor notifications. Confirm no `missing_since`, clear, or notification-state advancement occurs during non-authoritative polls. After recovery, confirm absent retained incidents clear only after five authoritative misses.

## Rollback

Enable Function shadow mode first so notification state cannot advance. Revert or fast-forward the production checkout to the last known-good backend commit, rebuild `api` and `ingester` with the compose command above, and verify both public endpoints. Do not delete or rewrite `source_polls`, incidents, or notification state. If authority classification is suspect, preserve last-known incidents until an authoritative source snapshot is manually verified. The implementation reuses existing nullable poll `notes` and authoritative `last_seen_at`, so there is no database migration to reverse.

# Windows Pipeline Audit - 2026-05-28

| Rank | Severity | Finding |
| --- | --- | --- |
| 1 | P1 silent failure | Uploader backfills oldest files first while health reports current data as OK |
| 2 | P1 silent failure | Half-written transcript JSON is recorded as permanent failure |
| 3 | P1 silent failure | Watchdog omits CactusUploader and the live box already has duplicate uploader processes |
| 4 | P1 silent failure | Real dispatch transcripts are missed by the parser gate |
| 5 | P1 silent failure | 429 and 5xx responses retry hot with no backoff or Retry-After handling |
| 6 | P1 silent failure | Clock and timezone drift can silently corrupt captured_at or permanently reject files |
| 7 | P1 silent failure | Local sqlite corruption or disk-full can stall uploads or reupload accepted files |
| 8 | P2 resource exhaustion | Polling scales with historical files and opens thousands of sqlite connections per batch |
| 9 | P2 resource exhaustion | Recordings retention is unbounded |
| 10 | P2 security | Cactus scripts, logs, and sqlite state are writable by Authenticated Users |

## Audit Baseline

- Local repo branch was `main` at `24dba13` before the audit. `PROJECT.md` was absent, so session-resume state could not be loaded from the expected project source of truth.
- Local worktree before writing this document had only untracked `.autopilot/`; I did not modify code or deployed scripts.
- Read-only SSH access to Dan's box succeeded with:

  ```text
  ssh -i ~/.ssh/ngrok_id_ed25519 -p 21632 Dan@4.tcp.us-cal-1.ngrok.io 'powershell -NoProfile -Command "hostname; Get-Date -Format o"'
  Ledyz
  2026-05-28T06:00:42.3403000-07:00
  ```

- Dan's box has `C:\cactus\cactus_uploader.py` deployed with the same functional code as `scripts/cactus_uploader/cactus_uploader.py`.
- D: drive free space during the audit:

  ```text
  Path           : D:\cactus\recordings
  TotalFiles     : 257985
  OldestWrite    : 4/27/2026 6:41:13 PM
  NewestWrite    : 5/28/2026 6:04:45 AM
  DriveFreeGB    : 772.18
  DriveUsedGB    : 159.2
  ElapsedSeconds : 12.44

  Extension        Count
  ---------        -----
  .audit.json      64498
  .json            64498
  .transcript.json 64491
  .wav             64498
  ```

- Live admin API health reported OK while ingesting backfill:

  ```json
  {
    "last_received_at": "2026-05-28T13:03:55.507467Z",
    "last_received_age_seconds": 6,
    "rows_last_hour": 3452,
    "rows_last_24h": 10871,
    "parser_rows_promoted_last_hour": 20,
    "parser_rows_gate_failed_last_hour": 10846,
    "parser_rows_geocode_failed_last_hour": 5,
    "parser_backlog_unparsed": 0,
    "status": "ok"
  }
  ```

## Uploader backfills oldest files first while health reports current data as OK

Severity: P1 silent failure

Files:
- `scripts/cactus_uploader/cactus_uploader.py:140`
- `scripts/cactus_uploader/cactus_uploader.py:141`
- `scripts/cactus_uploader/cactus_uploader.py:144`
- `internal/api/admin_dispatch_transcripts.go:293`
- `internal/api/admin_dispatch_transcripts.go:302`
- `internal/store/store.go:423`

Reproduction or evidence:
- The uploader scans `sorted(cfg.recordings_dir.glob("*.transcript.json"))`, so it processes lexicographically oldest filenames first (`scripts/cactus_uploader/cactus_uploader.py:140`).
- The batch limit stops attempts after 50, but only after walking old recorded rows (`scripts/cactus_uploader/cactus_uploader.py:141`, `scripts/cactus_uploader/cactus_uploader.py:144`).
- Live sqlite state showed the newest recorded WAV was still from May 4 while new May 28 transcripts existed:

  ```text
  total_recorded: 10990
  oldest_wav: 20260427_184058_unknown.wav
  newest_wav: 20260504_013221_unknown.wav
  ```

- The newest transcript files on disk were current:

  ```text
  20260528_060610_unknown.transcript.json 5/28/2026 6:07:04 AM
  20260528_060601_unknown.transcript.json 5/28/2026 6:06:49 AM
  ```

- The admin recent endpoint returned rows received on May 28 whose `captured_at` values were May 4. Example:

  ```json
  {
    "id": 10872,
    "wav_filename": "20260504_005439_unknown.wav",
    "captured_at": "2026-05-04T07:54:39Z",
    "received_at": "2026-05-28T13:03:55.507467Z",
    "display_text": "Ladder 10-9, Haydeck 8. Ill person. 1010 East Turney Avenue. Ladder 10-9, Haydeck 8.",
    "parsed_incident_id": null
  }
  ```

- Dispatch health uses recent `received_at`, not capture lag (`internal/store/store.go:423`, `internal/api/admin_dispatch_transcripts.go:293`, `internal/api/admin_dispatch_transcripts.go:302`), so it can report `status:"ok"` while the live Windows feed is more than three weeks behind.

Recommended fix:

```python
# Prefer fresh data first; run historical backfill as a separate mode.
pending = list_newest_transcripts(limit=batch_size, not_in_state=True)
for file in pending:
    upload(file)

# Health should include capture lag.
status = "ok" if now - max(received_at) < 10m and now - max(captured_at) < 10m else "stale"
```

Effort: M

## Half-written transcript JSON is recorded as permanent failure

Severity: P1 silent failure

Files:
- `C:\cactus\cactus_transcribe.py:511`
- `C:\cactus\cactus_transcribe.py:512`
- `scripts/cactus_uploader/cactus_uploader.py:159`
- `scripts/cactus_uploader/cactus_uploader.py:161`
- `scripts/cactus_uploader/cactus_uploader.py:162`
- `scripts/cactus_uploader/cactus_uploader.py:125`
- `scripts/cactus_uploader/cactus_uploader.py:131`

Reproduction or evidence:
- `cactus_transcribe.py` writes directly to the final `*.transcript.json` path with `open(..., "w")` and `json.dump(...)`; it does not write a temp file and rename atomically (`C:\cactus\cactus_transcribe.py:511`, `C:\cactus\cactus_transcribe.py:512`).
- The uploader reads any matching final filename immediately (`scripts/cactus_uploader/cactus_uploader.py:159`).
- Any JSON read/parse exception records a permanent failure in sqlite and logs `permanent_failure` (`scripts/cactus_uploader/cactus_uploader.py:161`, `scripts/cactus_uploader/cactus_uploader.py:162`).
- Future polls skip any sqlite row for that WAV filename, including permanent failures (`scripts/cactus_uploader/cactus_uploader.py:125`, `scripts/cactus_uploader/cactus_uploader.py:131`).
- This means a normal race, power loss, or crash during `json.dump` can turn a good transcript into a never-uploaded file even if the file becomes valid later.

Recommended fix:

```python
# transcribe side
tmp = transcript_path.with_suffix(transcript_path.suffix + ".tmp")
with open(tmp, "w", encoding="utf-8") as f:
    json.dump(out, f, indent=2, default=str)
    f.flush()
    os.fsync(f.fileno())
os.replace(tmp, transcript_path)

# uploader side
if json_parse_failed:
    if file_age < timedelta(minutes=5):
        retry_later()
    else:
        quarantine_parse_error_without_permanent_skip()
```

Effort: S

## Watchdog omits CactusUploader and the live box already has duplicate uploader processes

Severity: P1 silent failure

Files:
- `C:\cactus\watchdog.ps1:23`
- `C:\cactus\watchdog.ps1:30`
- `C:\cactus\run_uploader.bat:3`
- `scripts/cactus_uploader/cactus_uploader.py:288`

Reproduction or evidence:
- The watchdog watch list includes capture, transcribe, tunnel, report, scanner, ProScan, and dashboard (`C:\cactus\watchdog.ps1:23` through `C:\cactus\watchdog.ps1:30`), but it does not include `CactusUploader`.
- Scheduled task state during the audit:

  ```text
  TaskName       : CactusUploader
  State          : Ready
  Actions        : wscript.exe "C:\cactus\runhidden.vbs" "C:\cactus\run_uploader.bat"
  LastRunTime    : 5/28/2026 4:27:44 AM
  LastTaskResult : 1
  ```

- Process table at the same time showed two uploader Python processes:

  ```text
  ProcessId   : 7228
  Name        : python.exe
  CommandLine : "C:\cactus\venv\Scripts\python.exe"  "C:\cactus\cactus_uploader.py"

  ProcessId   : 19604
  Name        : python.exe
  CommandLine : "C:\Users\Dan\AppData\Local\Programs\Python\Python311\python.exe" "C:\cactus\cactus_uploader.py"
  ```

- `run_uploader.bat` is a plain process launch (`C:\cactus\run_uploader.bat:3`), and the Python main loop has no singleton lock (`scripts/cactus_uploader/cactus_uploader.py:288`).
- Duplicate instances double-post pending files, compete for sqlite, and double the request rate into the server limiter.

Recommended fix:

```python
lock = acquire_windows_named_mutex("Global\\CactusUploader")
if not lock.acquired:
    print("CactusUploader already running", file=sys.stderr)
    return 0
```

Also add `CactusUploader` to watchdog process checks and configure the scheduled task with "Do not start a new instance" plus an alert on non-zero last result.

Effort: S

## Real dispatch transcripts are missed by the parser gate

Severity: P1 silent failure

Files:
- `internal/dispatch/parser/parser.go:33`
- `internal/dispatch/parser/parser.go:35`
- `internal/dispatch/parser/parser.go:38`
- `internal/dispatch/parser/parser.go:40`
- `internal/dispatch/parser/parser.go:42`
- `internal/dispatch/parser/parser.go:46`
- `internal/dispatch/parser/worker.go:169`
- `internal/dispatch/parser/worker.go:172`

Reproduction or evidence:
- The parser requires `confidence >= 0.80`, a CDEC-like channel, at least one known unit, and an address (`internal/dispatch/parser/parser.go:33` through `internal/dispatch/parser/parser.go:48`).
- If the gate fails, the worker still marks the transcript parsed with no incident id (`internal/dispatch/parser/worker.go:169`, `internal/dispatch/parser/worker.go:172`).
- The recent 20 live rows contained obvious dispatches with address and units but `parsed_incident_id:null`, for example:

  ```text
  20260504_005439_unknown.wav
  display_text: Ladder 10-9, Haydeck 8. Ill person. 1010 East Turney Avenue. Ladder 10-9, Haydeck 8.
  confidence: 0.6685
  parsed_incident_id: null

  20260504_005219_unknown.wav
  display_text: Engine 605 and SQ 605 K-7. Chest pain. Goldust Avenue and Scottsdale Road. Engine 605 and SQ 605 K-7.
  confidence: 0.7082
  parsed_incident_id: null
  ```

- The live health response reported `parser_rows_gate_failed_last_hour:10846` versus `parser_rows_promoted_last_hour:20`.

Recommended fix:

```go
// Parse channel variants seen in real output, not only CDEC.
channelPattern := regexp.MustCompile(`(?i)\b(CDEC|K[- ]?Deck|Haydeck|Fire Channel A)\s*-?\s*(\d+)\b`)

// Do not permanently consume plausible rows.
if plausibleAddressAndUnit && confidence >= 0.60 {
    markParseStatus("needs_review")
    return
}
```

Add live-derived fixtures for `K-7`, `K-Deck`, `Haydeck`, and "Fire Channel A5" transcripts.

Effort: M

## 429 and 5xx responses retry hot with no backoff or Retry-After handling

Severity: P1 silent failure

Files:
- `scripts/cactus_uploader/cactus_uploader.py:171`
- `scripts/cactus_uploader/cactus_uploader.py:176`
- `scripts/cactus_uploader/cactus_uploader.py:197`
- `scripts/cactus_uploader/cactus_uploader.py:198`
- `scripts/cactus_uploader/cactus_uploader.py:199`
- `scripts/cactus_uploader/cactus_uploader.py:288`
- `scripts/cactus_uploader/cactus_uploader.py:293`

Reproduction or evidence:
- Each upload uses `requests.post(..., timeout=15)` without a `requests.Session` (`scripts/cactus_uploader/cactus_uploader.py:171`, `scripts/cactus_uploader/cactus_uploader.py:176`).
- Status `401`, `429`, and any `>=500` are logged as retryable, left unmarked, and retried on the next poll (`scripts/cactus_uploader/cactus_uploader.py:197` through `scripts/cactus_uploader/cactus_uploader.py:199`).
- The main loop sleeps only the fixed poll interval after every pass (`scripts/cactus_uploader/cactus_uploader.py:288`, `scripts/cactus_uploader/cactus_uploader.py:293`).
- Live logs showed sustained 429 bursts:

  ```text
  2026-05-28T13:05:01Z 20260504_010820_unknown.wav retry status=429 body=429 Too Many Requests
  2026-05-28T13:05:02Z 20260504_010834_unknown.wav retry status=429 body=429 Too Many Requests
  2026-05-28T13:05:10Z 20260504_011604_unknown.wav retry status=429 body=429 Too Many Requests
  ```

- The script will retry a same-file 5xx forever. It does not mark permanent failure for 5xx, which is correct for avoiding data loss, but it also has no exponential backoff, jitter, circuit breaker, or respect for `Retry-After`.

Recommended fix:

```python
if response.status_code == 429:
    sleep(parse_retry_after(response.headers) or global_backoff.next())
elif response.status_code >= 500:
    per_file_backoff[wav].increase()
    retry_after(per_file_backoff[wav].delay)
```

Use a single `requests.Session`, cap upload rate below the server limiter, and add a global backoff when many files receive retryable statuses.

Effort: S

## Clock and timezone drift can silently corrupt captured_at or permanently reject files

Severity: P1 silent failure

Files:
- `C:\cactus\cactus_capture.py:218`
- `C:\cactus\cactus_capture.py:221`
- `scripts/cactus_uploader/cactus_uploader.py:27`
- `scripts/cactus_uploader/cactus_uploader.py:217`
- `scripts/cactus_uploader/cactus_uploader.py:223`
- `scripts/cactus_uploader/cactus_uploader.py:229`
- `internal/api/admin_dispatch_transcripts.go:271`
- `internal/api/admin_dispatch_transcripts.go:272`
- `scripts/cactus_uploader/cactus_uploader.py:201`
- `scripts/cactus_uploader/cactus_uploader.py:202`

Reproduction or evidence:
- Capture filenames are built from the Windows local clock via `datetime.datetime.fromtimestamp(start_ts)` (`C:\cactus\cactus_capture.py:218`, `C:\cactus\cactus_capture.py:221`).
- The uploader assumes the filename clock is `America/Phoenix` (`scripts/cactus_uploader/cactus_uploader.py:27`, `scripts/cactus_uploader/cactus_uploader.py:217`, `scripts/cactus_uploader/cactus_uploader.py:223`, `scripts/cactus_uploader/cactus_uploader.py:229`).
- Dan's box timezone is Pacific, and Windows Time is not running:

  ```text
  Id            : Pacific Standard Time
  DisplayName   : (UTC-08:00) Pacific Time (US & Canada)

  w32tm /query /status
  The following error occurred: The service has not been started. (0x80070426)
  ```

- Pacific and Phoenix happen to both be UTC-07 on May 28, but they differ in winter. If the clock moves forward by more than 30 minutes, the API rejects the upload as future (`internal/api/admin_dispatch_transcripts.go:271`, `internal/api/admin_dispatch_transcripts.go:272`), and the uploader records a non-401/409/429 4xx as permanent (`scripts/cactus_uploader/cactus_uploader.py:201`, `scripts/cactus_uploader/cactus_uploader.py:202`).
- If the clock moves backward by days, the backend accepts stale `captured_at` values as long as they are after 2024, and current health still keys off `received_at`.

Recommended fix:

```python
if abs(system_utc_now() - trusted_network_time()) > timedelta(minutes=2):
    fail_fast("clock drift")

if parsed_captured_at < utc_now() - timedelta(hours=6):
    mark_stale_backfill_not_live()
```

Prefer writing UTC capture time into the capture sidecar JSON and having the uploader read that value instead of reconstructing time from a local-clock filename.

Effort: M

## Local sqlite corruption or disk-full can stall uploads or reupload accepted files

Severity: P1 silent failure

Files:
- `scripts/cactus_uploader/cactus_uploader.py:60`
- `scripts/cactus_uploader/cactus_uploader.py:62`
- `scripts/cactus_uploader/cactus_uploader.py:85`
- `scripts/cactus_uploader/cactus_uploader.py:87`
- `scripts/cactus_uploader/cactus_uploader.py:182`
- `scripts/cactus_uploader/cactus_uploader.py:188`
- `scripts/cactus_uploader/cactus_uploader.py:151`
- `scripts/cactus_uploader/cactus_uploader.py:152`
- `scripts/cactus_uploader/cactus_uploader.py:291`
- `scripts/cactus_uploader/cactus_uploader.py:292`

Reproduction or evidence:
- State DB initialization opens sqlite and creates/migrates the table every poll (`scripts/cactus_uploader/cactus_uploader.py:60`, `scripts/cactus_uploader/cactus_uploader.py:62`).
- A successful server response is recorded only after the POST returns 200 (`scripts/cactus_uploader/cactus_uploader.py:182`, `scripts/cactus_uploader/cactus_uploader.py:188`).
- If sqlite is corrupt, locked, or the disk is full after the server accepts a file, `record_uploaded` can fail (`scripts/cactus_uploader/cactus_uploader.py:85`, `scripts/cactus_uploader/cactus_uploader.py:87`). The per-file exception handler logs but leaves the file unrecorded (`scripts/cactus_uploader/cactus_uploader.py:151`, `scripts/cactus_uploader/cactus_uploader.py:152`), so it will be posted again later.
- If disk-full also prevents log writes, `log_status` can raise during both the per-file handler and outer loop handler (`scripts/cactus_uploader/cactus_uploader.py:291`, `scripts/cactus_uploader/cactus_uploader.py:292`), which can terminate the process.

Recommended fix:

```python
def startup():
    assert_state_db_writable()
    assert sqlite_integrity_check() == "ok"
    enable_wal()

def upload_one(file):
    if not state_db_healthy:
        pause_uploads()
    response = post(...)
    record_result_or_spool_for_reconcile(response)
```

Keep a recoverable state backup, store upload attempts and server ids, and fail closed before POSTing if the local state DB cannot be trusted.

Effort: M

## Polling scales with historical files and opens thousands of sqlite connections per batch

Severity: P2 resource exhaustion

Files:
- `scripts/cactus_uploader/cactus_uploader.py:125`
- `scripts/cactus_uploader/cactus_uploader.py:126`
- `scripts/cactus_uploader/cactus_uploader.py:140`
- `scripts/cactus_uploader/cactus_uploader.py:144`

Reproduction or evidence:
- The loop materializes and sorts all `*.transcript.json` paths (`scripts/cactus_uploader/cactus_uploader.py:140`).
- For each old file, it calls `already_recorded`, which opens a new sqlite connection (`scripts/cactus_uploader/cactus_uploader.py:125`, `scripts/cactus_uploader/cactus_uploader.py:126`, `scripts/cactus_uploader/cactus_uploader.py:144`).
- Read-only measurement on Dan's box:

  ```text
  transcript_glob_count: 64501
  glob_sort_seconds: 0.57
  first_file: 20260427_184058_unknown.transcript.json
  last_file: 20260528_060531_unknown.transcript.json
  ```

- Mimicking the uploader's skip loop to find the next 50 pending files:

  ```text
  skipped_before_batch: 11022
  sqlite_connections_opened: 11072
  time_to_find_50_pending_seconds: 2.80
  ```

- As the backfill state grows toward all historical files, the steady-state loop will scan and query through the whole historical directory before it reaches current files.

Recommended fix:

```python
with sqlite3.connect(db) as conn:
    recorded = load_recent_or_indexed_state(conn)
    for path in newest_first_files(recordings_dir):
        if conn.execute("SELECT 1 ...", (wav,)).fetchone():
            continue
        upload(path)
```

Better: maintain a `files_seen` table with filename, mtime, status, attempts, next_attempt_at, and only scan by a bounded recent time window during live mode.

Effort: M

## Recordings retention is unbounded

Severity: P2 resource exhaustion

Files:
- `C:\cactus\cactus_capture.py:222`
- `C:\cactus\cactus_capture.py:223`
- `C:\cactus\cactus_capture.py:224`
- `C:\cactus\cactus_transcribe.py:396`
- `scripts/cactus_uploader/cactus_uploader.py:140`

Reproduction or evidence:
- Capture writes `.wav`, `.json`, and `.audit.json` next to each call (`C:\cactus\cactus_capture.py:222` through `C:\cactus\cactus_capture.py:224`).
- Transcribe writes a fourth `*.transcript.json` file (`C:\cactus\cactus_transcribe.py:396`).
- Uploader reads but never deletes files (`scripts/cactus_uploader/cactus_uploader.py:140`), which matches the deployment constraint but leaves retention to another process.
- Live disk state showed 257,985 files and 159.2 GB used, with 772.18 GB free at the time of audit. At roughly 30k files/day during active periods, inode/directory performance and disk usage will keep growing.

Recommended fix:

```powershell
# Separate retention job, not uploader:
# delete only if uploaded_success AND age > retention_days AND optional archive complete.
Remove-CactusRecordingSet -OlderThanDays 14 -RequireUploadedState
```

Define retention separately for raw WAV, capture sidecar, audit JSON, and transcript JSON. Keep the uploader read-only, but add a post-upload janitor with dry-run and audit logs.

Effort: M

## Cactus scripts, logs, and sqlite state are writable by Authenticated Users

Severity: P2 security

Files:
- `C:\cactus\cactus_uploader.py`
- `C:\cactus\run_uploader.bat`
- `C:\cactus\logs\uploader_inscript.log`
- `C:\cactus\logs\uploader.stderr`
- `D:\cactus\state\uploaded_transcripts.sqlite3`

Reproduction or evidence:
- ACLs on uploader-related files included:

  ```text
  D:\cactus\state\uploaded_transcripts.sqlite3
  NT AUTHORITY\Authenticated Users  Modify, Synchronize
  BUILTIN\Users                     ReadAndExecute, Synchronize

  C:\cactus\cactus_uploader.py
  NT AUTHORITY\Authenticated Users  Modify, Synchronize
  BUILTIN\Users                     ReadAndExecute, Synchronize

  C:\cactus\run_uploader.bat
  NT AUTHORITY\Authenticated Users  Modify, Synchronize
  BUILTIN\Users                     ReadAndExecute, Synchronize
  ```

- `ADMIN_TOKEN` was not found in `C:\cactus\*.bat`, `*.ps1`, or `*.py` search output, and `run_uploader.bat` only sets `PHOENIX_FEED_URL`.
- The token is present as Dan's user environment variable and in the current process environment by length only:

  ```text
  UserAdminTokenPresent    : True
  UserAdminTokenLength     : 64
  MachineAdminTokenPresent : False
  ProcessAdminTokenPresent : True
  ProcessAdminTokenLength  : 64
  ```

- The writable ACL is still risky: any local authenticated user or compromised low-privilege account can modify `run_uploader.bat` or `cactus_uploader.py` to exfiltrate the inherited token at the next run.

Recommended fix:

```powershell
icacls C:\cactus\cactus_uploader.py /inheritance:r /grant:r "Dan:RX" "Administrators:F" "SYSTEM:F"
icacls C:\cactus\run_uploader.bat /inheritance:r /grant:r "Dan:RX" "Administrators:F" "SYSTEM:F"
icacls D:\cactus\state /inheritance:r /grant:r "Dan:(OI)(CI)M" "Administrators:(OI)(CI)F" "SYSTEM:(OI)(CI)F"
```

Use the narrowest account that runs the task. Keep logs readable only to Dan, Administrators, and SYSTEM.

Effort: S

## Missing or wrong recordings directory can create a false-empty pipeline

Severity: P2 silent failure

Files:
- `scripts/cactus_uploader/cactus_uploader.py:52`
- `scripts/cactus_uploader/cactus_uploader.py:136`
- `C:\cactus\cactus_transcribe.py:22`

Reproduction or evidence:
- Transcribe writes to `D:\cactus\recordings` (`C:\cactus\cactus_transcribe.py:22`).
- The uploader allows `CACTUS_RECORDINGS_DIR` override (`scripts/cactus_uploader/cactus_uploader.py:52`).
- On every poll, the uploader creates the configured recordings directory if it does not exist (`scripts/cactus_uploader/cactus_uploader.py:136`).
- A typo such as `D:\cactus\recording` would create an empty directory, keep the process alive, and produce no uploads. There is no fail-fast check that the directory pre-exists or contains expected capture sidecars.

Recommended fix:

```python
if not recordings_dir.exists():
    raise SystemExit(f"recordings dir missing: {recordings_dir}")
if not os.access(recordings_dir, os.R_OK):
    raise SystemExit(f"recordings dir unreadable: {recordings_dir}")
```

Only create state and log directories automatically. Treat the capture directory as owned by the capture pipeline and require it to exist.

Effort: S

## Wrong PHOENIX_FEED_URL can leak ADMIN_TOKEN or permanently fail files

Severity: P2 security

Files:
- `scripts/cactus_uploader/cactus_uploader.py:50`
- `scripts/cactus_uploader/cactus_uploader.py:170`
- `scripts/cactus_uploader/cactus_uploader.py:174`
- `scripts/cactus_uploader/cactus_uploader.py:201`
- `scripts/cactus_uploader/cactus_uploader.py:202`

Reproduction or evidence:
- `PHOENIX_FEED_URL` is accepted directly from the environment and only `rstrip("/")` is applied (`scripts/cactus_uploader/cactus_uploader.py:50`).
- The bearer token is sent to whatever URL that produces (`scripts/cactus_uploader/cactus_uploader.py:170`, `scripts/cactus_uploader/cactus_uploader.py:174`).
- If the URL points to a wrong but reachable host, the uploader may leak `ADMIN_TOKEN`.
- If the wrong host returns 404 or another non-retryable 4xx, the uploader records permanent failure for every attempted file (`scripts/cactus_uploader/cactus_uploader.py:201`, `scripts/cactus_uploader/cactus_uploader.py:202`).

Recommended fix:

```python
allowed_hosts = {"feed.cactuswatch.com"}
url = urllib.parse.urlparse(cfg.phoenix_feed_url)
if url.scheme != "https" or url.hostname not in allowed_hosts:
    raise SystemExit("PHOENIX_FEED_URL must be https://feed.cactuswatch.com")
```

Run a startup GET against `/v1/admin/dispatch/health` and require a valid authenticated response before processing files.

Effort: S

## Missing display_text or verification confidence is silently consumed

Severity: P2 correctness edge

Files:
- `C:\cactus\cactus_transcribe.py:459`
- `C:\cactus\cactus_transcribe.py:475`
- `C:\cactus\cactus_transcribe.py:501`
- `C:\cactus\cactus_transcribe.py:506`
- `internal/api/admin_dispatch_transcripts.go:228`
- `internal/api/admin_dispatch_transcripts.go:243`
- `internal/dispatch/parser/worker.go:208`
- `internal/dispatch/parser/worker.go:223`
- `internal/dispatch/parser/parser.go:35`

Reproduction or evidence:
- On the normal transcribe success path, `display_text` is always included (`C:\cactus\cactus_transcribe.py:459`, `C:\cactus\cactus_transcribe.py:475`) and `verification.confidence` is always written (`C:\cactus\cactus_transcribe.py:501`, `C:\cactus\cactus_transcribe.py:506`), even when `USE_SECONDARY=False`.
- If Whisper returns empty or the cleaning pipeline discards the call, `display_text` can be an empty string. The backend accepts it because it only extracts the string and checks maximum length, not non-empty content (`internal/api/admin_dispatch_transcripts.go:228`).
- If `verification.confidence` is absent in a malformed or older file, the backend stores null (`internal/api/admin_dispatch_transcripts.go:243`).
- The parser scans nullable confidence and leaves the default `0` when null (`internal/dispatch/parser/worker.go:208`, `internal/dispatch/parser/worker.go:223`), then the gate permanently fails as low confidence (`internal/dispatch/parser/parser.go:35`).

Recommended fix:

```go
if strings.TrimSpace(displayText) == "" {
    insertQuarantine(status="empty_display_text")
    return
}
if verificationConfidence == nil {
    insertQuarantine(status="missing_confidence")
    return
}
```

Do not mark malformed-but-stored rows parsed. Add explicit parse status fields so missing model fields remain reviewable.

Effort: S

## Network client has no pooling and no pending-upload cap

Severity: P3 polish

Files:
- `scripts/cactus_uploader/cactus_uploader.py:140`
- `scripts/cactus_uploader/cactus_uploader.py:172`
- `scripts/cactus_uploader/cactus_uploader.py:175`
- `scripts/cactus_uploader/cactus_uploader.py:176`

Reproduction or evidence:
- The script uses `requests.post` for each file rather than a shared `requests.Session` (`scripts/cactus_uploader/cactus_uploader.py:172`).
- It passes JSON per request and a 15-second timeout (`scripts/cactus_uploader/cactus_uploader.py:175`, `scripts/cactus_uploader/cactus_uploader.py:176`), but no connection pool or adapter retry settings.
- `batch_size` caps attempts after the full directory listing begins; it does not cap the pending queue or memory used by `sorted(...)` (`scripts/cactus_uploader/cactus_uploader.py:140`).

Recommended fix:

```python
session = requests.Session()
adapter = HTTPAdapter(pool_connections=2, pool_maxsize=2)
session.mount("https://", adapter)

for file in bounded_pending_iterator(max_scan=1000, max_attempts=batch_size):
    session.post(...)
```

This is secondary to the backoff and live-vs-backfill fixes, but it will reduce overhead once the pipeline is stable.

Effort: S

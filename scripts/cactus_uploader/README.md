# Cactus SDR Transcript Uploader

This script watches Dan's SDR transcript folder for `*.transcript.json` files and uploads each transcript to phoenix-feed at `POST /v1/admin/dispatch/transcript`. It stores local upload state in sqlite so restarts do not resend files that were already accepted or permanently rejected.

## Files

- `cactus_uploader.py` - polling uploader, stdlib plus `requests` and `tzdata`.
- `requirements.txt` - Python runtime dependencies for the uploader virtualenv.
- `run_uploader.bat` - logon task entry point.
- `D:\cactus\state\uploaded_transcripts.sqlite3` - default local state database.
- `C:\cactus\logs\uploader_inscript.log` - default in-script uploader status log.
- `C:\cactus\logs\uploader.stdout` and `C:\cactus\logs\uploader.stderr` - batch-level stdout/stderr redirects.

The uploader never deletes or modifies files under `D:\cactus\recordings`.

## Configuration

Set these environment variables on Dan's Windows box:

- `ADMIN_TOKEN` - required. Existing phoenix-feed admin bearer token.
- `PHOENIX_FEED_URL` - default `https://feed.cactuswatch.com`.
- `CACTUS_RECORDINGS_DIR` - default `D:\cactus\recordings`.
- `CACTUS_UPLOADER_STATE_DB` - default `D:\cactus\state\uploaded_transcripts.sqlite3`.
- `CACTUS_UPLOADER_LOG` - default `C:\cactus\logs\uploader_inscript.log`. Do not set this to the same path as the batch stdout/stderr redirects.
- `CACTUS_UPLOADER_POLL_SECONDS` - default `5`.
- `CACTUS_UPLOADER_BATCH_SIZE` - default `50`.

## Install On Dan's Box

1. Copy `cactus_uploader.py` to `C:\cactus\cactus_uploader.py`.
2. Copy `requirements.txt` to `C:\cactus\requirements.txt`.
3. Copy `run_uploader.bat` to `C:\cactus\run_uploader.bat`.
4. Install the uploader dependencies into the existing Python virtualenv:

   ```bat
   C:\cactus\venv\Scripts\python.exe -m pip install -r C:\cactus\requirements.txt
   ```

5. Set `ADMIN_TOKEN` in Windows Environment Variables for the account that runs the task.
6. Create the log and state directories:

   ```bat
   mkdir C:\cactus\logs
   mkdir D:\cactus\state
   ```

7. Register a scheduled task at logon:

   ```bat
   schtasks /Create /TN CactusUploader /TR "C:\cactus\run_uploader.bat" /SC ONLOGON /RL HIGHEST
   ```

## Verify

After the task is running, check recent rows on the API:

```bash
curl -H "Authorization: Bearer $ADMIN_TOKEN" \
  https://feed.cactuswatch.com/v1/admin/dispatch/transcripts/recent
```

Expect JSON rows with recent `wav_filename`, `captured_at`, `received_at`, and `display_text` values. During active dispatch periods, the newest rows should increment as new transcript files appear.

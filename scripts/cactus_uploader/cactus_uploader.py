#!/usr/bin/env python3
"""Upload Cactus SDR transcript JSON files to phoenix-feed."""

from __future__ import annotations

import dataclasses
import json
import os
import re
import sqlite3
import sys
import time
from datetime import datetime, timezone
from pathlib import Path
from typing import Any
from zoneinfo import ZoneInfo, ZoneInfoNotFoundError

import requests


DEFAULT_PHOENIX_FEED_URL = "https://feed.cactuswatch.com"
DEFAULT_RECORDINGS_DIR = r"D:\cactus\recordings"
DEFAULT_STATE_DB = r"D:\cactus\state\uploaded_transcripts.sqlite3"
DEFAULT_LOG_PATH = r"C:\cactus\logs\uploader_inscript.log"
DEFAULT_STDOUT_REDIRECT_PATH = Path(r"C:\cactus\logs\uploader.stdout")
DEFAULT_STDERR_REDIRECT_PATH = Path(r"C:\cactus\logs\uploader.stderr")
DEFAULT_CAPTURE_TIMEZONE = "America/Phoenix"
DEFAULT_POLL_SECONDS = 5.0
DEFAULT_BATCH_SIZE = 50
CAPTURE_RE = re.compile(r"^(\d{8}_\d{6})_")


@dataclasses.dataclass(frozen=True)
class Config:
    phoenix_feed_url: str
    admin_token: str
    recordings_dir: Path
    state_db: Path
    log_path: Path
    poll_seconds: float
    batch_size: int


def config_from_env() -> Config:
    admin_token = os.environ.get("ADMIN_TOKEN", "").strip()
    if not admin_token:
        raise ValueError("ADMIN_TOKEN is required")

    return Config(
        phoenix_feed_url=os.environ.get("PHOENIX_FEED_URL", DEFAULT_PHOENIX_FEED_URL).rstrip("/"),
        admin_token=admin_token,
        recordings_dir=Path(os.environ.get("CACTUS_RECORDINGS_DIR", DEFAULT_RECORDINGS_DIR)),
        state_db=Path(os.environ.get("CACTUS_UPLOADER_STATE_DB", DEFAULT_STATE_DB)),
        log_path=Path(os.environ.get("CACTUS_UPLOADER_LOG", DEFAULT_LOG_PATH)),
        poll_seconds=float(os.environ.get("CACTUS_UPLOADER_POLL_SECONDS", str(DEFAULT_POLL_SECONDS))),
        batch_size=int(os.environ.get("CACTUS_UPLOADER_BATCH_SIZE", str(DEFAULT_BATCH_SIZE))),
    )


def init_state_db(db_path: Path) -> None:
    db_path.parent.mkdir(parents=True, exist_ok=True)
    with sqlite3.connect(db_path) as conn:
        conn.execute(
            """
            CREATE TABLE IF NOT EXISTS uploaded_transcripts (
                wav_filename TEXT PRIMARY KEY,
                uploaded_at TEXT NOT NULL,
                permanent_failure_at TEXT,
                permanent_failure_status INTEGER,
                permanent_failure_error TEXT
            )
            """
        )
        existing_columns = {row[1] for row in conn.execute("PRAGMA table_info(uploaded_transcripts)")}
        for column, ddl in {
            "permanent_failure_at": "ALTER TABLE uploaded_transcripts ADD COLUMN permanent_failure_at TEXT",
            "permanent_failure_status": "ALTER TABLE uploaded_transcripts ADD COLUMN permanent_failure_status INTEGER",
            "permanent_failure_error": "ALTER TABLE uploaded_transcripts ADD COLUMN permanent_failure_error TEXT",
        }.items():
            if column not in existing_columns:
                conn.execute(ddl)
        conn.commit()


def record_uploaded(db_path: Path, wav_filename: str) -> None:
    now = utc_now()
    with sqlite3.connect(db_path) as conn:
        conn.execute(
            """
            INSERT INTO uploaded_transcripts (
                wav_filename, uploaded_at, permanent_failure_at,
                permanent_failure_status, permanent_failure_error
            ) VALUES (?, ?, NULL, NULL, NULL)
            ON CONFLICT(wav_filename) DO UPDATE SET
                uploaded_at = excluded.uploaded_at,
                permanent_failure_at = NULL,
                permanent_failure_status = NULL,
                permanent_failure_error = NULL
            """,
            (wav_filename, now),
        )
        conn.commit()


def record_permanent_failure(db_path: Path, wav_filename: str, status_code: int, message: str) -> None:
    now = utc_now()
    with sqlite3.connect(db_path) as conn:
        conn.execute(
            """
            INSERT INTO uploaded_transcripts (
                wav_filename, uploaded_at, permanent_failure_at,
                permanent_failure_status, permanent_failure_error
            ) VALUES (?, ?, ?, ?, ?)
            ON CONFLICT(wav_filename) DO UPDATE SET
                uploaded_at = excluded.uploaded_at,
                permanent_failure_at = excluded.permanent_failure_at,
                permanent_failure_status = excluded.permanent_failure_status,
                permanent_failure_error = excluded.permanent_failure_error
            """,
            (wav_filename, now, now, status_code, message[:500]),
        )
        conn.commit()


def already_recorded(db_path: Path, wav_filename: str) -> bool:
    with sqlite3.connect(db_path) as conn:
        row = conn.execute(
            "SELECT 1 FROM uploaded_transcripts WHERE wav_filename = ?",
            (wav_filename,),
        ).fetchone()
    return row is not None


def process_once(cfg: Config) -> int:
    init_state_db(cfg.state_db)
    cfg.recordings_dir.mkdir(parents=True, exist_ok=True)
    uploaded_count = 0
    attempted_count = 0

    for transcript_path in sorted(cfg.recordings_dir.glob("*.transcript.json")):
        if attempted_count >= cfg.batch_size:
            break
        wav_filename = wav_filename_from_path(transcript_path)
        if already_recorded(cfg.state_db, wav_filename):
            continue

        attempted_count += 1
        try:
            if upload_transcript(cfg, transcript_path, wav_filename):
                uploaded_count += 1
        except Exception as exc:  # noqa: BLE001 - keep the watcher alive per file.
            log_status(cfg, wav_filename, "error", f"exception={exc}", stderr=True)

    return uploaded_count


def upload_transcript(cfg: Config, transcript_path: Path, fallback_wav_filename: str) -> bool:
    try:
        payload = json.loads(transcript_path.read_text(encoding="utf-8"))
    except Exception as exc:  # noqa: BLE001
        record_permanent_failure(cfg.state_db, fallback_wav_filename, 0, f"local json error: {exc}")
        log_status(cfg, fallback_wav_filename, "permanent_failure", f"local_json_error={exc}", stderr=True)
        return False

    wav_filename = payload.get("wav") if isinstance(payload.get("wav"), str) and payload.get("wav").strip() else fallback_wav_filename
    wav_filename = wav_filename.strip()
    payload["wav_filename"] = wav_filename
    payload["captured_at"] = captured_at_from_filename(transcript_path.name)

    url = f"{cfg.phoenix_feed_url}/v1/admin/dispatch/transcript"
    try:
        response = requests.post(
            url,
            headers={"Authorization": f"Bearer {cfg.admin_token}", "Content-Type": "application/json"},
            json=payload,
            timeout=15,
        )
    except requests.RequestException as exc:
        log_status(cfg, wav_filename, "retry", f"network_error={exc}", stderr=True)
        return False

    if response.status_code == 200:
        duplicate = False
        try:
            duplicate = bool(response.json().get("duplicate"))
        except ValueError:
            duplicate = False
        record_uploaded(cfg.state_db, wav_filename)
        log_status(cfg, wav_filename, "duplicate" if duplicate else "uploaded", f"status=200")
        return True

    if response.status_code == 409:
        record_uploaded(cfg.state_db, wav_filename)
        log_status(cfg, wav_filename, "duplicate", "status=409")
        return True

    if response.status_code in (401, 429) or response.status_code >= 500:
        log_status(cfg, wav_filename, "retry", f"status={response.status_code} body={response.text[:300]}", stderr=True)
        return False

    if 400 <= response.status_code < 500:
        record_permanent_failure(cfg.state_db, wav_filename, response.status_code, response.text)
        log_status(cfg, wav_filename, "permanent_failure", f"status={response.status_code} body={response.text[:300]}", stderr=True)
        return False

    log_status(cfg, wav_filename, "retry", f"unexpected_status={response.status_code}", stderr=True)
    return False


def wav_filename_from_path(path: Path) -> str:
    name = path.name
    if name.endswith(".transcript.json"):
        return f"{name.removesuffix('.transcript.json')}.wav"
    return f"{path.stem}.wav"


def captured_at_from_filename(filename: str, box_tz: str = DEFAULT_CAPTURE_TIMEZONE) -> str:
    match = CAPTURE_RE.match(filename)
    if not match:
        raise ValueError(f"filename does not start with YYYYMMDD_HHMMSS_: {filename}")
    local_naive = datetime.strptime(match.group(1), "%Y%m%d_%H%M%S")
    try:
        local_time = local_naive.replace(tzinfo=ZoneInfo(box_tz))
    except ZoneInfoNotFoundError as exc:
        raise RuntimeError(
            f"timezone data for {box_tz} is unavailable; install tzdata with "
            "python -m pip install -r requirements.txt"
        ) from exc
    return local_time.astimezone(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def run_startup_self_checks(cfg: Config) -> bool:
    ok = True
    redirect_targets = (DEFAULT_STDOUT_REDIRECT_PATH, DEFAULT_STDERR_REDIRECT_PATH)
    for target in redirect_targets:
        if same_path(cfg.log_path, target):
            print(
                "cactus_uploader warning: CACTUS_UPLOADER_LOG points at "
                f"{target}, which run_uploader.bat also opens for redirection; "
                "use a separate file such as C:\\cactus\\logs\\uploader_inscript.log",
                file=sys.stderr,
            )
            break

    try:
        ZoneInfo(DEFAULT_CAPTURE_TIMEZONE)
    except ZoneInfoNotFoundError:
        print(
            "cactus_uploader timezone error: America/Phoenix is unavailable. "
            "Install tzdata in the uploader virtualenv with "
            "python -m pip install -r requirements.txt",
            file=sys.stderr,
        )
        ok = False

    return ok


def same_path(left: Path, right: Path) -> bool:
    return os.path.normcase(os.path.normpath(str(left))) == os.path.normcase(os.path.normpath(str(right)))


def log_status(cfg: Config, wav_filename: str, status: str, message: str = "", stderr: bool = False) -> None:
    cfg.log_path.parent.mkdir(parents=True, exist_ok=True)
    line = f"{utc_now()} {wav_filename} {status}"
    if message:
        line = f"{line} {message}"
    line = f"{line}\n"
    with cfg.log_path.open("a", encoding="utf-8") as log_file:
        log_file.write(line)
    if stderr:
        print(line, file=sys.stderr, end="")


def utc_now() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def main() -> int:
    try:
        cfg = config_from_env()
    except Exception as exc:  # noqa: BLE001
        print(f"cactus_uploader config error: {exc}", file=sys.stderr)
        return 2
    if not run_startup_self_checks(cfg):
        return 2

    while True:
        try:
            process_once(cfg)
        except Exception as exc:  # noqa: BLE001
            log_status(cfg, "-", "error", f"loop_exception={exc}", stderr=True)
        time.sleep(cfg.poll_seconds)


if __name__ == "__main__":
    raise SystemExit(main())

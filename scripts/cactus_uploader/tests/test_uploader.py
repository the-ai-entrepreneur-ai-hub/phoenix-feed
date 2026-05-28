import json
import sqlite3
import sys
from pathlib import Path
from zoneinfo import ZoneInfoNotFoundError


sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import cactus_uploader


def write_transcript(recordings_dir, stem="20260528_001043_unknown", wav=None):
    wav = wav or f"{stem}.wav"
    path = recordings_dir / f"{stem}.transcript.json"
    path.write_text(
        json.dumps(
            {
                "wav": wav,
                "audio_duration_s": 5.3,
                "display_text": "Engine 35 respond to a house fire.",
                "primary": {"model": "medium.en", "text": "Engine 35 respond to a house fire."},
                "verification": {"confidence": 0.91},
            }
        ),
        encoding="utf-8",
    )
    return path


def make_config(tmp_path, recordings_dir, requests_mock):
    return cactus_uploader.Config(
        phoenix_feed_url="https://feed.example.test",
        admin_token="secret-token",
        recordings_dir=recordings_dir,
        state_db=tmp_path / "uploaded_transcripts.sqlite3",
        log_path=tmp_path / "uploader.stdout",
        poll_seconds=0.01,
        batch_size=50,
    )


def status_for(db_path, wav_filename):
    with sqlite3.connect(db_path) as conn:
        return conn.execute(
            """
            SELECT uploaded_at, permanent_failure_at, permanent_failure_status
            FROM uploaded_transcripts
            WHERE wav_filename = ?
            """,
            (wav_filename,),
        ).fetchone()


def test_process_once_skips_files_already_in_sqlite(tmp_path, requests_mock):
    recordings_dir = tmp_path / "recordings"
    recordings_dir.mkdir()
    write_transcript(recordings_dir)
    cfg = make_config(tmp_path, recordings_dir, requests_mock)
    cactus_uploader.init_state_db(cfg.state_db)
    cactus_uploader.record_uploaded(cfg.state_db, "20260528_001043_unknown.wav")

    processed = cactus_uploader.process_once(cfg)

    assert processed == 0
    assert not requests_mock.called


def test_captured_at_from_filename_converts_phoenix_time_to_utc():
    captured_at = cactus_uploader.captured_at_from_filename("20260528_001043_unknown.transcript.json")

    assert captured_at == "2026-05-28T07:10:43Z"


def test_process_once_retries_5xx_without_marking_file(tmp_path, requests_mock):
    recordings_dir = tmp_path / "recordings"
    recordings_dir.mkdir()
    write_transcript(recordings_dir)
    cfg = make_config(tmp_path, recordings_dir, requests_mock)
    requests_mock.post("https://feed.example.test/v1/admin/dispatch/transcript", status_code=503, json={"error": "unavailable"})

    processed = cactus_uploader.process_once(cfg)

    assert processed == 0
    assert status_for(cfg.state_db, "20260528_001043_unknown.wav") is None


def test_process_once_marks_400_as_permanent_failure(tmp_path, requests_mock):
    recordings_dir = tmp_path / "recordings"
    recordings_dir.mkdir()
    write_transcript(recordings_dir)
    cfg = make_config(tmp_path, recordings_dir, requests_mock)
    requests_mock.post("https://feed.example.test/v1/admin/dispatch/transcript", status_code=400, json={"error": "bad transcript"})

    processed = cactus_uploader.process_once(cfg)

    assert processed == 0
    uploaded_at, permanent_failure_at, permanent_failure_status = status_for(cfg.state_db, "20260528_001043_unknown.wav")
    assert uploaded_at is not None
    assert permanent_failure_at is not None
    assert permanent_failure_status == 400


def test_process_once_records_duplicate_responses_locally(tmp_path, requests_mock):
    recordings_dir = tmp_path / "recordings"
    recordings_dir.mkdir()
    write_transcript(recordings_dir)
    cfg = make_config(tmp_path, recordings_dir, requests_mock)
    requests_mock.post(
        "https://feed.example.test/v1/admin/dispatch/transcript",
        status_code=200,
        json={"id": 123, "duplicate": True},
    )

    processed = cactus_uploader.process_once(cfg)

    assert processed == 1
    uploaded_at, permanent_failure_at, permanent_failure_status = status_for(cfg.state_db, "20260528_001043_unknown.wav")
    assert uploaded_at is not None
    assert permanent_failure_at is None
    assert permanent_failure_status is None


def test_default_log_path_does_not_collide_with_batch_stdout(monkeypatch):
    monkeypatch.setenv("ADMIN_TOKEN", "secret-token")
    monkeypatch.delenv("CACTUS_UPLOADER_LOG", raising=False)

    cfg = cactus_uploader.config_from_env()

    assert str(cfg.log_path) == r"C:\cactus\logs\uploader_inscript.log"
    assert cfg.log_path != cactus_uploader.DEFAULT_STDOUT_REDIRECT_PATH


def test_startup_self_check_warns_when_log_path_matches_stdout_redirect(tmp_path, capsys):
    cfg = cactus_uploader.Config(
        phoenix_feed_url="https://feed.example.test",
        admin_token="secret-token",
        recordings_dir=tmp_path / "recordings",
        state_db=tmp_path / "uploaded_transcripts.sqlite3",
        log_path=cactus_uploader.DEFAULT_STDOUT_REDIRECT_PATH,
        poll_seconds=0.01,
        batch_size=50,
    )

    ok = cactus_uploader.run_startup_self_checks(cfg)

    captured = capsys.readouterr()
    assert ok is True
    assert "CACTUS_UPLOADER_LOG" in captured.err
    assert "uploader.stdout" in captured.err


def test_startup_self_check_reports_missing_tzdata(monkeypatch, tmp_path, capsys):
    def missing_zoneinfo(_name):
        raise ZoneInfoNotFoundError("missing tzdata")

    monkeypatch.setattr(cactus_uploader, "ZoneInfo", missing_zoneinfo)
    cfg = cactus_uploader.Config(
        phoenix_feed_url="https://feed.example.test",
        admin_token="secret-token",
        recordings_dir=tmp_path / "recordings",
        state_db=tmp_path / "uploaded_transcripts.sqlite3",
        log_path=tmp_path / "uploader_inscript.log",
        poll_seconds=0.01,
        batch_size=50,
    )

    ok = cactus_uploader.run_startup_self_checks(cfg)

    captured = capsys.readouterr()
    assert ok is False
    assert "tzdata" in captured.err
    assert "python -m pip install -r requirements.txt" in captured.err

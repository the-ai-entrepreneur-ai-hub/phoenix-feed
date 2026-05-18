# Cactus Watch v1 Polish — Codex Progress Log

This file is the live audit trail for the work in `codex-cactus-watch-v1-polish.md`. Codex appends a row after every committed task and pushes immediately so George can `git pull` and watch progress in real time.

**Brief:** `codex-cactus-watch-v1-polish.md`
**Started:** _to be filled by Codex on first append_
**Last update:** _to be filled by Codex on every append_

---

## Status legend

- `STARTED` — task picked up, work in progress
- `DONE` — task finished and verified locally (tests green, openspec validate clean for spec tasks)
- `BLOCKED` — needs human decision; specify the question in notes
- `REVERTED` — task was started then rolled back; specify reason

## Conventions

- Times in UTC, ISO 8601 (`YYYY-MM-DDTHH:MM:SSZ`).
- One row per status transition. If a task goes STARTED then DONE, that is two rows.
- `commit` is the short SHA (7 chars) of the commit that made the change, or `-` if the row is STARTED without commit yet.
- `notes` stays under 80 characters. Use it for blockers, surprise findings, deviations from the brief.
- Append rows in chronological order at the bottom of the table below.
- After appending, run `git add docs/codex-cactus-watch-v1-polish-PROGRESS.md && git commit -m "chore(progress): <task short name>" && git push` so George sees it immediately.

---

## Task ledger

| UTC time | change-id | task | status | commit | notes |
|---|---|---|---|---|---|
| _example_ | _cactus-watch-landing-azincidents-mirror_ | _spec scaffolding_ | _STARTED_ | _-_ | _drafting proposal.md_ |

(remove the example row when you write the first real entry)

---

## Open questions for George

If Codex needs a decision, log the question here in numbered list form. George will answer inline by editing the file. Codex polls this section on every PROGRESS append and resumes blocked tasks once an answer is present.

1. _(none yet)_

---

## Final summary

When ALL tasks across both changes hit DONE, Codex pastes the verification output block from the brief here, then commits one final time with message `chore(progress): final summary, ready for George review`.

_(populated by Codex at end of run)_

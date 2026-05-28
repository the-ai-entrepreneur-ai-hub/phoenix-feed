## Overview

This change tightens the live SDR parser without widening the rest of the Phase 2 promotion gate. The confidence threshold remains `>= 0.80`, the fire/EMS unit matcher remains unchanged, the Phoenix-style address matcher remains unchanged, and promoted rows continue to flow through the existing `incidents` table and active endpoint.

## CDEC Marker Variants

The marker regex changes from literal `CDEC` only to:

```regex
(?i)\b(?:CDEC|Sea[\s-]?Deck|Sea[\s-]?Beck|Seabex|FedEx|CDC)[\s-]?(\d+)\b
```

The capture group is the channel number. The parser stores the normalized channel as `CDEC <number>` regardless of whether the transcript said `CDEC`, `Sea Deck`, `Sea Beck`, `Seabex`, `FedEx`, or `CDC`.

## Nature Sanitization

Nature extraction still looks only at the text before the first detected address and starts after the final accepted CDEC-like marker before that address. Instead of returning arbitrary text up to punctuation, it searches a curated dispatch nature list and returns the first matched canonical nature, with longer matches winning at the same position.

The curated list includes the user-supplied Phoenix nature set plus natures already covered by existing parser fixtures:

- Overdose
- Fall
- Cardiac Problem
- Cardiac Problems
- Difficulty Breathing
- Seizure
- Residential Fire Alarm
- Commercial Fire Alarm
- Vehicle Crash
- Vehicle Accident
- Structure Fire
- House Fire
- Brush Fire
- Trash Fire
- Working Fire
- Smoke Investigation
- Hazmat
- Level of Consciousness
- Welfare Check
- Check Welfare
- Sick Person
- Unknown Trouble
- Gas Leak
- Medical Assignment
- Breathing Problems
- Assault

If a non-empty candidate has no known nature match, the parser returns `Dispatch Call`. Empty candidates still fail as missing nature.

## Legacy Data Cleanup

Rows already promoted with over-captured nature text are cleaned by a data-only migration:

```sql
UPDATE incidents
SET nature_desc = TRIM(SPLIT_PART(nature_desc, ',', 1))
WHERE source = 'sdr_audio'
  AND LENGTH(nature_desc) > 50
  AND nature_desc LIKE '%,%';
```

This deliberately targets only scanner-derived rows with comma-delimited long nature text. It does not touch mapserver rows or long SDR nature text without commas.

## Non-Goals

- No change to the active endpoint query.
- No change to mapserver ingestion or Cactus app code.
- No deploy trigger.
- No geocoding, deduplication, or source-badge changes.

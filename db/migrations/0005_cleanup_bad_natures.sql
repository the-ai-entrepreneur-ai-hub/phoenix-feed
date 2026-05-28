UPDATE incidents
SET nature_desc = TRIM(SPLIT_PART(nature_desc, ',', 1))
WHERE source = 'sdr_audio'
  AND LENGTH(nature_desc) > 50
  AND nature_desc LIKE '%,%';

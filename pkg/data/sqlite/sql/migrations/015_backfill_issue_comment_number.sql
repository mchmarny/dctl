-- Backfill issue number from URL for existing issue_comment events.
-- URL pattern: https://github.com/org/repo/issues/123#issuecomment-456
UPDATE event
SET number = CAST(
    SUBSTR(
        url,
        INSTR(url, '/issues/') + 8,
        CASE
            WHEN INSTR(SUBSTR(url, INSTR(url, '/issues/') + 8), '#') > 0
            THEN INSTR(SUBSTR(url, INSTR(url, '/issues/') + 8), '#') - 1
            ELSE LENGTH(url)
        END
    ) AS INTEGER
)
WHERE type = 'issue_comment'
  AND number IS NULL
  AND url LIKE '%/issues/%';

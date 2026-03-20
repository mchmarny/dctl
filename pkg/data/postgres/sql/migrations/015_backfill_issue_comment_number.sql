-- Backfill issue number from URL for existing issue_comment events.
UPDATE event
SET number = CAST(
    SUBSTRING(url FROM '/issues/([0-9]+)') AS INTEGER
)
WHERE type = 'issue_comment'
  AND number IS NULL
  AND url LIKE '%/issues/%';

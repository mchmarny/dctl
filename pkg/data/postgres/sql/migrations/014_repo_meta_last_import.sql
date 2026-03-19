ALTER TABLE repo_meta ADD COLUMN IF NOT EXISTS last_import_at TEXT NOT NULL DEFAULT '';
UPDATE repo_meta SET last_import_at = updated_at WHERE last_import_at = '';

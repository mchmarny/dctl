ALTER TABLE repo_meta ADD COLUMN last_import_at TEXT NOT NULL DEFAULT '';
UPDATE repo_meta SET last_import_at = updated_at;

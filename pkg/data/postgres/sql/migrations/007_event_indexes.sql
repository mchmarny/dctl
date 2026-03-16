CREATE INDEX IF NOT EXISTS idx_event_org_repo_date ON event (org, repo, date);
CREATE INDEX IF NOT EXISTS idx_event_org_repo_type_date ON event (org, repo, type, date);
CREATE INDEX IF NOT EXISTS idx_event_org_repo_created_at ON event (org, repo, created_at);
CREATE INDEX IF NOT EXISTS idx_event_username ON event (username);

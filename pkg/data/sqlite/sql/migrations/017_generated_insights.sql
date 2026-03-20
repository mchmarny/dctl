CREATE TABLE repo_insights (
    org TEXT NOT NULL,
    repo TEXT NOT NULL,
    insights_json TEXT NOT NULL,
    period_months INTEGER NOT NULL DEFAULT 3,
    model TEXT NOT NULL DEFAULT '',
    generated_at TEXT NOT NULL,
    PRIMARY KEY (org, repo)
);

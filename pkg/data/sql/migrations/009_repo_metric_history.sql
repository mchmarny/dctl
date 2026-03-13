CREATE TABLE IF NOT EXISTS repo_metric_history (
    org TEXT NOT NULL,
    repo TEXT NOT NULL,
    date TEXT NOT NULL,
    stars INTEGER NOT NULL DEFAULT 0,
    forks INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (org, repo, date)
);

CREATE TABLE IF NOT EXISTS repo_meta (
    org TEXT NOT NULL,
    repo TEXT NOT NULL,
    stars INTEGER NOT NULL DEFAULT 0,
    forks INTEGER NOT NULL DEFAULT 0,
    open_issues INTEGER NOT NULL DEFAULT 0,
    language TEXT,
    license TEXT,
    archived INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (org, repo)
);

CREATE TABLE IF NOT EXISTS release (
    org TEXT NOT NULL,
    repo TEXT NOT NULL,
    tag TEXT NOT NULL,
    name TEXT,
    published_at TEXT,
    prerelease INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (org, repo, tag)
);

CREATE TABLE IF NOT EXISTS container_version (
    org TEXT NOT NULL,
    repo TEXT NOT NULL,
    package TEXT NOT NULL,
    version_id INTEGER NOT NULL,
    tag TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    PRIMARY KEY (org, repo, package, version_id)
);

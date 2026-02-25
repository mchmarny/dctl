CREATE TABLE IF NOT EXISTS release_asset (
    org TEXT NOT NULL,
    repo TEXT NOT NULL,
    tag TEXT NOT NULL,
    name TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT '',
    size INTEGER NOT NULL DEFAULT 0,
    download_count INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (org, repo, tag, name)
);

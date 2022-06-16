CREATE TABLE IF NOT EXISTS developer (
    username TEXT NOT NULL,
    id INTEGER NOT NULL,
    full_name TEXT NOT NULL,
    email TEXT,
    avatar TEXT,
    url TEXT,
    entity TEXT,
    PRIMARY KEY (username)
);

CREATE UNIQUE INDEX idx_developer ON developer (id);

CREATE TABLE IF NOT EXISTS event (
    id INTEGER NOT NULL, 
    type TEXT NOT NULL,
    time INTEGER NOT NULL, 
    org TEXT NOT NULL,
    repo TEXT NOT NULL,
    username TEXT NOT NULL,
    date TEXT NOT NULL,
    url TEXT NOT NULL,
    mentions TEXT NOT NULL,
    labels TEXT NOT NULL,
    PRIMARY KEY (id, type, time),
    FOREIGN KEY(username) REFERENCES developer(username) ON DELETE CASCADE
);

CREATE INDEX idx_event ON event (org, repo, type, username, date);

CREATE TABLE IF NOT EXISTS state (
    query TEXT NOT NULL,
    org TEXT NOT NULL,
    repo TEXT NOT NULL,
    page INTEGER NOT NULL, 
    since INTEGER NOT NULL, 
    PRIMARY KEY (query, org, repo)
);

CREATE TABLE IF NOT EXISTS sub (
    type TEXT NOT NULL,
    old TEXT NOT NULL,
    new TEXT NOT NULL,
    PRIMARY KEY (type, old)
);
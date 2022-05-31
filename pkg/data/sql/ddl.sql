CREATE TABLE IF NOT EXISTS developer (
    username TEXT NOT NULL,
    update_date TEXT NOT NULL,
    id INTEGER NOT NULL,
    full_name TEXT NOT NULL,
    email TEXT,
    avatar_url TEXT,
    profile_url TEXT,
    entity TEXT,
    location TEXT,
    PRIMARY KEY (username)
);

CREATE TABLE IF NOT EXISTS event (
    id INTEGER NOT NULL, 
    org TEXT NOT NULL,
    repo TEXT NOT NULL,
    username TEXT NOT NULL,
    event_type TEXT NOT NULL,
    event_date TEXT NOT NULL,
    event_url TEXT NOT NULL,
    mentions TEXT NOT NULL,
    labels TEXT NOT NULL,
    PRIMARY KEY (id, org, repo, username, event_type, event_date),
    FOREIGN KEY(username) REFERENCES developer(username) ON DELETE CASCADE
);

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
CREATE TABLE
    IF NOT EXISTS vault (
        id TEXT PRIMARY KEY,
        key TEXT NOT NULL,
        secret TEXT NOT NULL
    );

CREATE TABLE
    IF NOT EXISTS master_key (
        id INTEGER PRIMARY KEY CHECK (id = 0),
        key TEXT NOT NULL
    );
CREATE TABLE
    IF NOT EXISTS master_key (
        id INTEGER PRIMARY KEY CHECK (id = 0),
        key TEXT NOT NULL
    );

CREATE TABLE
    IF NOT EXISTS vault (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL UNIQUE,
        secret TEXT NOT NULL
    );
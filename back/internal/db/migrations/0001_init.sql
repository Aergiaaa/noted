-- 0001_init.sql - full MVP schema (DATABASE.md).
-- Pragmas (WAL, busy_timeout, foreign_keys, synchronous) live in the
-- connection DSN (internal/db/db.go), not here: sqlc parses this file.

CREATE TABLE notes (
    id         TEXT PRIMARY KEY,
    title      TEXT NOT NULL,
    body_md    TEXT NOT NULL,
    pinned     INTEGER NOT NULL DEFAULT 0 CHECK (pinned IN (0, 1)),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE events (
    id          TEXT PRIMARY KEY,
    title       TEXT NOT NULL,
    starts_at   TEXT NOT NULL,
    ends_at     TEXT NOT NULL,
    description TEXT,
    note_id     TEXT REFERENCES notes (id) ON DELETE SET NULL,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE tasks (
    id           TEXT PRIMARY KEY,
    title        TEXT NOT NULL,
    status       TEXT NOT NULL CHECK (status IN ('open', 'done')),
    due_date     TEXT,
    event_id     TEXT REFERENCES events (id) ON DELETE SET NULL,
    note_id      TEXT REFERENCES notes (id) ON DELETE SET NULL,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    completed_at TEXT
);

CREATE TABLE transactions (
    id           TEXT PRIMARY KEY,
    kind         TEXT NOT NULL CHECK (kind IN ('income', 'expense')),
    amount_cents INTEGER NOT NULL CHECK (amount_cents > 0),
    currency     TEXT NOT NULL,
    occurred_on  TEXT NOT NULL,
    memo         TEXT,
    task_id      TEXT REFERENCES tasks (id) ON DELETE SET NULL,
    event_id     TEXT REFERENCES events (id) ON DELETE SET NULL,
    note_id      TEXT REFERENCES notes (id) ON DELETE SET NULL,
    created_at   TEXT NOT NULL
);

CREATE TABLE sessions (
    id         TEXT PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    last_seen  TEXT NOT NULL
);

CREATE TABLE recovery_codes (
    id         TEXT PRIMARY KEY,
    code_hash  TEXT NOT NULL UNIQUE,
    used_at    TEXT,
    created_at TEXT NOT NULL
);

CREATE INDEX events_starts_at_idx ON events (starts_at);
CREATE INDEX notes_pinned_updated_at_idx ON notes (pinned, updated_at);
CREATE INDEX tasks_status_due_date_idx ON tasks (status, due_date);
CREATE INDEX tasks_event_id_idx ON tasks (event_id);
CREATE INDEX transactions_occurred_on_idx ON transactions (occurred_on);
CREATE INDEX transactions_task_id_idx ON transactions (task_id);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

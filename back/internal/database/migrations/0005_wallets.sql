-- 0005_wallets.sql - wallets + per-wallet transactions (F8, DOMAIN.md).
-- A transfer is one row (kind='transfer', from/to wallets); income and
-- expense rows carry wallet_id. The kind CHECK cannot be altered in
-- SQLite, so transactions is rebuilt (zero rows exist when this ships).
-- Service enforces the per-kind wallet exclusivity the schema cannot
-- express cheaply.

CREATE TABLE wallets (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    archived_at TEXT,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE INDEX wallets_archived_at_idx ON wallets (archived_at);

-- Seed wallet so the create form always has a picker target (F8).
INSERT INTO wallets (id, name, archived_at, created_at, updated_at)
VALUES ('wal_cash', 'Cash', NULL,
        strftime('%Y-%m-%dT%H:%M:%SZ', 'now'),
        strftime('%Y-%m-%dT%H:%M:%SZ', 'now'));

CREATE TABLE transactions_new (
    id             TEXT PRIMARY KEY,
    kind           TEXT NOT NULL CHECK (kind IN ('income', 'expense', 'transfer')),
    amount_cents   INTEGER NOT NULL CHECK (amount_cents > 0),
    currency       TEXT NOT NULL,
    occurred_on    TEXT NOT NULL,
    memo           TEXT,
    wallet_id      TEXT REFERENCES wallets (id) ON DELETE SET NULL,
    from_wallet_id TEXT REFERENCES wallets (id) ON DELETE SET NULL,
    to_wallet_id   TEXT REFERENCES wallets (id) ON DELETE SET NULL,
    task_id        TEXT REFERENCES tasks (id) ON DELETE SET NULL,
    event_id       TEXT REFERENCES events (id) ON DELETE SET NULL,
    note_id        TEXT REFERENCES notes (id) ON DELETE SET NULL,
    created_at     TEXT NOT NULL,
    deleted_at     TEXT
);

INSERT INTO transactions_new (id, kind, amount_cents, currency, occurred_on, memo,
                              wallet_id, from_wallet_id, to_wallet_id,
                              task_id, event_id, note_id, created_at, deleted_at)
SELECT id, kind, amount_cents, currency, occurred_on, memo,
       NULL, NULL, NULL,
       task_id, event_id, note_id, created_at, deleted_at
FROM transactions;

DROP TABLE transactions;
ALTER TABLE transactions_new RENAME TO transactions;

CREATE INDEX transactions_occurred_on_idx ON transactions (occurred_on);
CREATE INDEX transactions_task_id_idx ON transactions (task_id);
CREATE INDEX transactions_wallet_id_idx ON transactions (wallet_id);
CREATE INDEX transactions_deleted_at_idx ON transactions (deleted_at);

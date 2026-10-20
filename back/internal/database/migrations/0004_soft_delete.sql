-- 0004_soft_delete.sql - soft-delete columns for domain tables (DOMAIN.md).
-- Delete sets deleted_at; purge (retention window) is the only real DELETE.
-- Auth tables (sessions, recovery_codes, enrollment) stay hard-delete: a
-- retired credential must leave no row behind (SECURITY.md).

ALTER TABLE events ADD COLUMN deleted_at TEXT;
ALTER TABLE notes ADD COLUMN deleted_at TEXT;
ALTER TABLE tasks ADD COLUMN deleted_at TEXT;
ALTER TABLE transactions ADD COLUMN deleted_at TEXT;

-- Indexes back the purge range scan (deleted_at < cutoff).
CREATE INDEX events_deleted_at_idx ON events (deleted_at);
CREATE INDEX notes_deleted_at_idx ON notes (deleted_at);
CREATE INDEX tasks_deleted_at_idx ON tasks (deleted_at);
CREATE INDEX transactions_deleted_at_idx ON transactions (deleted_at);

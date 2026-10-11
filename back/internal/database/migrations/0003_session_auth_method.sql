-- Which credential minted the session: only a TOTP login may mint more
-- credentials, so POST /api/auth/recovery-codes refuses sessions created
-- from a recovery code (step-up, SECURITY.md). Rows that predate 0003
-- default to 'totp' so a deploy never locks the operator out of rotation.
-- ASCII only (sqlc gotcha, DATABASE.md).
ALTER TABLE sessions
    ADD COLUMN auth_method TEXT NOT NULL DEFAULT 'totp'
        CHECK (auth_method IN ('totp', 'recovery'));

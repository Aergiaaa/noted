-- F4: single-row enrollment state (id is pinned to 1 by the CHECK).
-- totp_secret holds the RFC4226 secret AES-GCM-sealed by the `server enroll`
-- CLI (base64 ciphertext, never plaintext); enrolled_at stays NULL until a
-- live code verifies, which is what lets `reset-auth` re-arm enrollment by
-- clearing this row. ASCII only (sqlc gotcha, DATABASE.md).
CREATE TABLE enrollment (
    id          INTEGER PRIMARY KEY CHECK (id = 1),
    totp_secret TEXT NOT NULL,
    enrolled_at TEXT,
    updated_at  TEXT NOT NULL
);

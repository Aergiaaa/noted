-- totp_secret holds the AES-GCM-sealed secret (base64 ciphertext, never
-- plaintext); enrolled_at stays NULL until a live code verifies.
-- ASCII only (sqlc gotcha, DATABASE.md).
CREATE TABLE enrollment (
    id          INTEGER PRIMARY KEY CHECK (id = 1),
    totp_secret TEXT NOT NULL,
    enrolled_at TEXT,
    updated_at  TEXT NOT NULL
);

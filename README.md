# Noted

A personal life-management app: Calendar, Notes/Tasks, Finance, and a Dashboard — all interconnected. Runs on your own hardware, single-user, TOTP-only auth.

## Quick start

```bash
# 1. Clone and configure
git clone https://github.com/Aergiaaa/noted.git
cd noted
cp .env.example .env
# Edit .env: set TOTP_ENC_KEY (generate: openssl rand -base64 32)
# Set APP_ORIGIN if not using the default dev setup

# 2. Start dev stack (API on :8080, Vite dev server on :5173)
make compose-up

# 3. Enroll your authenticator (one-time, host-local)
make enroll
# Scan the QR code with your TOTP app (Google Authenticator, 1Password, etc.)
# Enter the 6-digit code when prompted
# Save the 10 recovery codes shown — they're displayed once only

# 4. Open http://localhost:5173 and log in
```

## Production deploy

```bash
# Requires a domain with DNS pointing to the host
# Set in .env: APP_ENV=prod, APP_ORIGIN=https://your.domain, TOTP_ENC_KEY
docker compose -f docker-compose.yml --profile prod up -d

# Enroll on the server
docker compose run --rm -it api enroll
```

## Features

| Module | What it does |
|--------|--------------|
| **Calendar** | Week view (dates as rows, time as columns). Click empty slot → create event. Click event → details → Edit. Multi-day events render as strips. Covered hours show a "+" under the cursor for overlapping events. |
| **Notes** | Markdown notes with pinned toggle. `@[task:ID]`, `@[event:ID]`, `@[note:ID]` mentions in body — click to navigate. |
| **Tasks** | Title, due date, status (open/done), links to event + note. One-click complete sets `completed_at`. Filter by status and due date. |
| **Finance** | Multiple wallets (Cash, Bank, etc.). Income/expense per wallet. Transfers between wallets (single row, `from → to`). Derived balances — per wallet and total (transfers cancel out). History with cursor pagination. |
| **Dashboard** | Today at a glance: events, tasks due, overdue tasks, balance + recent transactions, pinned notes. Deep-links to each module. |

## Key workflows

### Linking things together
- Task → Event + Note: open TaskForm, pick from dropdowns (chips appear on the row)
- Transaction → Task / Event / Note: same pickers in TransactionForm
- Mentions: in Note body or Event description, click the mention button (or type `@[task:` etc.) → inserts token at cursor. Renders as clickable chip.

### Transfers
Finance page → "Transfer" button → pick from/to wallets, amount, date. Shows as one row "Cash → Bank" with distinct badge. Balances update instantly (derived on read).

### Keyboard / UX tips
- Calendar: `‹ ›` week nav (or arrow keys), month title flips at boundary
- Covered hour "+" appears only under cursor (touch/keyboard always shows it)
- Event block click → read-only card → Edit button opens form
- Modals: Esc closes, backdrop click closes, destructive actions need explicit confirm

## Backup & restore

```bash
# Dev (writes to ./.backups/noted-YYYY-MM-DD.db)
make backup

# Verify a backup
docker compose run --rm api backup --verify /backups/noted-2026-11-20.db

# Restore drill (boots real server on scratch copy, checks /healthz)
make restore-drill

# Prod: same CLI against the noted-backups volume
docker compose -f docker-compose.yml run --rm api backup
# Restore: stop api, swap /data/noted.db, start api
```

## Commands (Makefile)

| Command | Purpose |
|---------|---------|
| `make compose-up` / `make compose-down` | Dev stack (api + web) |
| `make enroll` / `make reset-auth` | TOTP enrollment / wipe & re-arm |
| `make test` / `make test-cover` | Unit + integration tests |
| `make format` | Prettier (run before commit) |
| `make sqlc` / `make migrate-check` | Regenerate queries / verify migrations |
| `make backup` / `make restore-drill` | Backup & restore drill |
| `make e2e` | Playwright critical flows against prod images |
| `make promote` | Push work branch → CI → auto-PR to staging |
| `make release` | Staging → main PR (manual, owner only) |

## Architecture snapshot

```
[Browser] --same-origin JSON--> [nginx] --/api--> [Go/Chi] --> [SQLite (WAL)]
                ^                                        |
                └────────── static files ────────────────┘
```

- Go: handler → service → sqlc-generated queries → SQLite
- Frontend: React + TS + Vite + Tailwind + DaisyUI (Bun runtime)
- Auth: TOTP (RFC 6238) + recovery codes, cookie sessions (HttpOnly, Secure, SameSite=Lax)
- Single currency (deployment constant `CURRENCY_DEFAULT`, default USD)
- Soft deletes (30-day purge), wallets archive-only

## Requirements

- Docker + Compose plugin
- Go ≥ 1.22, Bun ≥ 1.2 (for local frontend work)
- `sqlc`, `make` (for development)

## License

MIT — see [LICENSE](LICENSE).
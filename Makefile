.PHONY: test test-back test-front test-cover test-cover-back test-cover-front format promote release sqlc migrate-check compose-up compose-down backup restore-drill e2e enroll reset-auth

# Full test suite: backend (go test) + frontend (vitest).
test: test-back test-front

# CGO_ENABLED=0: static binaries; avoids NixOS mixed-channel glibc issues.
test-back:
	cd back && CGO_ENABLED=0 go test ./...

test-front:
	cd front && bunx --bun vitest run

# Coverage gates: both suites must hold 100% or the target fails.
# Backend excludes main.go's `func main` (thin boot entrypoint, only
# exercisable by running the real binary) and sqlc output under
# internal/database/generated (TESTING.md: generated code is never tested);
# every other function must be 100%.
# Frontend excludes src/main.tsx (createRoot entrypoint) and enforces
# 100% lines/functions/branches/statements via vitest thresholds.
test-cover: test-cover-back test-cover-front

test-cover-back:
	cd back && mkdir -p coverage && CGO_ENABLED=0 go test -count=1 -coverprofile=coverage/cover.out ./...
	cd back && CGO_ENABLED=0 go tool cover -func=coverage/cover.out | tee coverage/func.txt | \
		awk '$$1 == "total:" { next } $$1 ~ /generated\// { next } $$1 ~ /main\.go:/ && $$2 == "main" { next } $$3 != "100.0%" { bad=1; print "BELOW 100%: " $$0 } END { exit bad }'

test-cover-front:
	cd front && bunx --bun vitest run --coverage

# Format the frontend with Prettier defaults (FRONTEND.md: format before
# commit). CI runs the same check, so an unformatted tree fails the gate.
format:
	cd front && bun run format

# Local equivalent of the promote workflow: 100% gates, then push the
# current work branch (feat|fix|ci|chore|docs|refactor|test/*, see
# DEVELOPMENT.md) and wait for CI + the auto-promote PR to land on staging.
# staging is ruleset-protected like main (PR required, strict back/front
# checks, no bypass actors), so the old `checkout staging && merge` could
# never be pushed — the branch push *is* the promote. Safe to re-run: a
# branch already on staging exits early, an open staging PR is reused, and
# a merge refused for staleness updates the branch and retries once.
promote: test-cover
	@command -v gh >/dev/null 2>&1 || { echo "promote: gh CLI is required"; exit 1; }
	@branch=$$(git branch --show-current); \
	case "$$branch" in feat/*|fix/*|ci/*|chore/*|docs/*|refactor/*|test/*) ;; *) echo "promote only runs on work branches (feat|fix|ci|chore|docs|refactor|test)/* (on $$branch)"; exit 1;; esac; \
	git fetch origin staging || exit 1; \
	if git merge-base --is-ancestor HEAD origin/staging; then echo "promote: $$branch is already on staging"; exit 0; fi; \
	git push -u origin "$$branch" || exit 1; \
	sha=$$(git rev-parse HEAD); \
	echo "promote: waiting for CI on $$sha"; \
	run=""; n=0; \
	while [ -z "$$run" ] && [ $$n -lt 60 ]; do \
		run=$$(gh run list --workflow ci --commit "$$sha" --limit 1 --json databaseId --jq '.[0].databaseId // empty'); \
		n=$$((n + 1)); [ -n "$$run" ] || sleep 3; \
	done; \
	[ -n "$$run" ] || { echo "promote: no CI run appeared for $$sha"; exit 1; }; \
	gh run watch "$$run" --exit-status --interval 10 || { echo "promote: CI failed on $$branch"; exit 1; }; \
	pr=""; n=0; \
	while [ -z "$$pr" ] && [ $$n -lt 60 ]; do \
		pr=$$(gh pr list --head "$$branch" --base staging --state open --json number --jq '.[0].number // empty'); \
		n=$$((n + 1)); [ -n "$$pr" ] || sleep 3; \
	done; \
	[ -n "$$pr" ] || { echo "promote: auto-promote opened no staging PR for $$branch"; exit 1; }; \
	echo "promote: merging PR #$$pr"; \
	gh pr checks "$$pr" --watch || exit 1; \
	gh pr merge "$$pr" --merge || { \
		echo "promote: merge refused — updating $$branch from staging"; \
		gh pr update-branch "$$pr" || exit 1; \
		gh pr checks "$$pr" --watch || exit 1; \
		gh pr merge "$$pr" --merge || exit 1; \
	}; \
	git fetch origin staging:staging || exit 1; \
	echo "promote: PR #$$pr merged; local staging synced"

# Stable releases stay manual (DEVELOPMENT.md) and must go through a PR:
# the repo ruleset on main declines direct pushes (GH013 "Changes must be
# made through a pull request", no bypass actors). Staging gets main merged
# in first because the required status checks (back, front) run with strict
# up-to-date, so the PR head has to contain main. Reuses an open staging →
# main PR if one exists; idempotent to re-run if a step was interrupted.
release:
	@command -v gh >/dev/null 2>&1 || { echo "release: gh CLI is required"; exit 1; }
	git fetch origin main staging
	@dirty=$$(git status --porcelain); if [ -n "$$dirty" ]; then echo "release: working tree not clean:"; echo "$$dirty"; exit 1; fi
	@if [ "$$(git rev-list --count origin/main..origin/staging)" -eq 0 ]; then echo "release: staging has nothing that main lacks"; exit 1; fi
	git checkout staging && git pull --ff-only origin staging
	git merge origin/main -m "Sync main into staging before release" && git push origin staging
	git checkout main && git pull --ff-only origin main
	@pr=$$(gh pr list --base main --head staging --state open --json url --jq '.[0].url'); \
	[ -n "$$pr" ] || pr=$$(gh pr create --base main --head staging --title "Release staging to main (stable)" --body "Manual release of soaked staging (make release)."); \
	echo "release: waiting for checks on $$pr"; \
	gh pr checks "$$pr" --watch || exit 1; \
	gh pr merge "$$pr" --merge
	git pull --ff-only origin main
	@echo "release: main is at $$(git rev-parse --short main)"

# Regenerate sqlc queries (wired in F2).
sqlc:
	cd back && sqlc generate

# Verify migrations apply cleanly (wired in F2).
migrate-check:
	cd back && CGO_ENABLED=0 go test ./internal/database -run '^TestMigrate_freshDb_appliesCleanAndIsIdempotent$$' -count=1 -v

# UID/GID: bash and zsh never export their same-named readonly shell
# variables, so compose's "${UID:-1000}" interpolation always fell back
# to 1000 and containers chowned bind-mounted files to the wrong user on
# any host with a different uid. Export the real host ids here (direct
# `docker compose up` invocations still get the 1000 fallback).
compose-up: export UID := $(shell id -u)
compose-up: export GID := $(shell id -g)
compose-up:
	docker compose up --build -d

compose-down:
	docker compose down

# Nightly-style backup: VACUUM INTO ./.backups/noted-YYYY-MM-DD.db (bind
# mounted at /backups in the dev container), integrity-checked, 0600, 14
# kept. Run from the dev stack; prod uses the same subcommand against the
# noted-backups volume (DEPLOYMENT.md).
backup: export UID := $(shell id -u)
backup: export GID := $(shell id -g)
backup:
	mkdir -p .backups
	docker compose run --build --rm api run ./cmd/server backup

# Restore drill: copy the newest backup to a scratch DB, boot the real
# server against it, and require /healthz to answer. Never touches .db's
# live database.
restore-drill: export UID := $(shell id -u)
restore-drill: export GID := $(shell id -g)
restore-drill:
	@latest=$$(ls -1t .backups/noted-*.db 2>/dev/null | head -1); \
	[ -n "$$latest" ] || { echo "restore-drill: no backup yet — run make backup"; exit 1; }; \
	mkdir -p .db; \
	docker rm -f noted-restore-drill >/dev/null 2>&1 || true; \
	cp "$$latest" .db/restore-drill.db; \
	docker compose run --build --rm -d --name noted-restore-drill \
		-e DB_PATH=/data/restore-drill.db api run ./cmd/server >/dev/null; \
	i=0; \
	until docker exec noted-restore-drill wget -qO- http://localhost:8080/healthz >/dev/null 2>&1; do \
		i=$$((i+1)); \
		if [ $$i -ge 30 ]; then \
			docker logs noted-restore-drill || true; \
			docker rm -f noted-restore-drill >/dev/null; \
			rm -f .db/restore-drill.db*; \
			exit 1; \
		fi; \
		sleep 1; \
	done; \
	docker rm -f noted-restore-drill >/dev/null; \
	rm -f .db/restore-drill.db*; \
	echo "restore-drill: restored backup booted clean (healthz 200)"

# Playwright critical flows against the prod images. The isolated stack
# (docker-compose.e2e.yml) gets a fresh volume and a random encryption
# key each run; enrollment is seeded host-side with a known test secret
# and the browser mints the matching TOTP codes. Nothing here touches
# ./.db or the owner's enrollment.
E2E_PORT ?= 8088
E2E_TOTP_SECRET ?= JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP
# Prefer a distro Chromium when present (NixOS et al. lack the libraries
# Playwright's bundled build needs); otherwise the managed browser from
# `bunx playwright install --with-deps chromium` is used.
E2E_CHROMIUM_PATH ?= $(shell command -v chromium 2>/dev/null)

e2e: export E2E_PORT := $(E2E_PORT)
e2e: export E2E_TOTP_SECRET := $(E2E_TOTP_SECRET)
e2e: export E2E_CHROMIUM_PATH := $(E2E_CHROMIUM_PATH)
e2e: export E2E_TOTP_ENC_KEY := $(shell openssl rand -base64 32)
e2e:
	@set -eu; \
	compose="docker compose -f docker-compose.yml -f docker-compose.e2e.yml"; \
	teardown() { $$compose down -v --remove-orphans >/dev/null 2>&1 || true; }; \
	trap teardown EXIT INT TERM; \
	teardown; \
	$$compose up --build -d; \
	i=0; \
	until curl -fsS "http://localhost:$$E2E_PORT/healthz" >/dev/null 2>&1; do \
		i=$$((i+1)); \
		if [ $$i -ge 60 ]; then $$compose logs api; echo "e2e: stack never became healthy"; exit 1; fi; \
		sleep 2; \
	done; \
	$$compose run --rm -T api enroll --secret "$$E2E_TOTP_SECRET"; \
	( cd front && { [ -n "$$E2E_CHROMIUM_PATH" ] || bunx playwright install chromium; } && \
		E2E_BASE_URL="http://localhost:$$E2E_PORT" bunx playwright test )

# F4 enrollment, run inside the dev api container (entrypoint `go`, source
# bind-mounted at /src, DB_PATH=/data/noted.db hits the same ./.db file the
# running server uses). Prints the ASCII QR + otpauth URI, verifies one live
# 6-digit code, then persists the sealed secret + recovery pool once (shown
# once, store offline). Extra flags via ARGS, e.g.
# `make enroll ARGS='--qr /tmp/noted-qr.svg'`; `--secret BASE32` skips the
# live-code prompt for a known secret (E2E seeding, authenticator restore).
# UID/GID export: same reasoning as compose-up above — without it the
# container falls back to 1000:1000 and may write .db files the host
# user can't read or write.
enroll: export UID := $(shell id -u)
enroll: export GID := $(shell id -g)
enroll:
	mkdir -p .db
	docker compose run --build --rm -it api run ./cmd/server enroll $(ARGS)

# Wipes enrollment, recovery codes and sessions so the host can re-arm.
# --yes is passed deliberately: the CLI refuses a bare `reset-auth`.
reset-auth: export UID := $(shell id -u)
reset-auth: export GID := $(shell id -g)
reset-auth:
	mkdir -p .db
	docker compose run --build --rm -it api run ./cmd/server reset-auth --yes

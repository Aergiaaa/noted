.PHONY: dev test test-back test-front test-cover test-cover-back test-cover-front promote release sqlc migrate-check compose-up compose-down backup

# Local iteration: Go api :8080 + Vite :5173 (no containers needed).
# DB_PATH is absolute: the server runs from back/, but the SQLite file
# belongs in the repo-root .db/ this target creates (and .gitignore hides).
# Teardown walks each background job's whole process tree before killing
# the job: `go run` and `bun run dev` are wrappers, and killing only them
# orphans the server/vite children (which keep :8080/:5173 bound until
# killed by hand). Never `kill 0`: bash 5.3 (this system's /bin/sh)
# segfaults on a self group-kill inside a trap. The first job to end takes
# the whole target down with its status (`wait -n`, bash >= 4.3): a
# crashed server must not leave vite running in a hanging recipe.
dev:
	mkdir -p .db
	killtree() { for c in $$(pgrep -P $$1 2>/dev/null); do killtree $$c; done; kill -TERM $$1 2>/dev/null || true; }; \
	cleanup() { for j in $$(jobs -p); do killtree $$j; done; wait; }; \
	trap 'cleanup; exit 0' INT TERM; \
	trap cleanup EXIT; \
	(cd back && CGO_ENABLED=0 DB_PATH=$(CURDIR)/.db/noted.db go run ./cmd/server) & \
	(cd front && bun run dev) & \
	if [ -n "$${BASH_VERSION:-}" ]; then wait -n; else wait; fi; status=$$?; \
	cleanup; exit $$status

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
	docker compose up --build

compose-down:
	docker compose down

# Nightly-style backup drill target (wired in F12).
backup:
	@echo "TODO(F12): VACUUM INTO + integrity_check"

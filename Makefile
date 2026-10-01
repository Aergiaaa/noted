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

# Local equivalent of the promote workflow: merge the current work branch
# (feat|fix|ci|chore|docs|refactor|test/*, see DEVELOPMENT.md) into staging,
# but only after the 100% gates pass.
promote: test-cover
	@branch=$$(git branch --show-current); \
	case "$$branch" in feat/*|fix/*|ci/*|chore/*|docs/*|refactor/*|test/*) ;; *) echo "promote only runs on work branches (feat|fix|ci|chore|docs|refactor|test)/* (on $$branch)"; exit 1;; esac; \
	git checkout staging && git merge --no-ff "$$branch" -m "Promote $$branch to staging (gates green, 100% coverage)"

# Stable releases stay manual: merge soaked staging into main.
release:
	git checkout main && git merge --no-ff staging -m "Release staging to main (stable)"

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

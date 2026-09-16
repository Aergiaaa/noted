.PHONY: dev test test-back test-front test-cover test-cover-back test-cover-front promote release sqlc migrate-check compose-up compose-down backup

# Local iteration: Go api :8080 + Vite :5173 (no containers needed).
dev:
	mkdir -p .data
	trap 'kill 0' INT TERM EXIT; \
	(cd back && CGO_ENABLED=0 go run ./cmd/server) & \
	(cd front && bun run dev) & \
	wait

# Full test suite: backend (go test) + frontend (vitest).
test: test-back test-front

# CGO_ENABLED=0: static binaries; avoids NixOS mixed-channel glibc issues.
test-back:
	cd back && CGO_ENABLED=0 go test ./...

test-front:
	cd front && bunx --bun vitest run

# Coverage gates: both suites must hold 100% or the target fails.
# Backend excludes main.go's `func main` (thin boot entrypoint, only
# exercisable by running the real binary); every other function must be 100%.
# Frontend excludes src/main.tsx (createRoot entrypoint) and enforces
# 100% lines/functions/branches/statements via vitest thresholds.
test-cover: test-cover-back test-cover-front

test-cover-back:
	cd back && mkdir -p coverage && CGO_ENABLED=0 go test -count=1 -coverprofile=coverage/cover.out ./...
	cd back && CGO_ENABLED=0 go tool cover -func=coverage/cover.out | tee coverage/func.txt | \
		awk '$$1 == "total:" { next } $$1 ~ /main\.go:/ && $$2 == "main" { next } $$3 != "100.0%" { bad=1; print "BELOW 100%: " $$0 } END { exit bad }'

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
	@echo "TODO(F2): run migrations against temp DB"

compose-up:
	docker compose up --build

compose-down:
	docker compose down

# Nightly-style backup drill target (wired in F12).
backup:
	@echo "TODO(F12): VACUUM INTO + integrity_check"

# AGENTS.md

Code written in this repo MUST match the existing style 1:1. Study the current
code before writing anything new. Rules below.

## Testing (test-first)

- Test first, then implement: write a failing unit test that pins the expected
  behavior before touching any code. Bug fixes follow the same rule — reproduce
  the bug in a test first, then fix until it passes.
- Creating something new (feature, handler, service method, helper) = create
  the test first, see it fail (`go test ./...` red), then implement until the
  suite is green.
- Tests live next to the code: `service/*_test.go`, `cmd/handler/*_test.go`,
  `ui/modules/*_test.go`. Stdlib only (`testing`, `httptest`, `errors`).
- Service tests: `errors.Is` against sentinel errors (`ErrBlockKindMismatch`,
  etc.) on `&XyzService{}` without models, so tests reach only pre-DB
  validation; shared fixtures in `service/utils_test.go` (e.g. `validUUID`).
- Handler tests: `httptest.NewRequest` + `httptest.NewRecorder` with a fake
  `XyzServicer` wired through `service.Services{...}` (see `fakePageServicer`);
  assert status codes, error paths return `http.StatusInternalServerError`.
- templ/components: test pure helpers only (`blockChildren`, `linkSegments`);
  no template rendering in unit tests.
- Gate before every commit: `go test ./...`, `go vet ./...`,
  `gofmt -l cmd service ui` clean, then build + live e2e verify before
  committing.

## Language / tooling

- Go 1.26, templ (a-h/templ v0.3), sqlc (pgx/v5), chi router, Tailwind v4,
  Alpine.js v3, taskfile (go-task), TOTP auth via gorilla sessions.
- Run `go-task dev` for templ + tailwind watch. sqlc output is generated, do not
  hand-edit `internal/database/*.sql.go` / `*_templ.go` / compiled `output.css`.

## Naming convention (entire codebase)

- UPPERCASE (PascalCase) = exported / used outside the directory scope.
  Example: `GetAllPages` query, `SideBar` templ component, `PageService`.
- lowercase = internal calculation / helper only used within the file's scope.
  Example: `getOrder`, `head` template, `pageQuery` variable.
- Follow whatever the adjacent existing code does — match it 1:1.

## Go — cmd/ (HTTP layer)

- Handlers are methods on `*handler.Handler` (package `cmd/handler/`, split per
  domain: pages.go, blocks.go, tags.go, finance.go, edges.go), wired on `App`
  as `handle *handler.Handler` and called via `a.handle.X`.
  `func (h *Handler) name(w http.ResponseWriter, r *http.Request)`.
- Router in `cmd/routes.go`: route groups with comments like `// public page`,
  `// authorized page`, `// block routes`, blank lines separating method groups.
- `handleNOP` is a skeleton stub: route shapes are declared first as NOP, then
  each gets rewritten into a real handler. Never leave a rewritten handler as NOP.
- Real handler shape: parse query params with `strconv.Atoi` + fallback default
  on error; on service error `http.Error(w, err.Error(), http.StatusInternalServerError)`
  then `return`; JSON responses as inline `type response struct` with json tags,
  encoded via `json.NewEncoder(w).Encode(res)`.
- Verbose query param names: `pageQuery := r.URL.Query().Get("page")`.

## Go — service/ (business layer)

- One file per domain. Pattern per domain:
  - `type XyzServicer interface` + `type XyzService struct { models *database.Queries }`
  - pointer-receiver methods `(p *XyzService)`; wire in `service.go` via `&XyzService{models: models}`
  - multi-arg methods take an args struct (e.g. `CreateBlockArgs`), single-arg take plain strings
- Validation before DB: sentinel errors in `service/utils.go` (`ErrEmptyTitle`, etc.),
  then `parseUUID` / `parseDate` / `parseNum`, then
  `params := database.XParams{...}` → `return p.models.X(ctx, params)`.
- Type constants are SCREAMING_CASE: `BLOCK_TEXT`, `PAGE`, `TRANSACTION`.
- Enum validation via `switch Type(v) { case ...: default: return zero, ErrX }`,
  never via DB CHECK constraints.
- Soft delete is the norm: `deleted_at` set/cleared, `updated_at = now()` on update.
- Tx work (e.g. `BlockService.Reorder`): `tx, err := b.pool.Begin(ctx)`,
  `defer tx.Rollback(ctx)`, `q := database.New(tx)`, `tx.Commit(ctx)`.
- No comments in code. Errors passed through unwrapped.

## Go — internal/database (sqlc)

- Queries annotated `-- name: QueryName :many|:one|:exec`, keywords uppercase,
  soft-delete filters `AND deleted_at IS NULL` on every read.
- Lowercase SQL keywords in a query = internal calculation; uppercase = used
  outside the dir scope. Mirror what the neighbouring queries in the file do.
- EDIT QUERIES ONLY: add a query to `internal/database/queries/*.sql`, then
  regenerate via sqlc. Never edit generated `*.sql.go`.

## templ (ui/)

- Structure: `ui/layouts/` (shells), `ui/modules/` (Alpine chunks: navbar,
  sidebar, backdrop), `ui/pages/` (full pages), `ui/components/` (templui).
- Case rule from "Naming convention": internal helpers lowercase (`head`, `body`),
  exported components PascalCase (`SideBar`, `Landing`).
- Self-closing tags with trailing slash `<br/>`; templ expressions as `{ title }`.
- Alpine attributes inline (`x-data`, `x-show`, `:class`, `@click`); Tailwind v4
  utilities; dark mode via `class="h-full dark"` on the shell; shadcn-style
  oklch tokens in components, `bg-zinc-*` in modules.
- `Alpine.initTree(target)` after injecting dynamic HTML.

## JavaScript (static/js/)

- JS is ONLY used where Alpine needs it — no plain-DOM code otherwise; prefer
  templ for everything static. layout.js style: tabs, single quotes, no
  semicolons, arrow functions, async/await, `Alpine.data('x', () => ({...}))`
  registered on `alpine:init`. Fetch failures silently `return`.

## SQL migrations

- Numbered pairs `internal/database/migrations/NNNNNN_name.{up,down}.sql`;
  UUID PKs `gen_random_uuid()`, timestamps `default now()`. Applied by
  `cmd/migrate` (embedded via `migrations.FS`, tracked in `schema_migrations`,
  idempotent); compose runs it as a one-shot `migrate` service before `app`.

## Errors / logging

- `log.Printf` / `log.Fatalf("error ...: %v", err)` — lowercase, `: %v`;
  user-facing messages `"Error: ..."` uppercase in renderError calls.
- Explicit statuses (`http.StatusFound`, `http.StatusUnauthorized`).

## Language

- English only (keep existing Indonesian UI copy as is). No emojis.
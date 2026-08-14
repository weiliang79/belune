# CLAUDE.md

Belune — a self-hosted PaaS that ships as **one Go binary with the React SPA embedded**.
Monorepo: `apps/api` (Go 1.25 · chi · pgx · sqlc · asynq), `apps/web` (React 19 · Vite · TanStack Router/Query · Tailwind 4 · shadcn/base-ui), `apps/site` (Next.js + Fumadocs, belune.dev).

## Commands

Run `task` from the repo root; `Taskfile.yml` loads `.env`.

| Task | What it does |
|---|---|
| `task dev:infra` | Postgres + Redis + Caddy via compose (start this first) |
| `task dev:api` / `task dev:web` | Air hot-reload API / Vite dev server |
| `task build` | `build:web` (copies to `apps/api/web/dist/`) then `build:api` |
| `task test` / `test:api` / `test:web` | Go `./...` + Vitest |
| `task lint` / `task fmt` | golangci-lint + ESLint / gofmt + Prettier |
| `task db:migrate:create -- <name>` | New migration pair |
| `task db:migrate:up`, `task generate:sqlc` | Apply migrations, regenerate query code |

Single Go test: `cd apps/api && go test -count=1 -timeout=300s ./internal/handler/ -run TestName`.
Handler/worker tests are **integration tests using testcontainers — Docker must be running**. Use `-short` to skip them. Always pass `-count=1`; the cache silently replays passes for tests that read files outside the module.

## Architecture

Request path: `internal/server/routes.go` → `internal/handler/*.go` → `internal/service/*.go` → `internal/store/generated` (sqlc). Async work: `internal/worker/*.go` (asynq, task names in `worker/tasks.go`). Live updates: `internal/ws` hub, broadcast by channel string (`metrics:host`, `container-logs:{id}`).
Frontend mirrors it: `src/lib/api/<resource>.ts` (fetch) → `src/lib/hooks/use-<resource>.ts` (TanStack Query) → `src/routes/_app/**` (file-based routes) → `src/components/<domain>/`.

**Keep handlers thin** — validation, auth check, DTO mapping only. Business logic goes in a service.

## Standards

- Go: `gofmt`; wrap errors with `fmt.Errorf("doing x: %w", err)`; log with `slog`.
- Responses: **only** `writeJSON(w, status, data)` / `writeError(w, status, msg)` from `handler.go`. Never hand-roll JSON.
- Multi-statement writes go inside `store.WithTx(ctx, pool, func(q *generated.Queries) error {...})`.
- TS: strict mode, `@/*` alias, no `any`, `import type` for types (`verbatimModuleSyntax`).
- Comments explain **why**, not what — match the existing density; the codebase favors a short rationale over a restatement.
- **Never hand-edit** `apps/api/internal/store/generated/**` or `apps/web/src/routeTree.gen.ts`.

## Procedures

**Changing the schema** → add a forward-only `apps/api/internal/migrations/0000NN_name.up.sql` (forward-only is the convention here; new migrations ship without a `.down.sql`) → edit `internal/store/queries/*.sql` → `task generate:sqlc` → verify with `go run ./cmd/migrate-check` against a fresh DB.

**Dropping or restructuring existing data** → v0.1.x is a public release line and `update.sh` runs migrations against live installs, so a migration that drops a column/table or rewrites existing rows breaks real upgrades — and a down migration would not save them, since the container has already restarted by then. Preserve and backfill in place, or ship a documented upgrade path. Purely additive changes need no such ceremony.

**Writing/using a full-row `UPDATE`** (e.g. `UpdateApplication` sets every source column) → the handler MUST `Get<Resource>` first and fall back to the stored value for each field the request omits. Passing a partial body straight through silently clears `source_repo`, `branch`, `root_directory`, etc.

**Adding an endpoint** → register in `internal/server/routes.go` inside the right auth/rate-limit group and wrap with `withTimeout(handlerTimeout)`; add a handler + service method; call `h.audit(r, action, resourceType, id, details)` for any mutation; add an integration test in `internal/handler/*_test.go` (start from `resetDB(t)` + `testutil` fixtures).

**Adding a background job** → new `Type<Name>` const in `worker/tasks.go`, handler file in `internal/worker/`, register in `worker.go`, enqueue via the `TaskEnqueuer` interface (never `*asynq.Client` directly — tests mock the interface).

**Storing a secret** → encrypt with `cfg.Keyring.Encrypt`, persist to the `*_encrypted` column, and mask on read. Add new key columns to `cmd/rewrap` so key rotation covers them. Redact paths/errors with `internal/pkg/redact` before logging.

**Adding frontend data access** → API fn in `src/lib/api/`, hook in `src/lib/hooks/`, and register the key in `src/lib/hooks/query-keys.ts` — invalidate through that object, never an inline array literal.

**Type-checking the web app** → `npx tsc --noEmit` checks **nothing** here (root tsconfig is solution-style with `files: []`) and exits 0 in 0.6s. Use `tsc -b --noEmit` or `task build:web`.

**Adding a UI component** → reuse `src/components/ui/` (41 shadcn/base-ui primitives, `style: base-nova`, lucide icons). Add via the shadcn skill/CLI rather than pasting; use `Switch` for toggles and `PageTabs` for tabbed pages.

**Committing** → Conventional Commits with a scope (`feat(api,web):`, `fix(deploy):`) and **DCO sign-off**: `git commit -s`. One logical change per commit.

**Touching deploys, databases, backups, or Caddy routing** → mocks do not catch Docker or Caddy behavior. Say so, and smoke-test against a real dev stack before claiming it works.

**Editing `apps/site/content/docs/*.mdx`** → every `##`/`###`/`####` heading is Title Case.

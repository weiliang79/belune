# Belune — Project Structure & Architecture

## Overview

This document defines the monorepo structure, folder layout, naming conventions,
and architectural boundaries for the self-hosted PaaS project.

---

## Tech Stack Summary

| Layer | Choice |
|---|---|
| Frontend | SvelteKit (adapter-static) |
| UI Components | shadcn-svelte + Tailwind |
| Backend | Go + Chi |
| Database | PostgreSQL + sqlc |
| Job Queue | Asynq + Redis |
| Reverse Proxy | Caddy (programmatic API) |
| Container Runtime | Docker (abstracted interface) |
| Monorepo Tool | Task (Taskfile) |

---

## Monorepo Root Layout

```
belune/
├── apps/
│   ├── api/                  # Go backend
│   └── web/                  # SvelteKit frontend
├── packages/
│   └── types/                # Shared OpenAPI spec / type contracts
├── infra/
│   ├── docker-compose.yml    # Local development stack
│   ├── docker-compose.prod.yml
│   └── caddy/
│       └── Caddyfile.template
├── scripts/
│   ├── install.sh            # One-liner installer for end users
│   ├── update.sh
│   └── dev-setup.sh
├── docs/
│   ├── architecture.md
│   ├── api.md
│   └── contributing.md
├── Taskfile.yml              # Task runner (replaces Makefile)
├── .env.example
├── .gitignore
└── README.md
```

---

## Backend — `apps/api/`

```
apps/api/
├── main.go                   # Entry point — wires everything together
├── Dockerfile
│
├── cmd/
│   └── server/
│       └── main.go           # CLI flags, config loading, server start
│
├── internal/                 # Private application code (not importable externally)
│   │
│   ├── config/
│   │   └── config.go         # Env var parsing, defaults, validation
│   │
│   ├── server/
│   │   ├── server.go         # HTTP server setup, middleware chain
│   │   ├── routes.go         # Route registration
│   │   └── middleware/
│   │       ├── auth.go       # JWT validation middleware
│   │       ├── logger.go     # Request logging
│   │       └── cors.go
│   │
│   ├── handler/              # HTTP handlers (thin — delegate to services)
│   │   ├── auth.go
│   │   ├── projects.go
│   │   ├── services.go       # App services (deploy, stop, restart)
│   │   ├── databases.go      # Database provisioning
│   │   ├── domains.go
│   │   ├── envvars.go
│   │   ├── deployments.go
│   │   ├── logs.go           # SSE log streaming
│   │   └── metrics.go
│   │
│   ├── service/              # Business logic layer
│   │   ├── auth.go
│   │   ├── project.go
│   │   ├── deploy.go         # Core deploy orchestration
│   │   ├── database.go
│   │   ├── domain.go
│   │   ├── envvar.go
│   │   └── metrics.go
│   │
│   ├── worker/               # Asynq background job workers
│   │   ├── worker.go         # Worker server setup
│   │   ├── tasks.go          # Task type constants
│   │   ├── deploy_task.go    # Handle deploy jobs
│   │   ├── build_task.go     # Handle build jobs (CNB / Nixpacks / Dockerfile)
│   │   └── cleanup_task.go   # Handle cleanup jobs
│   │
│   ├── runtime/              # Container runtime abstraction
│   │   ├── interface.go      # ContainerRuntime interface definition
│   │   ├── docker/
│   │   │   ├── client.go     # Docker SDK wrapper
│   │   │   ├── container.go  # Container CRUD
│   │   │   ├── network.go    # Network management
│   │   │   ├── volume.go     # Volume management
│   │   │   └── image.go      # Image pull/build
│   │   └── podman/           # Future: Podman backend
│   │       └── client.go
│   │
│   ├── proxy/                # Caddy reverse proxy management
│   │   ├── interface.go      # ProxyManager interface
│   │   └── caddy/
│   │       ├── client.go     # Caddy Admin API client
│   │       ├── routes.go     # Add/remove routes
│   │       └── tls.go        # TLS cert management
│   │
│   ├── git/                  # Git integration
│   │   ├── clone.go          # Clone repos
│   │   ├── webhook.go        # GitHub/GitLab webhook parsing
│   │   └── providers/
│   │       ├── github.go
│   │       └── gitlab.go
│   │
│   ├── build/                # App build abstraction — layered strategy
│   │   ├── interface.go      # Builder interface + BuildOptions struct
│   │   ├── detector.go       # Auto-detect best builder for a given source
│   │   ├── chain.go          # Priority chain: Dockerfile → CNB → Nixpacks
│   │   │
│   │   ├── dockerfile/
│   │   │   └── builder.go    # Build via Docker BuildKit (user-provided Dockerfile)
│   │   │
│   │   ├── buildpacks/       # Cloud Native Buildpacks (CNB) — default auto-builder
│   │   │   ├── builder.go    # Runs `pack build` via Heroku/Paketo builder image
│   │   │   └── builders.go   # Known builder image registry (heroku/builder:24, etc.)
│   │   │
│   │   ├── nixpacks/         # Nixpacks — opt-in or CNB fallback
│   │   │   └── builder.go    # Runs nixpacks CLI, wraps errors with context
│   │   │
│   │   ├── railpack/         # Railpack — future, behind feature flag
│   │   │   └── builder.go
│   │   │
│   │   └── image/
│   │       └── builder.go    # Pre-built image — just pull and deploy, no build step
│   │
│   ├── store/                # Database access layer (sqlc generated)
│   │   ├── db.go             # DB connection setup
│   │   ├── queries/          # Raw SQL files
│   │   │   ├── projects.sql
│   │   │   ├── services.sql
│   │   │   ├── deployments.sql
│   │   │   ├── databases.sql
│   │   │   ├── domains.sql
│   │   │   ├── envvars.sql
│   │   │   └── users.sql
│   │   └── generated/        # sqlc output (do not edit manually)
│   │       ├── db.go
│   │       ├── models.go
│   │       └── *.sql.go
│   │
│   ├── migrations/           # SQL migration files
│   │   ├── 001_init.up.sql
│   │   ├── 001_init.down.sql
│   │   ├── 002_add_domains.up.sql
│   │   └── 002_add_domains.down.sql
│   │
│   └── pkg/                  # Reusable internal utilities
│       ├── logger/           # Structured logging (slog)
│       ├── validator/        # Request validation
│       ├── pagination/       # Cursor pagination helpers
│       └── sse/              # Server-Sent Events helpers
│
├── frontend/                 # Embedded SvelteKit build (at compile time)
│   └── .gitkeep              # Populated by build script
│
├── go.mod
├── go.sum
└── sqlc.yaml                 # sqlc config
```

---

## Frontend — `apps/web/`

```
apps/web/
├── src/
│   ├── app.html              # HTML shell
│   ├── app.css               # Global styles
│   │
│   ├── lib/
│   │   ├── components/       # Reusable UI components
│   │   │   ├── ui/           # shadcn-svelte base components
│   │   │   │   ├── button/
│   │   │   │   ├── badge/
│   │   │   │   ├── dialog/
│   │   │   │   ├── input/
│   │   │   │   ├── table/
│   │   │   │   └── ...
│   │   │   │
│   │   │   ├── layout/
│   │   │   │   ├── Sidebar.svelte
│   │   │   │   ├── Header.svelte
│   │   │   │   ├── Breadcrumb.svelte
│   │   │   │   └── PageLayout.svelte
│   │   │   │
│   │   │   ├── deploy/
│   │   │   │   ├── DeployButton.svelte
│   │   │   │   ├── DeployStatus.svelte
│   │   │   │   ├── DeployHistory.svelte
│   │   │   │   ├── BuilderSelector.svelte     # Choose builder or leave on auto-detect
│   │   │   │   └── LogStream.svelte           # SSE log viewer
│   │   │   │
│   │   │   ├── service/
│   │   │   │   ├── ServiceCard.svelte
│   │   │   │   ├── ServiceActions.svelte
│   │   │   │   └── ResourceMeter.svelte
│   │   │   │
│   │   │   ├── database/
│   │   │   │   ├── DatabaseCard.svelte
│   │   │   │   └── CreateDatabaseModal.svelte
│   │   │   │
│   │   │   └── common/
│   │   │       ├── EmptyState.svelte
│   │   │       ├── ErrorBoundary.svelte
│   │   │       ├── LoadingSpinner.svelte
│   │   │       └── ConfirmDialog.svelte
│   │   │
│   │   ├── api/              # API client functions
│   │   │   ├── client.ts     # Base fetch wrapper (auth headers, error handling)
│   │   │   ├── projects.ts
│   │   │   ├── services.ts
│   │   │   ├── deployments.ts
│   │   │   ├── databases.ts
│   │   │   ├── domains.ts
│   │   │   └── envvars.ts
│   │   │
│   │   ├── stores/           # Svelte stores (global state)
│   │   │   ├── auth.ts       # User session store
│   │   │   ├── toast.ts      # Notification store
│   │   │   └── sidebar.ts    # UI state
│   │   │
│   │   └── utils/
│   │       ├── format.ts     # Date, bytes, duration formatters
│   │       ├── sse.ts        # EventSource helper
│   │       └── clipboard.ts
│   │
│   └── routes/               # SvelteKit file-based routing
│       ├── +layout.svelte    # Root layout (auth check)
│       ├── +layout.ts        # Root load function
│       │
│       ├── login/
│       │   └── +page.svelte
│       │
│       ├── setup/            # First-run setup wizard
│       │   └── +page.svelte
│       │
│       └── (app)/            # Protected routes group
│           ├── +layout.svelte    # App shell with sidebar
│           │
│           ├── dashboard/
│           │   └── +page.svelte  # Overview — all services at a glance
│           │
│           ├── projects/
│           │   ├── +page.svelte          # Project list
│           │   ├── new/
│           │   │   └── +page.svelte      # Create project
│           │   └── [projectId]/
│           │       ├── +layout.svelte    # Project layout (tabs)
│           │       ├── +page.svelte      # Project overview
│           │       │
│           │       ├── services/
│           │       │   ├── +page.svelte           # Service list
│           │       │   ├── new/
│           │       │   │   └── +page.svelte       # Deploy new service
│           │       │   └── [serviceId]/
│           │       │       ├── +page.svelte        # Service overview
│           │       │       ├── deployments/
│           │       │       │   └── +page.svelte    # Deploy history
│           │       │       ├── logs/
│           │       │       │   └── +page.svelte    # Live log viewer
│           │       │       ├── env/
│           │       │       │   └── +page.svelte    # Env var editor
│           │       │       ├── domains/
│           │       │       │   └── +page.svelte    # Domain management
│           │       │       └── settings/
│           │       │           └── +page.svelte    # Service settings (incl. builder override)
│           │       │
│           │       └── databases/
│           │           ├── +page.svelte            # Database list
│           │           └── [databaseId]/
│           │               └── +page.svelte        # DB details + connection string
│           │
│           └── settings/
│               ├── +page.svelte          # General settings
│               ├── team/
│               │   └── +page.svelte      # Team members
│               └── server/
│                   └── +page.svelte      # Server info + resource usage
│
├── static/
│   └── favicon.ico
│
├── svelte.config.js          # SvelteKit config (adapter-static)
├── vite.config.ts
├── tailwind.config.ts
├── tsconfig.json
└── package.json
```

---

## Infrastructure — `infra/`

```
infra/
├── docker-compose.yml         # Local dev: postgres, redis, caddy
├── docker-compose.prod.yml    # Production stack
│
└── caddy/
    ├── Caddyfile.template     # Base Caddy config template
    └── sites/                 # Per-app configs (managed by PaaS at runtime)
        └── .gitkeep
```

---

## Database Schema (Core Tables)

```
users
  id, email, password_hash, role, created_at

projects
  id, name, slug, user_id, created_at

services
  id, project_id, name, type (git|image|compose)
  source_repo, source_image, dockerfile_path
  build_type (dockerfile|buildpacks|nixpacks|railpack|image)
  build_type_override  # NULL = auto-detect, set to force a specific builder
  status, created_at, updated_at

deployments
  id, service_id, status (pending|building|deploying|success|failed)
  triggered_by (push|manual|api)
  commit_sha, build_logs, error_message
  started_at, finished_at

databases
  id, project_id, type (postgres|mysql|redis|mongo)
  name, version, status
  internal_host, internal_port, credentials_encrypted
  created_at

domains
  id, service_id, hostname, ssl_enabled
  caddy_config_id, verified_at, created_at

env_vars
  id, service_id, key, value_encrypted, is_secret
  created_at, updated_at
```

---

## Key Architectural Rules

**1. Handlers stay thin**
Handlers only parse the request, call a service method, and write the response.
All business logic lives in `internal/service/`.

**2. Runtime is always abstracted**
Nothing outside `internal/runtime/` calls Docker directly.
All container operations go through the `ContainerRuntime` interface.

**3. Proxy is always abstracted**
Nothing outside `internal/proxy/` calls Caddy directly.
Route management goes through the `ProxyManager` interface.

**4. All deploy work is async**
HTTP handlers never run deploy logic inline. They enqueue an Asynq job
and return a deployment ID. The frontend polls or streams progress via SSE.

**5. Secrets are always encrypted at rest**
`env_vars.value_encrypted` and `databases.credentials_encrypted`
are always encrypted before hitting the database. The encryption key
comes from config, never hardcoded.

**6. Frontend is embedded in binary**
At build time, the SvelteKit static output is embedded into the Go binary
via `//go:embed`. End users run a single binary with no external file dependencies.

**7. Build strategy follows a priority chain**
The build system never assumes a single tool. For every deploy, `build/detector.go`
resolves the builder in this order:

```
Has Dockerfile in repo?
  └── YES → dockerfile/builder.go (Docker BuildKit)
  └── NO  → Was a builder explicitly set by user?
              └── YES → Use that builder directly
              └── NO  → Try buildpacks/builder.go (CNB, Heroku builder:24)
                          └── CNB fails or unsupported language?
                                └── Fallback → nixpacks/builder.go
```

Railpack is available behind a `FEATURE_RAILPACK=true` env flag until it matures.
This means users are never silently broken — there is always a fallback path.

---

## Task Runner — `Taskfile.yml`

```yaml
# Key tasks available to developers
tasks:
  dev:api       # Run Go API with hot reload (air)
  dev:web       # Run SvelteKit dev server
  dev:infra     # Start postgres, redis, caddy via docker compose

  build:api     # Build Go binary with embedded frontend
  build:web     # Build SvelteKit static output

  generate:sqlc       # Regenerate sqlc Go code from SQL queries
  generate:types      # Regenerate types from OpenAPI spec

  db:migrate:up       # Run migrations
  db:migrate:down     # Rollback last migration
  db:migrate:create   # Create new migration file

  test:api      # Run Go tests
  test:web      # Run Svelte component tests

  lint          # Run golangci-lint + eslint
  fmt           # Run gofmt + prettier
```

---

## Development Workflow

```
1. Clone repo
2. task dev:infra          → starts Postgres + Redis + Caddy locally
3. task dev:api            → Go API on :8080 with hot reload
4. task dev:web            → SvelteKit on :5173 with HMR
5. Visit http://localhost:5173 → proxied to :8080 for API calls

Production build:
1. task build:web          → outputs to apps/api/web/dist/
2. task build:api          → embeds frontend via //go:embed, outputs single binary
3. ./belune                  → serves both API and UI on :8080
```

---

## Installer Script (End User Experience)

```bash
# What end users run — single command to get started
curl -sSL https://get.yourbelune.com | bash

# Script does:
# 1. Detect OS + arch
# 2. Download latest binary from GitHub releases
# 3. Install to /usr/local/bin/belune
# 4. Create systemd service
# 5. Generate .env with secure random secrets
# 6. Start service
# 7. Print URL + initial admin credentials
```

---

## Phase 1 Scope (MVP)

Focus only on these to reach a usable v1:

- User auth (single admin user, invite flow later)
- Projects + Services CRUD
- Deploy from Docker image
- Deploy from Git repo with layered auto-build (Dockerfile → CNB → Nixpacks fallback)
- Docker Compose file import
- Environment variable management
- Custom domain + auto-SSL via Caddy
- Real-time deploy logs via SSE
- One-click Postgres, Redis provisioning
- Restart / Redeploy / Stop / Remove actions
- Single binary installer

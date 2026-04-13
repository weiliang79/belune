# Self-Hosted PaaS

A self-hosted Platform-as-a-Service for deploying applications with automatic builds, custom domains, and managed databases.

## Prerequisites

- Go 1.23+
- Node.js 20+
- Docker & Docker Compose
- [Task](https://taskfile.dev/) (`go install github.com/go-task/task/v3/cmd/task@latest`)
- [Air](https://github.com/air-verse/air) (`go install github.com/air-verse/air@latest`)
- [golang-migrate](https://github.com/golang-migrate/migrate) CLI
- [sqlc](https://sqlc.dev/) CLI

## Quick Start

```bash
# 1. Copy environment config
cp .env.example .env

# 2. Start infrastructure (Postgres, Redis, Caddy)
task dev:infra

# 3. Run database migrations
task db:migrate:up

# 4. Start the API server (with hot reload)
task dev:api

# 5. Start the frontend dev server
task dev:web
```

- API: http://localhost:8080
- Frontend: http://localhost:5173

## Develop with the devcontainer

The VS Code devcontainer runs the Go API inside a container that joins the `paas-infra` Docker network. This gives the API real Docker DNS (Postgres/Redis/Caddy reachable by service name) and lets Caddy route `localhost:80` to the API correctly. Vite always runs on the host for fast HMR.

**Prerequisites:** Docker Desktop or OrbStack, VS Code with the Dev Containers extension.

```bash
# 1. Ensure the shared network exists (done automatically by initializeCommand)
docker network create paas-infra 2>/dev/null || true

# 2. Open in devcontainer
# VS Code: Ctrl+Shift+P → "Dev Containers: Reopen in Container"

# 3. Inside the devcontainer terminal:
task db:migrate:up
task dev:api        # API listens on :8080 (forwarded to host)

# 4. On the host (separate terminal):
task dev:web        # Vite on :5173, proxies /api/* to localhost:8080
```

**Important:** `task dev:infra` and the devcontainer are **mutually exclusive** — they both try to bind ports 80, 443, 5432, and 6379. Stop one before starting the other.

## Project Structure

See [paas-project-structure.md](paas-project-structure.md) for full architecture details.

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

## Project Structure

See [paas-project-structure.md](paas-project-structure.md) for full architecture details.

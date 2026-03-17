# Contributing

## Development Setup

1. Clone the repository
2. Run `./scripts/dev-setup.sh`
3. Start the API: `task dev:api`
4. Start the frontend: `task dev:web`

## Project Structure

- `apps/api/` — Go backend
- `apps/web/` — SvelteKit frontend
- `packages/types/` — Shared OpenAPI spec
- `infra/` — Docker Compose and Caddy configs

## Code Style

- Go: `gofmt` + `golangci-lint`
- TypeScript: Prettier + ESLint
- Run `task fmt` and `task lint` before submitting changes

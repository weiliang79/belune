# Architecture

See [paas-project-structure.md](../paas-project-structure.md) for the full architecture document.

## Key Principles

- Handlers stay thin — business logic lives in services
- Container runtime is abstracted behind an interface
- Reverse proxy is abstracted behind an interface
- All deploy work is async via Asynq job queue
- Secrets are encrypted at rest with AES-256-GCM
- Frontend is embedded in the Go binary at build time
- Build strategy follows a priority chain with user override support

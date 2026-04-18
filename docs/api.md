# API Documentation

The authoritative API surface is the Chi route table in
[`apps/api/internal/server/routes.go`](../apps/api/internal/server/routes.go).
A shared OpenAPI spec is intentionally not maintained during alpha — the route
table and sqlc-generated models are the source of truth.

## Base URL

- Development: `http://localhost:8080/api`
- Production: `https://your-domain.com/api`

## Authentication

All API endpoints (except `/api/auth/login`) require authentication via session cookie.

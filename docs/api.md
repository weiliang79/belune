# API Documentation

The API is defined using OpenAPI 3.1. See the spec at:

- [packages/types/openapi.yaml](../packages/types/openapi.yaml)

## Base URL

- Development: `http://localhost:8080/api`
- Production: `https://your-domain.com/api`

## Authentication

All API endpoints (except `/api/auth/login`) require authentication via session cookie.

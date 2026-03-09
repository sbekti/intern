# intern-api

Clean-room backend for an internal site such as `intern.corp.example.com`.

## Status

Bootstrap service only. Current scope:

- Go HTTP server with `chi`
- environment-based config loading
- structured JSON logging via `log/slog`
- health endpoints

## Endpoints

- `GET /healthz`
- `GET /readyz`
- `GET /api/v1/system/ping`

Admin-only route enforcement is provided by `internal/auth.Authorizer`. Handlers should wrap authenticated routes with `RequireAuthenticated()` and admin-only routes with `RequireAdmin()`.

## API contract

- OpenAPI source: [`api/openapi.yaml`](./api/openapi.yaml)

## Configuration

- `INTERN_API_ADDR`
  - listen address
  - default: `:8080`
- `INTERN_API_LOG_LEVEL`
  - one of `debug`, `info`, `warn`, `error`
  - default: `info`
- `TRUSTED_PROXY_CIDRS`
  - comma-separated CIDRs allowed to supply `Remote-*` headers
  - default: `127.0.0.1/32,::1/128`
- `AUTH_REMOTE_USER_HEADER`, `AUTH_REMOTE_NAME_HEADER`, `AUTH_REMOTE_EMAIL_HEADER`, `AUTH_REMOTE_GROUPS_HEADER`
  - forwarded identity header names
  - defaults: `Remote-User`, `Remote-Name`, `Remote-Email`, `Remote-Groups`
- `AUTH_ADMIN_GROUPS`
  - comma-separated admin group names
  - default: `Super-Users`
- `AUTH_JWT_ISSUER`, `AUTH_JWT_AUDIENCE`, `AUTH_JWT_HMAC_SECRET`
  - JWT validation settings for CLI bearer tokens
  - defaults: `intern.corp.example.com`, `internctl`, `dev-insecure-jwt-secret`

## Local development

```bash
go generate ./internal/api
go generate ./internal/db
go test ./...
go run ./cmd/intern-api
```

## Database migrations

Pinned migration tool:

- `github.com/pressly/goose/v3 v3.27.0`

Migration files live under [`db/migrations`](./db/migrations).

Tooling for `oapi-codegen`, `sqlc`, and `goose` is isolated in [`tools/go.mod`](./tools/go.mod) so the main module keeps only runtime dependencies.

This repo is public-safe by design. Do not commit secrets.

# intern-api

Clean-room backend for an internal site such as `intern.corp.example.com`.

## Status

Bootstrap service only. Current scope:

- Go HTTP server with `chi`
- environment-based config loading
- structured JSON logging via `log/slog`
- health endpoints
- authenticated profile endpoint
- authenticated dashboard endpoint with Redis-backed weather caching
- VLAN listing and admin-managed VLAN CRUD endpoints

## Endpoints

- `GET /healthz`
- `GET /readyz`
- `GET /api/v1/system/ping`
- `GET /api/v1/profile`
- `GET /api/v1/dashboard`
- `GET /api/v1/networks/vlans`
- `GET /api/v1/networks/vlans/{id}`
- `POST /api/v1/networks/vlans`
- `PATCH /api/v1/networks/vlans/{id}`
- `DELETE /api/v1/networks/vlans/{id}`

Admin-only route enforcement is provided by `internal/auth.Authorizer`. Handlers should wrap authenticated routes with `RequireAuthenticated()` and admin-only routes with `RequireAdmin()`.

## API contract

- OpenAPI source: [`api/openapi.yaml`](./api/openapi.yaml)

## Configuration

- `INTERN_API_ADDR`
  - listen address
  - default: `:8080`
- `INTERN_API_DATABASE_URL`
  - PostgreSQL connection string for persisted application state
  - required
- `INTERN_API_REDIS_URL`
  - Redis connection string for caching and future auth/session features
  - required
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
- `WEATHER_BASE_URL`
  - Open-Meteo forecast API base URL
  - default: `https://api.open-meteo.com/v1/forecast`
- `WEATHER_LOCATION_NAME`
  - display name returned in the dashboard weather payload
  - default: `Configured Location`
- `WEATHER_LATITUDE`, `WEATHER_LONGITUDE`
  - coordinates used for the weather query
  - default: `0`, `0`
- `WEATHER_CACHE_TTL`
  - Redis cache TTL for weather responses
  - default: `15m`

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

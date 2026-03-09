# intern-api

Clean-room backend for `intern.corp.bekti.com`.

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

## API contract

- OpenAPI source: [`api/openapi.yaml`](./api/openapi.yaml)

## Configuration

- `INTERN_API_ADDR`
  - listen address
  - default: `:8080`
- `INTERN_API_LOG_LEVEL`
  - one of `debug`, `info`, `warn`, `error`
  - default: `info`

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

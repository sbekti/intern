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
- admin-managed network device CRUD with RADIUS synchronization
- client auth device-code flow with short-lived access tokens and rotating refresh tokens
- bearer-token requests are validated against active auth sessions on every request

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
- `GET /api/v1/networks/devices`
- `GET /api/v1/networks/devices/{id}`
- `POST /api/v1/networks/devices`
- `PATCH /api/v1/networks/devices/{id}`
- `DELETE /api/v1/networks/devices/{id}`
- `POST /api/v1/auth/device_codes`
- `POST /api/v1/auth/device_codes/{user_code}/approve`
- `POST /api/v1/auth/device_codes/{user_code}/deny`
- `POST /api/v1/auth/tokens`
- `POST /api/v1/auth/tokens/refresh`
- `POST /api/v1/auth/logout`

Route authorization is provided by `internal/auth.Authorizer`. Bearer-token session validity is enforced separately in the auth middleware before request handlers run.

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
  - Redis connection string for weather caching and auth-flow rate limiting
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
  - JWT validation settings for public-client bearer tokens
  - defaults: `intern.corp.example.com`, `internctl`, `dev-insecure-jwt-secret`
- `AUTH_PUBLIC_BASE_URL`
  - canonical public browser base URL used to build device-flow verification links
  - required
- `AUTH_ACCESS_TOKEN_TTL`
  - access token lifetime
  - default: `15m`
- `AUTH_REFRESH_IDLE_TTL`
  - refresh session idle lifetime
  - default: `720h`
- `AUTH_REFRESH_ABSOLUTE_TTL`
  - refresh session absolute lifetime
  - default: `2160h`
- `AUTH_DEVICE_CODE_TTL`
  - device code lifetime
  - default: `10m`
- `AUTH_DEVICE_POLL_INTERVAL`
  - minimum polling interval for device token exchange
  - default: `5s`
- `AUTH_DEVICE_CODE_CREATE_RATE_LIMIT`, `AUTH_DEVICE_CODE_CREATE_RATE_WINDOW`
  - IP-based rate limit for unauthenticated device-code creation
  - defaults: `10`, `1m`
- `AUTH_DEVICE_TOKEN_EXCHANGE_RATE_LIMIT`, `AUTH_DEVICE_TOKEN_EXCHANGE_RATE_WINDOW`
  - IP-based rate limit for unauthenticated device token exchange polling
  - defaults: `120`, `1m`
- `AUTH_DEVICE_DECISION_RATE_LIMIT`, `AUTH_DEVICE_DECISION_RATE_WINDOW`
  - user-plus-IP rate limit for authenticated approve and deny actions
  - defaults: `30`, `1m`
- `AUTH_REFRESH_TOKEN_RATE_LIMIT`, `AUTH_REFRESH_TOKEN_RATE_WINDOW`
  - IP-based rate limit for unauthenticated refresh-token rotation
  - defaults: `60`, `1m`
- `AUTH_LOGOUT_RATE_LIMIT`, `AUTH_LOGOUT_RATE_WINDOW`
  - IP-based rate limit for unauthenticated logout requests
  - defaults: `60`, `1m`
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
go test -tags=integration ./internal/vlans ./internal/devices ./internal/clientauth ./internal/authspam ./internal/weather ./internal/httpserver
go run ./cmd/intern-api
```

The tagged integration tests start ephemeral Postgres and Redis containers with `testcontainers-go`. If Docker is not accessible, they skip cleanly.

## Local Compose E2E

Run the local API stack with a dev auth proxy:

```bash
docker compose -p intern-api-dev up --build -d
docker compose -p intern-api-dev logs -f intern-api dev-auth-proxy
```

The one-shot `migrate` service runs `goose up`, so repeated `docker compose up` runs are safe against an already-initialized dev database.

If you have an older local volume that was bootstrapped before Compose switched to `goose`, reset it once with:

```bash
docker compose -p intern-api-dev down -v
```

Shortcuts:

```bash
./scripts/dev-up.sh
./scripts/dev-logs.sh
./scripts/dev-down.sh
```

Proxy entrypoints:

- `http://localhost:18080`
  - fixed normal user: `alice`
  - groups: `Users`
- `http://localhost:18081`
  - fixed admin user: `bob`
  - groups: `Users,Super-Users`

The proxy overwrites inbound `Remote-*` headers before forwarding to the API, so client-supplied spoofed values do not win.

Useful smoke checks:

```bash
curl http://localhost:18080/api/v1/profile
curl http://localhost:18080/api/v1/networks/devices
curl -X POST http://localhost:18081/api/v1/networks/vlans \
  -H 'Content-Type: application/json' \
  -d '{"name":"lab","vlan_id":30}'
```

Device-code auth flow through the proxy:

```bash
curl -X POST http://localhost:18080/api/v1/auth/device_codes
curl -X POST http://localhost:18080/api/v1/auth/device_codes/ABCD-EFGH/approve
curl -X POST http://localhost:18080/api/v1/auth/tokens \
  -H 'Content-Type: application/json' \
  -d '{"device_code":"..."}'
```

During device login, the API will use `AUTH_PUBLIC_BASE_URL` as the canonical browser origin for verification links such as `/auth/device`.

Shut the stack down with:

```bash
docker compose -p intern-api-dev down -v
```

## Database migrations

Pinned migration tool:

- `github.com/pressly/goose/v3 v3.27.0`

Migration files live under [`db/migrations`](./db/migrations).

Tooling for `oapi-codegen`, `sqlc`, and `goose` is isolated in [`tools/go.mod`](./tools/go.mod) so the main module keeps only runtime dependencies.

This repo is public-safe by design. Do not commit secrets.

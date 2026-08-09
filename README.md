# Intern

Intern is a Go API, `internctl` CLI, and Next.js authenticated web app in one repository.

The API preserves authentication, authorization, sessions, audit logs, VLAN and network-device CRUD, disabled device state, and RADIUS synchronization. The authenticated home route intentionally contains no dashboard content; it only renders the application shell.

## Development

Copy `.env.example` to `.env`, replace every placeholder, and run:

```sh
npm ci
npm run dev
```

The root command runs Docker Compose watch directly. Compose starts only `intern-api` and `intern-web`; the API has no published host port and connects to the explicitly configured database URL. It never migrates or seeds the database. The web app is published on the configured bind address and port.

## Verification and generation

```sh
go generate ./internal/api ./internal/db
go test ./...
go test -race ./internal/authspam
```

The API contract is [`api/openapi.yaml`](api/openapi.yaml). Generated API models/client and SQLC output are derived from that contract and [`sqlc.yaml`](sqlc.yaml). Tool dependencies are isolated in [`tools/go.mod`](tools/go.mod).

Integration tests use isolated ephemeral PostgreSQL containers and are opt-in with `-tags=integration`.

## Releases

`.goreleaser.yaml` is prepared for the local `v0.2.0` release line. It builds future `internctl` archives and Homebrew updates; this checkpoint does not tag, publish, or modify the Homebrew tap.

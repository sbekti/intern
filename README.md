# Intern

Intern is a Go API, `internctl` CLI, and Next.js authenticated web app in one repository.

The API preserves authentication, authorization, sessions, audit logs, VLAN and network-device CRUD, disabled device state, and RADIUS synchronization. The authenticated home route intentionally contains no dashboard content; it only renders the application shell.

## Development

Copy `.env.example` to `.env` and replace every placeholder. Before starting the
development stack, keep these production database and Prometheus port-forwards
running:

```sh
kubectl -n db port-forward service/postgres-rw 15432:5432 --address 127.0.0.1,172.17.0.1
kubectl -n monitoring port-forward service/prometheus-kube-prometheus-prometheus 19090:9090 --address 127.0.0.1,172.17.0.1
```

Set `INTERN_API_DATABASE_URL` to the forwarded host URL, such as
`postgres://user:password@host.docker.internal:15432/intern?sslmode=require`.
Set `INTERN_PROMETHEUS_BASE_URL` to `http://host.docker.internal:19090`. Compose
provides the `host.docker.internal` host gateway, and database SSL remains
required. These connect to production services: use real credentials carefully
and remember that development actions perform real writes.

Then run:

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

Intern is maintained as one monorepo. The first release merged into this repository
is `v0.2.0`. Releases are tag-driven and use signed tags: push a signed `v*.*.*` tag
to run the release workflow. It validates the Go and web workspaces, then publishes
`internctl` release artifacts and updates `sbekti/homebrew-tap/Casks`. The Docker
workflow signs the published GHCR images.

Before creating a release, configure these repository-scoped GitHub values without
committing credentials:

- Variable: `HOMEBREW_PUBLISHER_CLIENT_ID`
- Secret: `HOMEBREW_PUBLISHER_PRIVATE_KEY`

The workflow exchanges these values for a short-lived GitHub App token scoped to the
`homebrew-tap` repository.

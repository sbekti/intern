# Intern

Intern is a Go API, `internctl` CLI, and Next.js authenticated web app in one repository.

The API preserves authentication, authorization, sessions, audit logs, VLAN and network-device CRUD, disabled device state, and RADIUS synchronization. The authenticated home route shows live host metrics from Prometheus.

## Development

Intern development connects to the production PostgreSQL and Prometheus services
available through the current kubectl context. The API does not migrate or seed
the database, but actions in the development UI can still write production data.

Copy the non-secret local configuration:

```sh
cp .env.example .env
```

The defaults work at `http://127.0.0.1:3000`. To open Intern from another
machine, change `AUTH_PUBLIC_BASE_URL` in `.env` to the URL used by your browser,
such as `http://<host-ip>:3000`. This is the only host-specific setting.

At startup, the helper reads only `INTERN_API_DATABASE_URL` from
`intern-backend-secret` in the current Kubernetes namespace. It rewrites the host
to the local PostgreSQL port-forward and passes the URL to Compose without
printing or storing it in `.env`. Fresh local-only JWT and forward-auth secrets
are generated for every run. Docker administrators can still inspect container
environment variables.

### Common commands

Run the helper from the repository root:

```sh
# Show the available options.
./scripts/dev.sh --help

# Start as the development user configured in .env.
npm run dev

# Stop the Intern Compose development stack.
npm run dev -- stop
```

The helper discovers the Docker bridge address, starts both Kubernetes
port-forwards, and runs the Compose watch stack. Keep it in the foreground and
press Ctrl-C once to stop the stack and both port-forwards. After changing `.env`,
stop and restart the helper so Compose reads the new values.

Open the app locally or check that it is responding:

```sh
curl --head http://127.0.0.1:3000
```

The web app listens on `0.0.0.0:3000` by default, so it is also available at
`http://<host-ip>:3000` when the host firewall allows it. The API has no published
host port. Every client that can reach the web app acts as the administrator
configured in `.env` and can modify production-backed data. Run it only on a
trusted network and stop the server when you finish.

The stop command is safe to run when no stack is active. A normally running
helper exits when its Compose stack stops and cleans up both port-forwards:

```sh
npm run dev -- stop
```

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

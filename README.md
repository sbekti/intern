# Intern

Intern is a Go API, `internctl` CLI, and Next.js authenticated web app in one repository.

The API preserves authentication, authorization, sessions, audit logs, VLAN and network-device CRUD, disabled device state, and RADIUS synchronization. The authenticated home route shows live IAD2 CPU and memory utilization from Prometheus.

## Development

Intern development connects to the production PostgreSQL and Prometheus services.
The API does not migrate or seed the database, but actions in the development UI
can still write production data.

Copy the example configuration and replace every `replace-with-...` placeholder:

```sh
cp .env.example .env
```

Set `INTERN_API_DATABASE_URL` to the forwarded host URL, such as
`postgres://user:password@host.docker.internal:15432/intern?sslmode=require`.
Set `INTERN_PROMETHEUS_BASE_URL` to `http://host.docker.internal:19090`. Compose
provides the `host.docker.internal` host gateway, and database SSL remains required.
Use credentials with only the permissions needed for your work.

### Common commands

Run the helper from the repository root:

```sh
# Show the available options.
./scripts/dev.sh --help

# Start without development identity injection.
./scripts/dev.sh

# Start as the development user configured in .env.
./scripts/dev.sh --dev-identity

# The npm wrapper accepts the same option.
npm run dev -- --dev-identity
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
host port. With `--dev-identity`, every client that can reach the web app acts as
the development user configured in `.env`. Use this option only on a trusted
network, choose the least privileged groups needed, and stop the server when you
finish.

If a previous Compose run was force-killed and left containers behind, clean them
up before starting again:

```sh
docker compose down --remove-orphans
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

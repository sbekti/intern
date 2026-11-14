# Intern

Intern is a Go API, `internctl` CLI, and Next.js authenticated web app in one repository.

The API preserves authentication, authorization, sessions, audit logs, VLAN and network-device CRUD, disabled device state, gizmo kiosk destinations, and RADIUS policy snapshots. The authenticated home route shows live host metrics from Prometheus.

The monitoring page reuses the groups and lanes in the frontend metrics YAML.
Electrical lanes support `watts`, `volt-amperes-reactive`, `volts`, `amperes`,
and `hertz`; use the existing `percent` format for power factor multiplied by 100.
Signed extents such as `[-5000, 5000]` display imported and exported active power.
An optional lane `normalRange`, such as `{ min: 210, max: 240 }` for voltage or
`{ min: 90 }` for power factor, colors only excursions outside those inclusive
bounds. Below-range values use blue bands and above-range values use green.
For these lanes, `extent` scales the excursion from the nearest bound, while
live and hover readouts retain the actual measurement. Missing samples remain
gaps. Lanes without `normalRange` continue showing measurement magnitude.

Inventory pages use segmented filters. Each text token searches its selected
field with literal, case-insensitive substring matching and applies on Enter or
focus-out. Names, MACs, VLAN descriptions, and Gizmo URLs accept text fragments;
VLAN IDs, colors, status, and destination state remain exact. Tokens combine
using AND. Escape cancels an edit; removing or clearing filters discards drafts.

Audit Logs use the same segmented bar. Actor and Metadata tokens use literal,
case-insensitive contains matching; Action, Resource Type, and Resource ID match
exactly. All filters combine using AND on the server before pagination. The
API exposes `actor_username_contains` and `metadata_contains` for text tokens;
existing exact filters and global `q` search remain available to API clients.
Exact actor restrictions in Activity URLs appear as removable `Actor (exact)`
tokens. Category choices come from `filter_options` across all records,
independent of the current filters or page. Changing a filter resets the page
and preserves page size and sort order.

Update both list and count predicates when changing audit search, and regenerate
OpenAPI and SQLC output. For a split API/web rollout, update the API first: the
web app requires the contains parameters and `filter_options` response.
No database migration is needed for search.

## Dashboard layout

Admins can use **Sections** on Home to show, hide, and reorder whole metric
sections for everyone. Save applies the shared layout; Cancel discards edits,
and Reset restores the metrics configuration order with all sections visible.
New sections appear automatically after the saved order. Other viewers see
changes on reload. Hiding sections does not change metric collection or polling.

Shared display preferences use authenticated `GET /api/v1/preferences` and
admin-only `PATCH /api/v1/preferences`. Patches replace supplied top-level
namespaces and preserve others. The first namespace is `dashboard`, containing
`section_order` and `hidden_sections` arrays of configured IDs. Storage is generic;
there are no per-account overrides. Deploy the additive database migration
before the updated API and frontend.

## Private Wi-Fi passwords

The device editor can save a private Wi-Fi password or retain the shared-password
policy. Generate fills an unsaved four-word EFF passphrase; Save commits it and
Cancel discards edits. Blank password fields retain the saved value. Turning
private access off retains the encrypted password for later reuse. These are
saved policies, not live access-point status. Network policy delivery is described
below.

Passwords accept 8 to 63 printable ASCII characters. They are encrypted with
AES-256-GCM using random nonces and the immutable device UUID as authenticated
data. Ordinary device responses expose only enabled/present metadata. Saved
passwords are revealed only through an admin-only, audited POST with
`Cache-Control: no-store`; audit records contain neither plaintext nor ciphertext.

RADIUS needs the original Wi-Fi key. This is the
[recoverable-secret case described by OWASP](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html#hashing-vs-encryption).
The implementation uses Go's
[AES-GCM with automatic random nonces](https://pkg.go.dev/crypto/cipher#NewGCMWithRandomNonce)
and follows [OWASP's key separation guidance](https://cheatsheetseries.owasp.org/cheatsheets/Cryptographic_Storage_Cheat_Sheet.html#separation-of-keys-and-data).

Set `WIFI_PASSWORD_ENCRYPTION_KEY` to a base64-encoded random 32-byte key in the
deployment's encrypted secret. Keep it outside the database. Back up both the
encrypted database and this key: restoring one without the other cannot recover
passwords. Restore the original device UUIDs with the ciphertext. For SOPS
deployments, retain the identity needed to decrypt the secret. Keep the same
key across replicas and deployments. Do not regenerate
it during startup or development. Rotation requires re-encrypting stored records;
replacing the environment value alone makes existing ciphertext unreadable.
The backend requires this key at startup, like its existing JWT secret.

Apply the additive database migration, provision the backend key, then deploy the
API and frontend. Development against the deployed database uses its deployed key.

```sh
# Generate and save through stdin, without password arguments or terminal output.
internctl device password generate |
  internctl device update <id> --private-wifi-password --wifi-password
# Alternatively, prompt without echoing the password.
internctl device update <id> --private-wifi-password --wifi-password
internctl device update <id> --private-wifi-password=false
# These explicit actions print a password to stdout.
internctl device password generate
internctl device password show <id>
```

## RADIUS policy delivery

`GET /api/v1/radius/policy-snapshot` returns a versioned FreeRADIUS users file
with active device VLANs and enabled private passwords. It serves wired MAC
registration and PPSK from one inventory. Disabled devices and retained passwords
with private access off are excluded. Unknown devices use the shared password
configured at the RADIUS server.

Configure `RADIUS_SNAPSHOT_TOKEN_HASHES` as comma-separated `site=sha256` entries.
Hash the exact opaque bearer token supplied to the router. Each site has its own
token for independent attribution and revocation; every authorized site receives
the same inventory policy. An empty setting denies all downloads. Reusing a token
across sites is rejected. Download tokens cannot access human API routes or
password-reveal endpoints.

Set the required `RADIUS_SNAPSHOT_MAX_BYTES` environment variable to a positive
byte limit appropriate for the receiver transport. There is no numeric default
in the application. The device service handles inventory and decryption.
The HTTP handler formats the policy and enforces the configured byte limit.

Downloads use `Cache-Control: no-store`. The private-policy response and
its persistent cache, staging file, and rollback copy contain recoverable secrets.
Use HTTPS with certificate validation and protected, read-only container mounts.
FreeRADIUS holds private passwords in control attributes; only a successful PPSK
request copies the selected password into the encrypted `Tunnel-Password` reply.
Wired MAB, Enterprise Wi-Fi, operator authentication, and rejected requests do not
receive Wi-Fi passwords. Literal values are escaped for FreeRADIUS before parsing.

Revisions depend on effective policy, not inventory names or encryption nonces.
Unchanged polls need no disk write or restart. Policy responses at or above the
configured byte limit, unreadable active ciphertext, or unavailable inventory
return HTTP 503
without partial policy. The router retains its last-good file. Revoking a
download token stops updates; it does not erase already cached policy.

Consumers should retain the last working policy when a download fails. Changes
apply at a fresh RADIUS authorization; there is no automatic disconnect or CoA.

The optional integration check below exercises a real API/database policy with
an external native FreeRADIUS protocol test. The executable receives the runtime
image, policy file, and password file as positional arguments. It inherits the
caller's environment. All credentials in this check are synthetic.

```sh
RADIUS_NATIVE_TEST_SCRIPT=/path/to/radius-protocol-test \
RADIUS_NATIVE_TEST_IMAGE=<published-runtime-image> \
  go test -tags integration ./internal/httpserver -run TestHandlerIntegrationRADIUSPolicy -v
```

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

At startup, the helper reads `INTERN_API_DATABASE_URL`, the deployed Wi-Fi
encryption key, the backend snapshot limit, and the deployed metrics
ConfigMap from the current Kubernetes namespace. It rewrites the database host
to the local PostgreSQL port-forward and mounts the metrics definition from a
temporary file, without storing either value in `.env`. The temporary metrics
file is removed when the helper exits. Fresh local-only JWT and forward-auth
secrets are generated for every run. Docker administrators can still inspect
container environment variables and bind mounts.

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
stop and restart the helper so Compose reads the new values. Restart it after a
metrics ConfigMap update to load the deployed definition again.

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

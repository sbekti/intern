#!/usr/bin/env bash
set -euo pipefail

cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."

usage() {
  cat <<'EOF'
Usage: ./scripts/dev.sh [stop]

Starts the production service port-forwards and the Docker Compose watch stack
with the development identity configured in .env.

Options:
  stop        Stop the Intern Docker Compose development stack.
  -h, --help  Show this help.
EOF
}

if (($# > 1)); then
  printf 'Only one option may be provided.\n\n' >&2
  usage >&2
  exit 2
fi

case "${1:-}" in
  "") ;;
  stop)
    export INTERN_API_DATABASE_URL=postgres://unused
    export AUTH_JWT_HMAC_SECRET=unused
    export INTERN_DEV_IDENTITY_ENABLED=false
    export INTERN_DEV_IDENTITY_MARKER=unused
    docker compose --env-file .env.example down --remove-orphans
    exit 0
    ;;
  -h | --help)
    usage
    exit 0
    ;;
  *)
    printf 'Unknown option: %s\n\n' "$1" >&2
    usage >&2
    exit 2
    ;;
esac

if [[ ! -f .env ]]; then
  printf 'Missing .env. Create it with: cp .env.example .env\n' >&2
  exit 1
fi

export INTERN_DEV_IDENTITY_ENABLED=true
printf '%s\n' 'WARNING: Development identity is enabled.' >&2
printf '%s\n' 'Every client that can reach the web app can act as the configured user.' >&2

printf '%s\n' 'WARNING: This stack connects to production PostgreSQL and Prometheus.' >&2
printf '%s\n' 'Development actions can write production data.' >&2

if ! docker_gateway="$(docker network inspect bridge --format '{{(index .IPAM.Config 0).Gateway}}')"; then
  printf '%s\n' 'Could not inspect the Docker bridge network.' >&2
  exit 1
fi
if [[ -z "$docker_gateway" ]]; then
  printf '%s\n' 'Could not determine the Docker bridge gateway.' >&2
  exit 1
fi

printf 'Kubernetes context: %s\n' "$(kubectl config current-context)"

load_database_url() {
  local encoded

  if ! encoded="$(kubectl get secret intern-backend-secret -o 'jsonpath={.data.INTERN_API_DATABASE_URL}')"; then
    printf '%s\n' 'Could not read INTERN_API_DATABASE_URL from intern-backend-secret in the current namespace.' >&2
    exit 1
  fi
  if [[ -z "$encoded" ]]; then
    printf '%s\n' 'INTERN_API_DATABASE_URL is missing from intern-backend-secret.' >&2
    exit 1
  fi

  printf '%s' "$encoded" | base64 --decode
}

rewrite_database_url() {
  local database_url=$1 credentials host_and_path database_path

  case "$database_url" in
    postgres://*@*/* | postgresql://*@*/*) ;;
    *)
      printf '%s\n' 'The cluster database URL has an unexpected format.' >&2
      return 1
      ;;
  esac

  credentials="${database_url%%@*}"
  host_and_path="${database_url#*@}"
  database_path="${host_and_path#*/}"
  if [[ -z "$database_path" || "$database_path" == "$host_and_path" ]]; then
    printf '%s\n' 'The cluster database URL is missing its database path.' >&2
    return 1
  fi

  printf '%s@host.docker.internal:15432/%s' "$credentials" "$database_path"
}

cluster_database_url="$(load_database_url)"
export INTERN_API_DATABASE_URL="$(rewrite_database_url "$cluster_database_url")"
unset cluster_database_url
export AUTH_JWT_HMAC_SECRET="$(openssl rand -hex 32)"
export INTERN_DEV_IDENTITY_MARKER="$(openssl rand -hex 32)"

dev_runtime_dir="$(mktemp -d)"
dev_metrics_config="$dev_runtime_dir/metrics.yaml"
port_forward_pids=()

cleanup() {
  if ((${#port_forward_pids[@]} > 0)); then
    kill "${port_forward_pids[@]}" 2>/dev/null || true
    wait "${port_forward_pids[@]}" 2>/dev/null || true
  fi
  rm -f -- "$dev_metrics_config"
  rmdir -- "$dev_runtime_dir" 2>/dev/null || true
}

trap cleanup EXIT

if ! kubectl get configmap intern-frontend-metrics \
  -o 'jsonpath={.data.metrics\.yaml}' >"$dev_metrics_config"; then
  printf '%s\n' 'Could not read metrics.yaml from intern-frontend-metrics in the current namespace.' >&2
  exit 1
fi
if [[ ! -s "$dev_metrics_config" ]]; then
  printf '%s\n' 'metrics.yaml is missing from intern-frontend-metrics.' >&2
  exit 1
fi
chmod 755 "$dev_runtime_dir"
chmod 644 "$dev_metrics_config"
export INTERN_METRICS_CONFIG_SOURCE="$dev_metrics_config"
printf '%s\n' 'Production metrics configuration loaded.'

docker compose config --quiet

start_forward() {
  local label=$1 namespace=$2 service=$3 local_port=$4 remote_port=$5

  kubectl -n "$namespace" port-forward "service/$service" \
    "$local_port:$remote_port" --address "127.0.0.1,$docker_gateway" &
  local pid=$!
  port_forward_pids+=("$pid")

  for _ in {1..150}; do
    if ! kill -0 "$pid" 2>/dev/null; then
      printf '%s port-forward stopped before it became ready.\n' "$label" >&2
      return 1
    fi
    if (: >"/dev/tcp/127.0.0.1/$local_port") 2>/dev/null; then
      printf '%s port-forward is ready.\n' "$label"
      return 0
    fi
    sleep 0.2
  done

  printf 'Timed out waiting for the %s port-forward.\n' "$label" >&2
  return 1
}

start_forward PostgreSQL db postgres-rw 15432 5432
start_forward Prometheus monitoring prometheus-kube-prometheus-prometheus 19090 9090

docker compose up --build --watch --remove-orphans

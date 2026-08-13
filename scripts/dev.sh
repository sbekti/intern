#!/usr/bin/env bash
set -euo pipefail

cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."

usage() {
  cat <<'EOF'
Usage: ./scripts/dev.sh [--dev-identity]

Starts the production service port-forwards and the Docker Compose watch stack.

Options:
  --dev-identity  Inject the development user configured in .env.
  -h, --help      Show this help.
EOF
}

if (($# > 1)); then
  printf 'Only one option may be provided.\n\n' >&2
  usage >&2
  exit 2
fi

case "${1:-}" in
  "") dev_identity=false ;;
  --dev-identity) dev_identity=true ;;
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

if grep -q 'replace-with-' .env; then
  printf 'Replace every replace-with-... placeholder in .env before starting.\n' >&2
  exit 1
fi

export INTERN_DEV_IDENTITY_ENABLED="$dev_identity"
if [[ "$dev_identity" == true ]]; then
  printf '%s\n' 'WARNING: Development identity is enabled.' >&2
  printf '%s\n' 'Every client that can reach the web app can act as the configured user.' >&2
fi

printf '%s\n' 'WARNING: This stack connects to production PostgreSQL and Prometheus.' >&2
printf '%s\n' 'Development actions can write production data.' >&2

docker compose config --quiet

docker_gateway="$(docker network inspect bridge --format '{{(index .IPAM.Config 0).Gateway}}')"
if [[ -z "$docker_gateway" ]]; then
  printf 'Could not determine the Docker bridge gateway.\n' >&2
  exit 1
fi

port_forward_pids=()

cleanup() {
  ((${#port_forward_pids[@]} > 0)) || return
  kill "${port_forward_pids[@]}" 2>/dev/null || true
  wait "${port_forward_pids[@]}" 2>/dev/null || true
}

trap cleanup EXIT

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

printf 'Kubernetes context: %s\n' "$(kubectl config current-context)"

start_forward PostgreSQL db postgres-rw 15432 5432
start_forward Prometheus monitoring prometheus-kube-prometheus-prometheus 19090 9090

docker compose up --build --watch --remove-orphans

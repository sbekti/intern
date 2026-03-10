#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${REPO_DIR}"

docker compose -p intern-api-dev up --build -d

cat <<'EOF'
intern-api local stack is starting.

User proxy:
  http://localhost:18080

Admin proxy:
  http://localhost:18081
EOF

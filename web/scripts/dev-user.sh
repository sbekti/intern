#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
API_DIR="${REPO_DIR}/../intern-api"

"${API_DIR}/scripts/dev-up.sh"

cd "${REPO_DIR}"

export INTERN_API_BASE_URL="${INTERN_API_BASE_URL:-http://127.0.0.1:18080}"

exec npm run dev -- --hostname 0.0.0.0 --port 3000

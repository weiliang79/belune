#!/usr/bin/env bash
# Self-Hosted PaaS — Installer
# Usage: curl -sSL https://raw.githubusercontent.com/ungweiliang/selfhost-paas/main/scripts/install.sh | bash
set -euo pipefail

INSTALL_DIR="${PAAS_DIR:-/opt/paas}"
GITHUB_REPO="ungweiliang/selfhost-paas"
IMAGE="ghcr.io/${GITHUB_REPO}:latest"

# ── Helpers ────────────────────────────────────────────────────────────────────

info()    { echo "  [info]  $*"; }
success() { echo "  [ok]    $*"; }
die()     { echo "  [err]   $*" >&2; exit 1; }

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "'$1' is required but not installed. $2"
}

random_hex() {
  head -c 32 /dev/urandom | xxd -p | tr -d '\n'
}

# ── Pre-flight checks ──────────────────────────────────────────────────────────

echo ""
echo "  Self-Hosted PaaS — Installer"
echo "  =============================="
echo ""

require_cmd docker   "Install Docker: https://docs.docker.com/engine/install/"
require_cmd curl     "Install curl via your package manager."
require_cmd xxd      "Install xxd (package: vim-common or util-linux)."

if ! docker compose version >/dev/null 2>&1; then
  die "Docker Compose v2 is required. Update Docker Desktop or install the compose plugin."
fi

DOCKER_VERSION=$(docker version --format '{{.Server.Version}}' 2>/dev/null || echo "unknown")
info "Docker version: ${DOCKER_VERSION}"

# ── Create install directory ───────────────────────────────────────────────────

info "Creating install directory at ${INSTALL_DIR}..."
mkdir -p "${INSTALL_DIR}/infra/caddy/sites"
cd "${INSTALL_DIR}"

# ── Download Compose + Caddy configs ──────────────────────────────────────────

RAW_URL="https://raw.githubusercontent.com/${GITHUB_REPO}/main"

info "Downloading docker-compose.yml..."
curl -sSfL "${RAW_URL}/infra/docker-compose.prod.yml" -o docker-compose.yml

info "Downloading Caddyfile..."
curl -sSfL "${RAW_URL}/infra/caddy/Caddyfile.template" -o infra/caddy/Caddyfile.template

# ── Generate .env ──────────────────────────────────────────────────────────────

if [[ -f ".env" ]]; then
  info ".env already exists — skipping secret generation."
else
  info "Generating secure .env..."

  PG_PASS=$(random_hex)
  JWT_SECRET=$(random_hex)
  ENC_KEY=$(random_hex)
  INSTALL_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

  cat > .env <<EOF
# Self-Hosted PaaS — generated on ${INSTALL_DATE}

POSTGRES_USER=paas
POSTGRES_PASSWORD=${PG_PASS}
POSTGRES_DB=paas
DATABASE_URL=postgres://paas:${PG_PASS}@postgres:5432/paas?sslmode=disable

REDIS_URL=redis://redis:6379

JWT_SECRET=${JWT_SECRET}
JWT_EXPIRY_HOURS=24

ENCRYPTION_KEY=${ENC_KEY}

CADDY_ADMIN_URL=http://caddy:2019

PORT=8080
TLS_ENABLED=false
CORS_ORIGINS=http://localhost

PAAS_IMAGE=${IMAGE}
EOF

  success ".env written with generated secrets."
fi

# ── Pull and start ─────────────────────────────────────────────────────────────

info "Pulling latest image..."
docker pull "${IMAGE}" 2>/dev/null || info "Image pull failed — will attempt local build."

info "Starting services..."
docker compose up -d

# ── Wait for health ────────────────────────────────────────────────────────────

info "Waiting for PaaS to become ready..."
MAX_WAIT=60
ELAPSED=0
until curl -sf http://localhost:8080/healthz >/dev/null 2>&1; do
  sleep 2
  ELAPSED=$((ELAPSED + 2))
  if [[ ${ELAPSED} -ge ${MAX_WAIT} ]]; then
    die "API did not become ready within ${MAX_WAIT}s. Check: docker compose logs paas"
  fi
done

# ── Done ──────────────────────────────────────────────────────────────────────

echo ""
success "Installation complete!"
echo ""
echo "  Dashboard : http://localhost"
echo "  API health: http://localhost:8080/healthz"
echo "  Install dir: ${INSTALL_DIR}"
echo ""
echo "  Open http://localhost in your browser to finish setup."
echo "  Logs: cd ${INSTALL_DIR} && docker compose logs -f"
echo ""

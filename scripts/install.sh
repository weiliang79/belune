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
mkdir -p "${INSTALL_DIR}/infra/caddy/sites" "${INSTALL_DIR}/infra/systemd"
cd "${INSTALL_DIR}"

# ── Download Compose + Caddy configs ──────────────────────────────────────────

RAW_URL="https://raw.githubusercontent.com/${GITHUB_REPO}/main"

info "Downloading docker-compose.yml..."
curl -sSfL "${RAW_URL}/infra/docker-compose.prod.yml" -o docker-compose.yml

info "Downloading Caddyfile..."
curl -sSfL "${RAW_URL}/infra/caddy/Caddyfile.template" -o infra/caddy/Caddyfile.template

info "Downloading .env.defaults reference..."
curl -sSfL "${RAW_URL}/.env.defaults" -o .env.defaults

info "Downloading systemd unit..."
curl -sSfL "${RAW_URL}/infra/systemd/paas.service" -o infra/systemd/paas.service

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

# ── Extract helper binaries ────────────────────────────────────────────────────

# Copy paas-backup-upload out of the API image so backup.sh can run it on the
# host without requiring a running container. Re-extracted on every update.
mkdir -p "${INSTALL_DIR}/bin"
info "Extracting paas-backup-upload helper..."
docker run --rm --entrypoint="" "${IMAGE}" \
  cat /usr/local/bin/paas-backup-upload \
  > "${INSTALL_DIR}/bin/paas-backup-upload" 2>/dev/null \
  && chmod +x "${INSTALL_DIR}/bin/paas-backup-upload" \
  && success "paas-backup-upload installed at ${INSTALL_DIR}/bin/paas-backup-upload." \
  || info "paas-backup-upload not found in image — remote backup upload will not be available."

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

# ── Optional: install systemd unit so the stack starts on reboot ──────────────

# Only attempt this when systemd is the init system AND we're running as
# root. Anywhere else (macOS dev, rootless install) we just print the
# manual instructions and move on.
if [[ -d /run/systemd/system ]] && [[ ${EUID:-$(id -u)} -eq 0 ]]; then
  if [[ ! -f /etc/systemd/system/paas.service ]]; then
    info "Installing paas.service systemd unit..."
    # Patch WorkingDirectory if the operator chose a non-default path. The
    # bundled unit hard-codes /opt/paas; sed-rewrite it in place rather than
    # shipping a templated file users would have to render themselves.
    if [[ "${INSTALL_DIR}" != "/opt/paas" ]]; then
      sed "s|^WorkingDirectory=.*|WorkingDirectory=${INSTALL_DIR}|" \
        infra/systemd/paas.service > /etc/systemd/system/paas.service
    else
      cp infra/systemd/paas.service /etc/systemd/system/paas.service
    fi
    systemctl daemon-reload
    systemctl enable paas.service >/dev/null 2>&1 || true
    success "paas.service installed and enabled (auto-starts on reboot)."
  else
    info "/etc/systemd/system/paas.service already exists — skipping."
  fi

  # ── Backup timer ──────────────────────────────────────────────────────────────
  if [[ ! -f /etc/systemd/system/paas-backup.timer ]]; then
    info "Installing paas-backup.service and paas-backup.timer..."
    if [[ "${INSTALL_DIR}" != "/opt/paas" ]]; then
      sed "s|/opt/paas|${INSTALL_DIR}|g" \
        infra/systemd/paas-backup.service > /etc/systemd/system/paas-backup.service
    else
      cp infra/systemd/paas-backup.service /etc/systemd/system/paas-backup.service
    fi
    cp infra/systemd/paas-backup.timer /etc/systemd/system/paas-backup.timer
    systemctl daemon-reload
    systemctl enable --now paas-backup.timer >/dev/null 2>&1 || true
    success "paas-backup.timer installed and enabled (daily backups at 02:00)."
  else
    info "/etc/systemd/system/paas-backup.timer already exists — skipping."
  fi
else
  info "Skipping systemd install (not root or non-systemd host)."
  info "To enable auto-start on reboot:"
  info "  sudo cp ${INSTALL_DIR}/infra/systemd/paas.service /etc/systemd/system/"
  info "  sudo systemctl daemon-reload && sudo systemctl enable --now paas.service"
  info "To enable daily backups:"
  info "  sudo cp ${INSTALL_DIR}/infra/systemd/paas-backup.{service,timer} /etc/systemd/system/"
  info "  sudo systemctl daemon-reload && sudo systemctl enable --now paas-backup.timer"
fi

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
echo "  Next steps (DNS, TLS, first deploy):"
echo "    https://github.com/${GITHUB_REPO}/blob/main/docs/runbooks/install.md"
echo ""
echo "  Full config reference: ${INSTALL_DIR}/.env.defaults"
echo ""

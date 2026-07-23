#!/usr/bin/env bash
# Belune — Installer
# Usage: curl -sSL https://raw.githubusercontent.com/weiliang79/belune/main/scripts/install.sh | bash
set -euo pipefail

INSTALL_DIR="${BELUNE_DIR:-/opt/belune}"
GITHUB_REPO="weiliang79/belune"

# The version to install. Defaults to the latest published release; set
# BELUNE_VERSION=v0.1.0 to install a specific one.
#
# Installs are pinned rather than tracking :latest, so a routine `docker compose
# pull` can never move an install across a migration boundary on its own. Moving
# to a new version is update.sh's job, and it takes a backup first.
BELUNE_VERSION="${BELUNE_VERSION:-}"

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
echo "  Belune — Installer"
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
mkdir -p "${INSTALL_DIR}/infra/caddy/sites" "${INSTALL_DIR}/infra/systemd" "${INSTALL_DIR}/scripts"
cd "${INSTALL_DIR}"

# ── Resolve the version to install ────────────────────────────────────────────

if [[ -z "${BELUNE_VERSION}" ]]; then
  info "Resolving the latest release..."
  BELUNE_VERSION=$(curl -sSfL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" \
    | grep '"tag_name"' | head -1 | cut -d'"' -f4 || true)
  [[ -n "${BELUNE_VERSION}" ]] || die "Could not resolve the latest release. Set BELUNE_VERSION=vX.Y.Z and retry."
fi

IMAGE="ghcr.io/${GITHUB_REPO}:${BELUNE_VERSION}"
info "Installing ${BELUNE_VERSION}"

# ── Download Compose + Caddy configs ──────────────────────────────────────────

# Fetched from the release tag, not main: config files must match the image
# being installed, and main moves on between releases.
RAW_URL="https://raw.githubusercontent.com/${GITHUB_REPO}/${BELUNE_VERSION}"

info "Downloading docker-compose.yml..."
curl -sSfL "${RAW_URL}/infra/docker-compose.prod.yml" -o docker-compose.yml

info "Downloading Caddyfile..."
curl -sSfL "${RAW_URL}/infra/caddy/Caddyfile.template" -o infra/caddy/Caddyfile.template

info "Downloading .env.example reference..."
curl -sSfL "${RAW_URL}/.env.example" -o .env.example

info "Downloading systemd units..."
curl -sSfL "${RAW_URL}/infra/systemd/belune.service" -o infra/systemd/belune.service
# belune-backup.{service,timer} are copied into /etc/systemd/system below; the
# service's ExecStart runs scripts/backup.sh from the install dir. All three were
# referenced by the systemd step but never fetched, so a root+systemd install
# died on the missing file — after the stack was already up.
curl -sSfL "${RAW_URL}/infra/systemd/belune-backup.service" -o infra/systemd/belune-backup.service
curl -sSfL "${RAW_URL}/infra/systemd/belune-backup.timer" -o infra/systemd/belune-backup.timer

info "Downloading host scripts..."
# backup.sh is run by belune-backup.service (and by update.sh before it moves the
# version); restore.sh is the disaster-recovery entry point the runbook invokes as
# /opt/belune/scripts/restore.sh; update.sh performs a version move with a backup.
# None were installed before, so the whole host-side backup/restore/update path
# was absent on a real install.
for s in backup.sh restore.sh update.sh; do
  curl -sSfL "${RAW_URL}/scripts/${s}" -o "scripts/${s}"
  chmod +x "scripts/${s}"
done

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
# Belune — generated on ${INSTALL_DATE}

POSTGRES_USER=belune
POSTGRES_PASSWORD=${PG_PASS}
POSTGRES_DB=belune
DATABASE_URL=postgres://belune:${PG_PASS}@postgres:5432/belune?sslmode=disable

REDIS_URL=redis://redis:6379

JWT_SECRET=${JWT_SECRET}

ENCRYPTION_KEY=${ENC_KEY}

CADDY_ADMIN_URL=http://caddy:2019

PORT=8080
TLS_ENABLED=false
CORS_ORIGINS=http://localhost

BELUNE_IMAGE=${IMAGE}
EOF

  success ".env written with generated secrets."
fi

# ── Pull and start ─────────────────────────────────────────────────────────────

info "Pulling ${IMAGE}..."
docker pull "${IMAGE}" || die "Could not pull ${IMAGE}. Check the version exists and the host can reach ghcr.io."

info "Starting services..."
docker compose up -d

# ── Extract helper binaries ────────────────────────────────────────────────────

# Copy belune-backup-upload out of the API image so backup.sh can run it on the
# host without requiring a running container. Re-extracted on every update.
mkdir -p "${INSTALL_DIR}/bin"
info "Extracting belune-backup-upload helper..."
docker run --rm --entrypoint="" "${IMAGE}" \
  cat /usr/local/bin/belune-backup-upload \
  > "${INSTALL_DIR}/bin/belune-backup-upload" 2>/dev/null \
  && chmod +x "${INSTALL_DIR}/bin/belune-backup-upload" \
  && success "belune-backup-upload installed at ${INSTALL_DIR}/bin/belune-backup-upload." \
  || info "belune-backup-upload not found in image — remote backup upload will not be available."

# ── Wait for health ────────────────────────────────────────────────────────────

info "Waiting for Belune to become ready..."
MAX_WAIT=60
ELAPSED=0
until curl -sf http://localhost:8080/healthz >/dev/null 2>&1; do
  sleep 2
  ELAPSED=$((ELAPSED + 2))
  if [[ ${ELAPSED} -ge ${MAX_WAIT} ]]; then
    die "API did not become ready within ${MAX_WAIT}s. Check: docker compose logs belune"
  fi
done

# ── Optional: install systemd unit so the stack starts on reboot ──────────────

# Only attempt this when systemd is the init system AND we're running as
# root. Anywhere else (macOS dev, rootless install) we just print the
# manual instructions and move on.
if [[ -d /run/systemd/system ]] && [[ ${EUID:-$(id -u)} -eq 0 ]]; then
  if [[ ! -f /etc/systemd/system/belune.service ]]; then
    info "Installing belune.service systemd unit..."
    # Patch WorkingDirectory if the operator chose a non-default path. The
    # bundled unit hard-codes /opt/belune; sed-rewrite it in place rather than
    # shipping a templated file users would have to render themselves.
    if [[ "${INSTALL_DIR}" != "/opt/belune" ]]; then
      sed "s|^WorkingDirectory=.*|WorkingDirectory=${INSTALL_DIR}|" \
        infra/systemd/belune.service > /etc/systemd/system/belune.service
    else
      cp infra/systemd/belune.service /etc/systemd/system/belune.service
    fi
    systemctl daemon-reload
    systemctl enable belune.service >/dev/null 2>&1 || true
    success "belune.service installed and enabled (auto-starts on reboot)."
  else
    info "/etc/systemd/system/belune.service already exists — skipping."
  fi

  # ── Backup timer ──────────────────────────────────────────────────────────────
  if [[ ! -f /etc/systemd/system/belune-backup.timer ]]; then
    info "Installing belune-backup.service and belune-backup.timer..."
    if [[ "${INSTALL_DIR}" != "/opt/belune" ]]; then
      sed "s|/opt/belune|${INSTALL_DIR}|g" \
        infra/systemd/belune-backup.service > /etc/systemd/system/belune-backup.service
    else
      cp infra/systemd/belune-backup.service /etc/systemd/system/belune-backup.service
    fi
    cp infra/systemd/belune-backup.timer /etc/systemd/system/belune-backup.timer
    systemctl daemon-reload
    systemctl enable --now belune-backup.timer >/dev/null 2>&1 || true
    success "belune-backup.timer installed and enabled (daily backups at 02:00)."
  else
    info "/etc/systemd/system/belune-backup.timer already exists — skipping."
  fi
else
  info "Skipping systemd install (not root or non-systemd host)."
  info "To enable auto-start on reboot:"
  info "  sudo cp ${INSTALL_DIR}/infra/systemd/belune.service /etc/systemd/system/"
  info "  sudo systemctl daemon-reload && sudo systemctl enable --now belune.service"
  info "To enable daily backups:"
  info "  sudo cp ${INSTALL_DIR}/infra/systemd/belune-backup.{service,timer} /etc/systemd/system/"
  info "  sudo systemctl daemon-reload && sudo systemctl enable --now belune-backup.timer"
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
echo "  Options you can set:    ${INSTALL_DIR}/.env.example"
echo "  Full reference:         https://github.com/${GITHUB_REPO}/blob/main/docs/configuration.md"
echo ""

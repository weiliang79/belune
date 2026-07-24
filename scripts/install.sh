#!/usr/bin/env bash
# Belune — Installer
# Usage: curl -sSL https://raw.githubusercontent.com/weiliang79/belune/refs/heads/main/scripts/install.sh | bash
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

# 32 random bytes as hex. openssl first because it is on essentially every
# server; xxd comes from vim-common and is genuinely absent from minimal cloud
# images, so requiring it outright turned a fresh VPS into a package hunt.
random_hex() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 32
  elif command -v xxd >/dev/null 2>&1; then
    head -c 32 /dev/urandom | xxd -p | tr -d '\n'
  else
    od -An -tx1 -N32 /dev/urandom | tr -d ' \n'
  fi
}

# Docker's official convenience script. Every comparable installer uses it: it
# handles the per-distro repository setup and ships the Compose v2 plugin, which
# a distro's own docker.io package does not.
install_docker() {
  info "Installing Docker via https://get.docker.com ..."
  local tmp
  tmp="$(mktemp)"
  curl -fsSL https://get.docker.com -o "${tmp}" \
    || die "Could not download the Docker install script. Install Docker manually: https://docs.docker.com/engine/install/"
  sh "${tmp}" \
    || { rm -f "${tmp}"; die "Docker installation failed. Install it manually: https://docs.docker.com/engine/install/"; }
  rm -f "${tmp}"
  if [[ -d /run/systemd/system ]]; then
    systemctl enable --now docker >/dev/null 2>&1 || true
  fi
}

# ── Pre-flight checks ──────────────────────────────────────────────────────────

echo ""
echo "  Belune — Installer"
echo "  =============================="
echo ""

require_cmd curl "Install curl via your package manager."

# Docker is installed rather than demanded. A fresh VPS almost never has it, and
# stopping to say so just sends the operator away to run someone else's install
# script and come back. Set BELUNE_SKIP_DOCKER_INSTALL=1 to manage Docker
# yourself and have this only check.
if ! command -v docker >/dev/null 2>&1; then
  if [[ "${BELUNE_SKIP_DOCKER_INSTALL:-0}" == "1" ]]; then
    die "Docker is not installed (BELUNE_SKIP_DOCKER_INSTALL=1). Install it: https://docs.docker.com/engine/install/"
  fi
  if [[ "$(uname -s)" != "Linux" ]]; then
    die "Docker is not installed. On macOS install Docker Desktop or OrbStack, then re-run."
  fi
  if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
    die "Docker is not installed, and installing it needs root. Re-run with sudo, or install Docker first: https://docs.docker.com/engine/install/"
  fi
  info "Docker is not installed."
  install_docker
  command -v docker >/dev/null 2>&1 || die "Docker still not found after installation."
  success "Docker installed."
fi

# Installed but not running is its own failure: the next docker command fails
# with something far less obvious than "the daemon is stopped".
if ! docker info >/dev/null 2>&1; then
  if [[ -d /run/systemd/system ]] && [[ ${EUID:-$(id -u)} -eq 0 ]]; then
    info "Docker is installed but not running — starting it..."
    systemctl start docker >/dev/null 2>&1 || true
  fi
  docker info >/dev/null 2>&1 \
    || die "Docker is installed but the daemon is not reachable. Start it (sudo systemctl start docker) and re-run."
fi

if ! docker compose version >/dev/null 2>&1; then
  die "Docker Compose v2 is required but not available.
          If Docker came from a distro package (docker.io), add the plugin:
            sudo apt-get install -y docker-compose-plugin
          Or install Docker from https://get.docker.com, which includes it."
fi

DOCKER_VERSION=$(docker version --format '{{.Server.Version}}' 2>/dev/null || echo "unknown")
info "Docker version: ${DOCKER_VERSION}"

# ── Create install directory ───────────────────────────────────────────────────

# The layout mirrors the repo's, because docker-compose.prod.yml's relative
# volume paths (./infra/caddy, ./infra/buildkit) resolve against whichever
# directory holds the compose file — here, the install root.
info "Creating install directory at ${INSTALL_DIR}..."
mkdir -p "${INSTALL_DIR}/infra/caddy/sites" "${INSTALL_DIR}/infra/buildkit" \
         "${INSTALL_DIR}/infra/systemd" "${INSTALL_DIR}/scripts"
cd "${INSTALL_DIR}"

# ── Resolve the version to install ────────────────────────────────────────────

if [[ -z "${BELUNE_VERSION}" ]]; then
  info "Resolving the latest release..."
  BELUNE_VERSION=$(curl -sSfL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" \
    | grep '"tag_name"' | head -1 | cut -d'"' -f4 || true)
  [[ -n "${BELUNE_VERSION}" ]] || die "Could not resolve the latest release. Set BELUNE_VERSION=vX.Y.Z and retry."
fi

# A release has two names, and they are not the same string. The git tag keeps
# the leading v (v0.1.0) because that is the tag convention; the image tag drops
# it (0.1.0) because docker/metadata-action normalises the semver and Docker tags
# conventionally have no v — postgres:16, not postgres:v16. Conflating them pulls
# a reference that was never published. Either form is accepted here.
BELUNE_VERSION="${BELUNE_VERSION#v}"
GIT_REF="v${BELUNE_VERSION}"

IMAGE="ghcr.io/${GITHUB_REPO}:${BELUNE_VERSION}"
info "Installing ${GIT_REF}"

# ── Download Compose + Caddy configs ──────────────────────────────────────────

# Fetched from the release tag, not main: config files must match the image
# being installed, and main moves on between releases. This one wants the git
# ref, so it keeps the v.
RAW_URL="https://raw.githubusercontent.com/${GITHUB_REPO}/${GIT_REF}"

info "Downloading docker-compose.yml..."
curl -sSfL "${RAW_URL}/infra/docker-compose.prod.yml" -o docker-compose.yml

info "Downloading Caddyfile..."
curl -sSfL "${RAW_URL}/infra/caddy/Caddyfile.template" -o infra/caddy/Caddyfile.template

# The buildkit daemon is started with --config pointing at this file. When it is
# absent Docker helpfully creates a *directory* at the bind source, and buildkit
# exits — taking every git-based deploy with it, on a stack that otherwise looks
# healthy.
info "Downloading BuildKit config..."
curl -sSfL "${RAW_URL}/infra/buildkit/buildkitd.toml" -o infra/buildkit/buildkitd.toml

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

  # Secrets are kept, but the pinned image is not silently kept as well: it is
  # what Compose actually starts, so an install that pulls one version and boots
  # another is worse than a refusal. Moving an existing install across versions
  # is update.sh's job — it backs up first, which this script does not.
  EXISTING_IMAGE=$(grep '^BELUNE_IMAGE=' .env | head -1 | cut -d= -f2- || true)
  if [[ -n "${EXISTING_IMAGE}" && "${EXISTING_IMAGE}" != "${IMAGE}" ]]; then
    die "This install is already pinned to ${EXISTING_IMAGE}, but you asked for ${IMAGE}.
          To move an existing install to a new version (takes a backup first):
            cd ${INSTALL_DIR} && sudo bash scripts/update.sh ${GIT_REF}
          To start over and discard its data:
            docker compose -f ${INSTALL_DIR}/docker-compose.yml down -v && rm -rf ${INSTALL_DIR}"
  fi
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

# ── Docker socket group ────────────────────────────────────────────────────────

# Belune runs as a non-root user, so mounting the Docker socket is not enough:
# the container must also be in the group that OWNS the socket, and that GID
# differs per host (Debian/Ubuntu usually 999, but not always — a real VPS came
# up 988). Compose falls back to 999, and when the guess is wrong Belune starts,
# serves the UI, and can do nothing at all: every deploy fails and /healthz
# reports 503 because the Docker check cannot connect.
#
# Written outside the block above so an existing .env from an earlier install
# gets it too.
if ! grep -q '^DOCKER_GID=' .env 2>/dev/null; then
  DOCKER_GID=$(stat -c '%g' /var/run/docker.sock 2>/dev/null || true)
  [[ -n "${DOCKER_GID}" ]] || DOCKER_GID=$(getent group docker 2>/dev/null | cut -d: -f3 || true)
  if [[ -n "${DOCKER_GID}" ]]; then
    printf '\n# GID owning /var/run/docker.sock on this host.\nDOCKER_GID=%s\n' "${DOCKER_GID}" >> .env
    info "Detected Docker socket group ${DOCKER_GID}."
  else
    info "Could not detect the Docker socket group — falling back to 999."
  fi
fi

# ── CPU limits ─────────────────────────────────────────────────────────────────

# The compose file caps each service's CPU. Docker rejects a hard cpus cap that
# exceeds the host's core count outright — "range of CPUs is from 0.01 to 1.00,
# as there are only 1 CPUs available" — so the default 2.0 cap on belune could
# not even create the container on a 1-vCPU VPS. Clamp each cap to nproc: a small
# box gets a fitting ceiling, a large box keeps the headroom.
if ! grep -q '^BELUNE_CPU_LIMIT=' .env 2>/dev/null; then
  NPROC=$(nproc 2>/dev/null || getconf _NPROCESSORS_ONLN 2>/dev/null || echo 1)
  clamp_cpu() { awk -v d="$1" -v n="$2" 'BEGIN{ print (d < n ? d : n) }'; }
  {
    echo ""
    echo "# CPU ceilings, clamped to this host's ${NPROC} core(s) at install time."
    echo "BELUNE_CPU_LIMIT=$(clamp_cpu 2.0 "${NPROC}")"
    echo "POSTGRES_CPU_LIMIT=$(clamp_cpu 1.0 "${NPROC}")"
    echo "REDIS_CPU_LIMIT=$(clamp_cpu 0.5 "${NPROC}")"
    echo "CADDY_CPU_LIMIT=$(clamp_cpu 1.0 "${NPROC}")"
  } >> .env
  info "Clamped CPU limits to ${NPROC} core(s)."
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

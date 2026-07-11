#!/usr/bin/env bash
# Belune — Updater
# Run from the install directory: bash update.sh
set -euo pipefail

INSTALL_DIR="${BELUNE_DIR:-/opt/belune}"

info()    { echo "  [info]  $*"; }
success() { echo "  [ok]    $*"; }
die()     { echo "  [err]   $*" >&2; exit 1; }

[[ -f "${INSTALL_DIR}/docker-compose.yml" ]] || \
  die "No docker-compose.yml found at ${INSTALL_DIR}. Is Belune installed?"

cd "${INSTALL_DIR}"

echo ""
echo "  Belune — Updater"
echo "  ============================"
echo ""

# ── Pull latest image ──────────────────────────────────────────────────────────

BELUNE_IMAGE=$(grep 'BELUNE_IMAGE' .env 2>/dev/null | cut -d= -f2 || echo "ghcr.io/weiliang79/belune:latest")
info "Pulling ${BELUNE_IMAGE}..."
docker pull "${BELUNE_IMAGE}"

# ── Restart belune service (migrations run automatically on startup) ────────────

info "Restarting belune container..."
docker compose up -d --no-deps belune

# ── Re-extract helper binaries ─────────────────────────────────────────────────

BELUNE_IMAGE=$(grep 'BELUNE_IMAGE' .env 2>/dev/null | cut -d= -f2 || echo "ghcr.io/weiliang79/belune:latest")
mkdir -p "${INSTALL_DIR}/bin"
info "Re-extracting belune-backup-upload helper..."
docker run --rm --entrypoint="" "${BELUNE_IMAGE}" \
  cat /usr/local/bin/belune-backup-upload \
  > "${INSTALL_DIR}/bin/belune-backup-upload" 2>/dev/null \
  && chmod +x "${INSTALL_DIR}/bin/belune-backup-upload" \
  && success "belune-backup-upload updated." \
  || info "belune-backup-upload not found in image — skipping."

# ── Wait for health ────────────────────────────────────────────────────────────

info "Waiting for Belune to become ready..."
MAX_WAIT=60
ELAPSED=0
until curl -sf http://localhost:8080/healthz >/dev/null 2>&1; do
  sleep 2
  ELAPSED=$((ELAPSED + 2))
  if [[ ${ELAPSED} -ge ${MAX_WAIT} ]]; then
    die "API did not become ready after update. Check: docker compose logs belune"
  fi
done

echo ""
success "Update complete!"
echo ""

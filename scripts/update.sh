#!/usr/bin/env bash
# Self-Hosted PaaS — Updater
# Run from the install directory: bash update.sh
set -euo pipefail

INSTALL_DIR="${PAAS_DIR:-/opt/paas}"

info()    { echo "  [info]  $*"; }
success() { echo "  [ok]    $*"; }
die()     { echo "  [err]   $*" >&2; exit 1; }

[[ -f "${INSTALL_DIR}/docker-compose.yml" ]] || \
  die "No docker-compose.yml found at ${INSTALL_DIR}. Is PaaS installed?"

cd "${INSTALL_DIR}"

echo ""
echo "  Self-Hosted PaaS — Updater"
echo "  ============================"
echo ""

# ── Pull latest image ──────────────────────────────────────────────────────────

PAAS_IMAGE=$(grep 'PAAS_IMAGE' .env 2>/dev/null | cut -d= -f2 || echo "ghcr.io/ungweiliang/selfhost-paas:latest")
info "Pulling ${PAAS_IMAGE}..."
docker pull "${PAAS_IMAGE}"

# ── Restart paas service (migrations run automatically on startup) ────────────

info "Restarting paas container..."
docker compose up -d --no-deps paas

# ── Re-extract helper binaries ─────────────────────────────────────────────────

PAAS_IMAGE=$(grep 'PAAS_IMAGE' .env 2>/dev/null | cut -d= -f2 || echo "ghcr.io/ungweiliang/selfhost-paas:latest")
mkdir -p "${INSTALL_DIR}/bin"
info "Re-extracting paas-backup-upload helper..."
docker run --rm --entrypoint="" "${PAAS_IMAGE}" \
  cat /usr/local/bin/paas-backup-upload \
  > "${INSTALL_DIR}/bin/paas-backup-upload" 2>/dev/null \
  && chmod +x "${INSTALL_DIR}/bin/paas-backup-upload" \
  && success "paas-backup-upload updated." \
  || info "paas-backup-upload not found in image — skipping."

# ── Wait for health ────────────────────────────────────────────────────────────

info "Waiting for PaaS to become ready..."
MAX_WAIT=60
ELAPSED=0
until curl -sf http://localhost:8080/healthz >/dev/null 2>&1; do
  sleep 2
  ELAPSED=$((ELAPSED + 2))
  if [[ ${ELAPSED} -ge ${MAX_WAIT} ]]; then
    die "API did not become ready after update. Check: docker compose logs paas"
  fi
done

echo ""
success "Update complete!"
echo ""

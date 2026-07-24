#!/usr/bin/env bash
# Belune — Updater
#
# Usage, from anywhere:
#   bash update.sh              # update to the latest release
#   bash update.sh v0.2.0       # update to a specific version
#
# An update is a deliberate version move, never a drift: it resolves a target
# version, takes a backup before touching anything, rewrites the pinned image in
# .env, and tells you exactly how to roll back if the new version does not come
# up. Migrations run automatically at boot and are not reversible, which is why
# the backup happens first.
set -euo pipefail

INSTALL_DIR="${BELUNE_DIR:-/opt/belune}"
GITHUB_REPO="weiliang79/belune"

info()    { echo "  [info]  $*"; }
success() { echo "  [ok]    $*"; }
warn()    { echo "  [warn]  $*"; }
die()     { echo "  [err]   $*" >&2; exit 1; }

[[ -f "${INSTALL_DIR}/docker-compose.yml" ]] || \
  die "No docker-compose.yml found at ${INSTALL_DIR}. Is Belune installed?"

cd "${INSTALL_DIR}"

echo ""
echo "  Belune — Updater"
echo "  ============================"
echo ""

# ── Resolve current and target versions ────────────────────────────────────────

current_image() {
  grep '^BELUNE_IMAGE=' .env 2>/dev/null | head -1 | cut -d= -f2- || true
}

CURRENT_IMAGE=$(current_image)
[[ -n "${CURRENT_IMAGE}" ]] || die "No BELUNE_IMAGE in .env — cannot tell what is installed."
CURRENT_VERSION="${CURRENT_IMAGE##*:}"

TARGET_VERSION="${1:-}"
if [[ -z "${TARGET_VERSION}" ]]; then
  info "Resolving the latest release..."
  TARGET_VERSION=$(curl -sSfL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" \
    | grep '"tag_name"' | head -1 | cut -d'"' -f4 || true)
  [[ -n "${TARGET_VERSION}" ]] || die "Could not resolve the latest release. Pass a version: bash update.sh v0.2.0"
fi

# Git tags carry a leading v, image tags do not (see install.sh). CURRENT_VERSION
# is read back off an image reference, so it never has one — without normalising
# here, "already on this version" could not match even when it was true, and the
# pull below would ask for a tag that was never published.
TARGET_VERSION="${TARGET_VERSION#v}"

TARGET_IMAGE="ghcr.io/${GITHUB_REPO}:${TARGET_VERSION}"

info "Currently installed: ${CURRENT_VERSION}"
info "Updating to:         ${TARGET_VERSION}"

if [[ "${CURRENT_VERSION}" == "${TARGET_VERSION}" ]]; then
  success "Already on ${TARGET_VERSION} — nothing to do."
  exit 0
fi

# Pull first: no point backing up and stopping anything if the image is not
# there to move to.
info "Pulling ${TARGET_IMAGE}..."
docker pull "${TARGET_IMAGE}" || die "Could not pull ${TARGET_IMAGE}. Does that version exist?"

# ── Back up before migrating ───────────────────────────────────────────────────

# 0.x minor releases may contain breaking changes, and migrations are
# forward-only: this backup is the rollback path for the data, while the image
# tag below is the rollback path for the code.
if [[ -x "${INSTALL_DIR}/scripts/backup.sh" ]] || [[ -f "${INSTALL_DIR}/scripts/backup.sh" ]]; then
  info "Taking a pre-update backup..."
  if bash "${INSTALL_DIR}/scripts/backup.sh"; then
    success "Backup complete."
  else
    warn "Backup failed."
    read -rp "  Continue updating without a backup? [y/N] " reply
    [[ "${reply}" =~ ^[Yy]$ ]] || die "Aborted. Nothing has changed."
  fi
else
  warn "scripts/backup.sh not found — no pre-update backup was taken."
  read -rp "  Continue without a backup? [y/N] " reply
  [[ "${reply}" =~ ^[Yy]$ ]] || die "Aborted. Nothing has changed."
fi

# ── Move the pin ───────────────────────────────────────────────────────────────

cp .env ".env.backup-${CURRENT_VERSION}"
if grep -q '^BELUNE_IMAGE=' .env; then
  # Portable in-place edit: GNU and BSD sed disagree about -i.
  sed "s|^BELUNE_IMAGE=.*|BELUNE_IMAGE=${TARGET_IMAGE}|" .env > .env.tmp && mv .env.tmp .env
else
  echo "BELUNE_IMAGE=${TARGET_IMAGE}" >> .env
fi
success "Pinned to ${TARGET_VERSION}."

# ── Restart (migrations run automatically on startup) ──────────────────────────

rollback_hint() {
  echo ""
  echo "  To roll back to ${CURRENT_VERSION}:"
  echo ""
  echo "      cd ${INSTALL_DIR}"
  echo "      sed -i 's|^BELUNE_IMAGE=.*|BELUNE_IMAGE=${CURRENT_IMAGE}|' .env"
  echo "      docker compose up -d --no-deps belune"
  echo ""
  echo "  If the new version already applied migrations, restore the pre-update"
  echo "  backup as well — see docs/runbooks/disaster-recovery.md."
  echo ""
}

info "Restarting belune..."
if ! docker compose up -d --no-deps belune; then
  rollback_hint
  die "Failed to start ${TARGET_VERSION}."
fi

# ── Re-extract helper binaries ─────────────────────────────────────────────────

mkdir -p "${INSTALL_DIR}/bin"
info "Re-extracting belune-backup-upload helper..."
docker run --rm --entrypoint="" "${TARGET_IMAGE}" \
  cat /usr/local/bin/belune-backup-upload \
  > "${INSTALL_DIR}/bin/belune-backup-upload" 2>/dev/null \
  && chmod +x "${INSTALL_DIR}/bin/belune-backup-upload" \
  && success "belune-backup-upload updated." \
  || info "belune-backup-upload not found in image — skipping."

# ── Wait for health ────────────────────────────────────────────────────────────

info "Waiting for Belune to become ready..."
MAX_WAIT=90
ELAPSED=0
until curl -sf http://localhost:8080/healthz >/dev/null 2>&1; do
  sleep 2
  ELAPSED=$((ELAPSED + 2))
  if [[ ${ELAPSED} -ge ${MAX_WAIT} ]]; then
    echo ""
    warn "API did not become ready after ${MAX_WAIT}s."
    echo "  Check the logs:  docker compose logs --tail=50 belune"
    rollback_hint
    exit 1
  fi
done

echo ""
success "Updated ${CURRENT_VERSION} → ${TARGET_VERSION}"
info "Previous .env saved as .env.backup-${CURRENT_VERSION}"
echo ""

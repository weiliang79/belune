#!/usr/bin/env bash
# Self-Hosted PaaS — Backup
# Creates a timestamped backup of Postgres data and Caddy TLS certs.
# Usage: bash backup.sh [output-dir]
set -euo pipefail

INSTALL_DIR="${PAAS_DIR:-/opt/paas}"
BACKUP_DIR="${1:-${INSTALL_DIR}/backups}"
TIMESTAMP=$(date -u +"%Y%m%dT%H%M%SZ")
BACKUP_NAME="paas-backup-${TIMESTAMP}"
WORK_DIR="${BACKUP_DIR}/${BACKUP_NAME}"

info()    { echo "  [info]  $*"; }
success() { echo "  [ok]    $*"; }
die()     { echo "  [err]   $*" >&2; exit 1; }

[[ -f "${INSTALL_DIR}/docker-compose.yml" ]] || \
  die "No docker-compose.yml found at ${INSTALL_DIR}. Is PaaS installed?"

cd "${INSTALL_DIR}"

echo ""
echo "  Self-Hosted PaaS — Backup"
echo "  ==========================="
echo ""

mkdir -p "${WORK_DIR}"

# ── Postgres dump ──────────────────────────────────────────────────────────────

info "Dumping Postgres database..."
DB_CONTAINER=$(docker compose ps -q postgres 2>/dev/null) || die "Postgres container not running."
PG_USER=$(grep 'POSTGRES_USER' .env 2>/dev/null | cut -d= -f2 || echo "paas")
PG_DB=$(grep 'POSTGRES_DB'   .env 2>/dev/null | cut -d= -f2 || echo "paas")

docker exec "${DB_CONTAINER}" \
  pg_dump -U "${PG_USER}" -d "${PG_DB}" --no-password \
  > "${WORK_DIR}/postgres.sql"
success "Postgres dump written to ${WORK_DIR}/postgres.sql"

# ── Caddy TLS data ─────────────────────────────────────────────────────────────

info "Backing up Caddy TLS data..."
# caddydata volume → /data inside the caddy container
CADDY_CONTAINER=$(docker compose ps -q caddy 2>/dev/null) || true
if [[ -n "${CADDY_CONTAINER}" ]]; then
  docker exec "${CADDY_CONTAINER}" tar -czf - /data /config 2>/dev/null \
    > "${WORK_DIR}/caddy-data.tar.gz" || info "Caddy data backup skipped (no data yet)."
  success "Caddy data written to ${WORK_DIR}/caddy-data.tar.gz"
else
  info "Caddy container not running — skipping TLS backup."
fi

# ── .env ──────────────────────────────────────────────────────────────────────

info "Copying .env..."
cp .env "${WORK_DIR}/.env"
success ".env backed up."

# ── Archive ────────────────────────────────────────────────────────────────────

ARCHIVE="${BACKUP_DIR}/${BACKUP_NAME}.tar.gz"
info "Creating archive ${ARCHIVE}..."
tar -czf "${ARCHIVE}" -C "${BACKUP_DIR}" "${BACKUP_NAME}"
rm -rf "${WORK_DIR}"

echo ""
success "Backup complete: ${ARCHIVE}"
echo ""

#!/usr/bin/env bash
# Self-Hosted PaaS — Restore
# Restores a backup created by backup.sh.
# Usage: bash restore.sh <backup.tar.gz>
set -euo pipefail

INSTALL_DIR="${PAAS_DIR:-/opt/paas}"
BACKUP_ARCHIVE="${1:-}"

info()    { echo "  [info]  $*"; }
success() { echo "  [ok]    $*"; }
die()     { echo "  [err]   $*" >&2; exit 1; }

[[ -n "${BACKUP_ARCHIVE}" ]] || die "Usage: $0 <backup.tar.gz>"
[[ -f "${BACKUP_ARCHIVE}" ]] || die "Backup file not found: ${BACKUP_ARCHIVE}"
[[ -f "${INSTALL_DIR}/docker-compose.yml" ]] || \
  die "No docker-compose.yml found at ${INSTALL_DIR}. Is PaaS installed?"

cd "${INSTALL_DIR}"

echo ""
echo "  Self-Hosted PaaS — Restore"
echo "  ============================"
echo "  Source: ${BACKUP_ARCHIVE}"
echo ""

# ── Extract archive ────────────────────────────────────────────────────────────

WORK_DIR=$(mktemp -d)
trap 'rm -rf "${WORK_DIR}"' EXIT

info "Extracting backup..."
tar -xzf "${BACKUP_ARCHIVE}" -C "${WORK_DIR}"
BACKUP_DIR=$(find "${WORK_DIR}" -mindepth 1 -maxdepth 1 -type d | head -1)
[[ -n "${BACKUP_DIR}" ]] || die "Could not find backup directory inside archive."

# ── Restore .env ──────────────────────────────────────────────────────────────

if [[ -f "${BACKUP_DIR}/.env" ]]; then
  info "Restoring .env..."
  cp "${BACKUP_DIR}/.env" .env
  success ".env restored."
fi

# ── Restore Postgres ──────────────────────────────────────────────────────────

if [[ -f "${BACKUP_DIR}/postgres.sql" ]]; then
  info "Restoring Postgres database..."
  DB_CONTAINER=$(docker compose ps -q postgres 2>/dev/null) || die "Postgres container not running."
  PG_USER=$(grep 'POSTGRES_USER' .env 2>/dev/null | cut -d= -f2 || echo "paas")
  PG_DB=$(grep 'POSTGRES_DB'   .env 2>/dev/null | cut -d= -f2 || echo "paas")

  # Drop and recreate the database to ensure a clean restore
  docker exec "${DB_CONTAINER}" \
    psql -U "${PG_USER}" -c "DROP DATABASE IF EXISTS ${PG_DB};" postgres
  docker exec "${DB_CONTAINER}" \
    psql -U "${PG_USER}" -c "CREATE DATABASE ${PG_DB};" postgres
  docker exec -i "${DB_CONTAINER}" \
    psql -U "${PG_USER}" -d "${PG_DB}" < "${BACKUP_DIR}/postgres.sql"
  success "Postgres restored."
fi

# ── Restore Caddy TLS data ────────────────────────────────────────────────────

if [[ -f "${BACKUP_DIR}/caddy-data.tar.gz" ]]; then
  info "Restoring Caddy TLS data..."
  CADDY_CONTAINER=$(docker compose ps -q caddy 2>/dev/null) || true
  if [[ -n "${CADDY_CONTAINER}" ]]; then
    docker exec -i "${CADDY_CONTAINER}" tar -xzf - -C / \
      < "${BACKUP_DIR}/caddy-data.tar.gz"
    success "Caddy data restored."
  else
    info "Caddy container not running — skipping TLS restore."
  fi
fi

# ── Restart paas to pick up restored data ─────────────────────────────────────

info "Restarting paas service..."
docker compose restart paas

echo ""
success "Restore complete!"
echo ""

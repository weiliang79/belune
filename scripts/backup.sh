#!/usr/bin/env bash
# Self-Hosted PaaS — Backup
# Creates a timestamped backup of Postgres data and Caddy TLS certs.
# Usage: bash backup.sh [output-dir]
#
# Optional encryption (age):
#   Set BACKUP_ENCRYPTION_KEY to an age public key (starts with "age1...")
#   or to a path of a file containing the public key.
#   The final archive will be written as <name>.tar.gz.age and can be
#   decrypted with:  age --decrypt -i <private-key-file> <archive>.tar.gz.age
#
# Example .env entry:
#   BACKUP_ENCRYPTION_KEY=age1ql3z7hjy54pw...
set -euo pipefail

INSTALL_DIR="${PAAS_DIR:-/opt/paas}"
BACKUP_DIR="${1:-${INSTALL_DIR}/backups}"
TIMESTAMP=$(date -u +"%Y%m%dT%H%M%SZ")
BACKUP_NAME="paas-backup-${TIMESTAMP}"
WORK_DIR="${BACKUP_DIR}/${BACKUP_NAME}"

# Resolve encryption key from env or .env file
BACKUP_ENCRYPTION_KEY="${BACKUP_ENCRYPTION_KEY:-}"
if [[ -z "${BACKUP_ENCRYPTION_KEY}" && -f "${INSTALL_DIR}/.env" ]]; then
  BACKUP_ENCRYPTION_KEY=$(grep '^BACKUP_ENCRYPTION_KEY=' "${INSTALL_DIR}/.env" 2>/dev/null \
    | cut -d= -f2- | tr -d '"' || true)
fi

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

# ── Encrypt (optional) ─────────────────────────────────────────────────────────

if [[ -n "${BACKUP_ENCRYPTION_KEY}" ]]; then
  if ! command -v age &>/dev/null; then
    die "'age' is not installed. Install it (https://github.com/FiloSottile/age) or unset BACKUP_ENCRYPTION_KEY."
  fi

  ENCRYPTED_ARCHIVE="${ARCHIVE}.age"
  info "Encrypting archive with age..."

  # Key can be a literal public key string or a path to a file containing one
  if [[ -f "${BACKUP_ENCRYPTION_KEY}" ]]; then
    age --encrypt -r "$(cat "${BACKUP_ENCRYPTION_KEY}")" --output "${ENCRYPTED_ARCHIVE}" "${ARCHIVE}"
  else
    age --encrypt -r "${BACKUP_ENCRYPTION_KEY}" --output "${ENCRYPTED_ARCHIVE}" "${ARCHIVE}"
  fi

  rm -f "${ARCHIVE}"
  ARCHIVE="${ENCRYPTED_ARCHIVE}"
  success "Encrypted archive: ${ARCHIVE}"
fi

# ── Remote upload (optional) ──────────────────────────────────────────────────

BACKUP_REMOTE_ENABLED="${BACKUP_REMOTE_ENABLED:-}"
if [[ -z "${BACKUP_REMOTE_ENABLED}" && -f "${INSTALL_DIR}/.env" ]]; then
  BACKUP_REMOTE_ENABLED=$(grep '^BACKUP_REMOTE_ENABLED=' "${INSTALL_DIR}/.env" 2>/dev/null \
    | cut -d= -f2- | tr -d '"' || true)
fi

if [[ "${BACKUP_REMOTE_ENABLED}" == "true" ]]; then
  UPLOAD_BIN="${INSTALL_DIR}/bin/paas-backup-upload"
  if [[ ! -x "${UPLOAD_BIN}" ]]; then
    die "paas-backup-upload not found at ${UPLOAD_BIN}. Re-run update.sh to extract it."
  fi
  info "Uploading ${ARCHIVE} to remote storage..."
  # Source .env so all BACKUP_S3_* variables are available to the helper.
  set -a
  # shellcheck source=/dev/null
  [[ -f "${INSTALL_DIR}/.env" ]] && source "${INSTALL_DIR}/.env"
  set +a
  "${UPLOAD_BIN}" "${ARCHIVE}"
  success "Remote upload complete."
fi

echo ""
success "Backup complete: ${ARCHIVE}"
echo ""

#!/usr/bin/env bash
# Belune — Backup
# Creates a timestamped backup of Postgres data and Caddy TLS certs.
# Usage: bash backup.sh [output-dir]
#
# This is the host/CLI backup path — used for manual runs
# (`systemctl start belune-backup.service`) and by update.sh before a version
# move. Daily automatic backups run in-app instead (Server → Backups: cron +
# retention), executed natively by the worker via the Docker API. Both
# producers write the SAME archive format to the SAME directory and record
# their own row in backup_runs (this script as trigger='cli', the worker as
# trigger='worker'), and take the same flock so they can never race each other.
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

INSTALL_DIR="${BELUNE_DIR:-/opt/belune}"
BACKUP_DIR="${1:-${INSTALL_DIR}/backups}"
TIMESTAMP=$(date -u +"%Y%m%dT%H%M%SZ")
BACKUP_NAME="belune-backup-${TIMESTAMP}"
WORK_DIR="${BACKUP_DIR}/${BACKUP_NAME}"
LOCK_FILE="${BACKUP_DIR}/.lock"

# Resolve encryption key from env or .env file
BACKUP_ENCRYPTION_KEY="${BACKUP_ENCRYPTION_KEY:-}"
if [[ -z "${BACKUP_ENCRYPTION_KEY}" && -f "${INSTALL_DIR}/.env" ]]; then
  BACKUP_ENCRYPTION_KEY=$(grep '^BACKUP_ENCRYPTION_KEY=' "${INSTALL_DIR}/.env" 2>/dev/null \
    | cut -d= -f2- | tr -d '"' || true)
fi

info()    { echo "  [info]  $*"; }
success() { echo "  [ok]    $*"; }
die()     { echo "  [err]   $*" >&2; record_finish "failed" 0 "" "$*"; exit 1; }

# ── backup_runs recording (best-effort — a DB hiccup must never fail the
# actual backup, which is the whole point of this being a host/offline path) ──

BACKUP_RUN_ID=""
RUN_RECORDED=0

# sql_escape doubles single quotes so a value can be safely interpolated into
# a single-quoted SQL string literal.
sql_escape() { printf '%s' "$1" | sed "s/'/''/g"; }

# run_sql executes a statement via `docker exec … psql` and prints its first
# output line. Silently returns empty on any failure (no DB_CONTAINER yet, DB
# unreachable, migration not applied) — recording a run is a nice-to-have, not
# a requirement for the backup itself to succeed.
#
# `head -1` matters even for a single-row `-tAc` query: psql still appends its
# command completion tag (e.g. "INSERT 0 1") as a second line after an
# INSERT...RETURNING result, which would otherwise corrupt a captured id with
# a trailing newline + tag.
run_sql() {
  [[ -n "${DB_CONTAINER:-}" ]] || return 0
  docker exec -i "${DB_CONTAINER}" psql -U "${PG_USER}" -d "${PG_DB}" -tAc "$1" 2>/dev/null | head -1 || true
}

# record_finish updates BACKUP_RUN_ID's row once (idempotent — die() and the
# exit trap can both try). No-op if no row was ever inserted.
record_finish() {
  [[ -n "${BACKUP_RUN_ID}" && "${RUN_RECORDED}" == "0" ]] || return 0
  RUN_RECORDED=1
  local status="$1" size="${2:-0}" key="${3:-}" err="${4:-}"
  local key_sql="NULL" err_sql="NULL"
  [[ -n "${key}" ]] && key_sql="'$(sql_escape "${key}")'"
  [[ -n "${err}" ]] && err_sql="'$(sql_escape "${err}")'"
  run_sql "UPDATE backup_runs SET finished_at = NOW(), status = '${status}', size_bytes = ${size}, remote_key = ${key_sql}, error = ${err_sql} WHERE id = '${BACKUP_RUN_ID}';" >/dev/null
}

# Any exit before the explicit "succeeded" record_finish call at the bottom —
# a bare `set -e` exit from a command with no `|| die`, or Ctrl-C — is a
# failure that die() never got a chance to record with a specific message.
on_exit() {
  local rc=$?
  if [[ ${rc} -ne 0 ]]; then
    record_finish "failed" 0 "" "backup.sh exited with status ${rc}"
  fi
}
trap on_exit EXIT

[[ -f "${INSTALL_DIR}/docker-compose.yml" ]] || \
  die "No docker-compose.yml found at ${INSTALL_DIR}. Is Belune installed?"

cd "${INSTALL_DIR}"

echo ""
echo "  Belune — Backup"
echo "  ==========================="
echo ""

mkdir -p "${BACKUP_DIR}"

# Exclusive, non-blocking: the worker (in-app cron/manual) takes this same
# lock via the bind-mounted ./backups directory, so a concurrent worker run
# and CLI run can never write over each other's archive.
exec 200>"${LOCK_FILE}"
flock -n 200 || die "a backup is already in progress (${LOCK_FILE} is locked)"

mkdir -p "${WORK_DIR}"

# ── Postgres dump ──────────────────────────────────────────────────────────────

info "Dumping Postgres database..."
DB_CONTAINER=$(docker compose ps -q postgres 2>/dev/null) || die "Postgres container not running."
PG_USER=$(grep 'POSTGRES_USER' .env 2>/dev/null | cut -d= -f2 || echo "belune")
PG_DB=$(grep 'POSTGRES_DB'   .env 2>/dev/null | cut -d= -f2 || echo "belune")

BACKUP_RUN_ID=$(run_sql "INSERT INTO backup_runs (trigger) VALUES ('cli') RETURNING id;")

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

REMOTE_KEY=""
if [[ "${BACKUP_REMOTE_ENABLED}" == "true" ]]; then
  UPLOAD_BIN="${INSTALL_DIR}/bin/belune-backup-upload"
  if [[ ! -x "${UPLOAD_BIN}" ]]; then
    die "belune-backup-upload not found at ${UPLOAD_BIN}. Re-run update.sh to extract it."
  fi
  info "Uploading ${ARCHIVE} to remote storage..."
  # Source .env so all BACKUP_S3_* variables are available to the helper.
  set -a
  # shellcheck source=/dev/null
  [[ -f "${INSTALL_DIR}/.env" ]] && source "${INSTALL_DIR}/.env"
  set +a
  UPLOAD_OUTPUT=$("${UPLOAD_BIN}" "${ARCHIVE}")
  echo "${UPLOAD_OUTPUT}"
  REMOTE_KEY="${UPLOAD_OUTPUT#uploaded: }"
  success "Remote upload complete."
fi

SIZE_BYTES=$(stat -c%s "${ARCHIVE}" 2>/dev/null || echo 0)
record_finish "succeeded" "${SIZE_BYTES}" "${REMOTE_KEY}" ""

echo ""
success "Backup complete: ${ARCHIVE}"
echo ""

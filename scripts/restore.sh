#!/usr/bin/env bash
# Self-Hosted PaaS — Restore
# Restores a backup created by backup.sh.
#
# Usage:
#   bash restore.sh [options] <backup.tar.gz[.age]> [age-identity-file]
#
# Options:
#   --dry-run        Inspect the archive and print what WOULD be restored, then
#                    exit without touching the running system. Safe to run any
#                    time to verify a backup is readable.
#   -y, --yes        Skip the interactive confirmation prompt (required when
#                    running non-interactively, e.g. from another script).
#   -h, --help       Show this help and exit.
#
# If the archive ends in .age it will be decrypted first. Provide the age
# identity (private key) file as the positional argument after the archive, or
# set BACKUP_IDENTITY_FILE in the environment.
#
# SAFETY: A restore is DESTRUCTIVE — it drops and recreates the Postgres
# database and overwrites .env and Caddy TLS data. Before dropping the database
# this script writes a pre-restore snapshot of the CURRENT database to
# ${BELUNE_DIR}/backups/pre-restore-<timestamp>.sql so a botched restore can be
# rolled back.
set -euo pipefail

INSTALL_DIR="${BELUNE_DIR:-/opt/belune}"

info()    { echo "  [info]  $*"; }
success() { echo "  [ok]    $*"; }
warn()    { echo "  [warn]  $*" >&2; }
die()     { echo "  [err]   $*" >&2; exit 1; }

usage() {
  sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}

# ── Parse arguments ─────────────────────────────────────────────────────────

DRY_RUN=false
ASSUME_YES=false
POSITIONAL=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)   DRY_RUN=true; shift ;;
    -y|--yes)    ASSUME_YES=true; shift ;;
    -h|--help)   usage 0 ;;
    --)          shift; while [[ $# -gt 0 ]]; do POSITIONAL+=("$1"); shift; done ;;
    -*)          die "Unknown option: $1 (see --help)" ;;
    *)           POSITIONAL+=("$1"); shift ;;
  esac
done

BACKUP_ARCHIVE="${POSITIONAL[0]:-}"
BACKUP_IDENTITY_FILE="${POSITIONAL[1]:-${BACKUP_IDENTITY_FILE:-}}"

[[ -n "${BACKUP_ARCHIVE}" ]] || usage 1
[[ -f "${BACKUP_ARCHIVE}" ]] || die "Backup file not found: ${BACKUP_ARCHIVE}"
[[ -f "${INSTALL_DIR}/docker-compose.yml" ]] || \
  die "No docker-compose.yml found at ${INSTALL_DIR}. Is PaaS installed?"

cd "${INSTALL_DIR}"

echo ""
echo "  Self-Hosted PaaS — Restore"
echo "  ============================"
echo "  Source:  ${BACKUP_ARCHIVE}"
${DRY_RUN} && echo "  Mode:    DRY RUN (no changes will be made)"
echo ""

# ── Decrypt archive (if encrypted) ────────────────────────────────────────────

WORK_DIR=$(mktemp -d)
trap 'rm -rf "${WORK_DIR}"' EXIT

ARCHIVE_TO_EXTRACT="${BACKUP_ARCHIVE}"

if [[ "${BACKUP_ARCHIVE}" == *.age ]]; then
  if ! command -v age &>/dev/null; then
    die "'age' is not installed. Install it (https://github.com/FiloSottile/age) to restore encrypted backups."
  fi
  [[ -n "${BACKUP_IDENTITY_FILE}" ]] || \
    die "Encrypted backup requires an age identity file. Pass it as the second argument or set BACKUP_IDENTITY_FILE."
  [[ -f "${BACKUP_IDENTITY_FILE}" ]] || \
    die "Age identity file not found: ${BACKUP_IDENTITY_FILE}"

  DECRYPTED="${WORK_DIR}/decrypted.tar.gz"
  info "Decrypting archive with age..."
  age --decrypt -i "${BACKUP_IDENTITY_FILE}" --output "${DECRYPTED}" "${BACKUP_ARCHIVE}" \
    || die "Decryption failed. Wrong identity file, or the archive is corrupt."
  ARCHIVE_TO_EXTRACT="${DECRYPTED}"
  success "Archive decrypted."
fi

# ── Verify archive integrity before extracting ─────────────────────────────────

info "Verifying archive integrity..."
tar -tzf "${ARCHIVE_TO_EXTRACT}" >/dev/null 2>&1 \
  || die "Archive is corrupt or not a valid gzip tarball: ${BACKUP_ARCHIVE}"
success "Archive is a readable gzip tarball."

# ── Extract archive ────────────────────────────────────────────────────────────

info "Extracting backup..."
tar -xzf "${ARCHIVE_TO_EXTRACT}" -C "${WORK_DIR}"
BACKUP_DIR=$(find "${WORK_DIR}" -mindepth 1 -maxdepth 1 -type d | head -1)
[[ -n "${BACKUP_DIR}" ]] || die "Could not find backup directory inside archive."

# ── Manifest: show what this archive contains ──────────────────────────────────

have_env=false; have_pg=false; have_caddy=false
[[ -f "${BACKUP_DIR}/.env" ]]              && have_env=true
[[ -f "${BACKUP_DIR}/postgres.sql" ]]      && have_pg=true
[[ -f "${BACKUP_DIR}/caddy-data.tar.gz" ]] && have_caddy=true

pg_size="—"; pg_lines="—"
if ${have_pg}; then
  pg_size=$(du -h "${BACKUP_DIR}/postgres.sql" | cut -f1)
  pg_lines=$(wc -l < "${BACKUP_DIR}/postgres.sql" | tr -d ' ')
fi

echo ""
echo "  Archive contents"
echo "  ----------------"
printf "  %-18s %s\n" ".env"          "$(${have_env}   && echo "present" || echo "MISSING")"
printf "  %-18s %s\n" "postgres.sql"  "$(${have_pg}    && echo "present (${pg_size}, ${pg_lines} lines)" || echo "MISSING")"
printf "  %-18s %s\n" "caddy-data"    "$(${have_caddy} && echo "present" || echo "absent (no TLS data)")"
echo ""

${have_pg} || die "Archive has no postgres.sql — refusing to restore an archive with no database dump."

# ── Dry run stops here ─────────────────────────────────────────────────────────

if ${DRY_RUN}; then
  success "Dry run complete — archive is valid and restorable. No changes were made."
  echo ""
  echo "  To perform the restore for real, re-run without --dry-run."
  echo ""
  exit 0
fi

# ── Confirm the destructive action ─────────────────────────────────────────────

echo "  This will OVERWRITE the current install at ${INSTALL_DIR}:"
${have_pg}    && echo "    • DROP and recreate the Postgres database"
${have_env}   && echo "    • Overwrite .env"
${have_caddy} && echo "    • Overwrite Caddy TLS data (certificates + config)"
echo "    • Restart the belune service"
echo ""

if ! ${ASSUME_YES}; then
  if [[ -t 0 ]]; then
    read -r -p "  Type 'restore' to proceed: " reply
    [[ "${reply}" == "restore" ]] || die "Aborted — confirmation not given."
  else
    die "Refusing to run destructively without a TTY. Re-run with --yes to confirm non-interactively."
  fi
fi

# ── Restore .env ──────────────────────────────────────────────────────────────

if ${have_env}; then
  info "Restoring .env..."
  cp "${BACKUP_DIR}/.env" .env
  success ".env restored."
fi

# ── Restore Postgres ──────────────────────────────────────────────────────────

if ${have_pg}; then
  DB_CONTAINER=$(docker compose ps -q postgres 2>/dev/null) || die "Postgres container not running."
  [[ -n "${DB_CONTAINER}" ]] || die "Postgres container not running. Start it first: docker compose up -d postgres"
  PG_USER=$(grep 'POSTGRES_USER' .env 2>/dev/null | cut -d= -f2 || echo "belune")
  PG_DB=$(grep 'POSTGRES_DB'   .env 2>/dev/null | cut -d= -f2 || echo "belune")

  # Wait for Postgres to accept connections before touching it — a just-started
  # container (e.g. after a corrupt-volume rebuild) may not be ready yet.
  info "Waiting for Postgres to be ready..."
  pg_ready=false
  for _ in $(seq 1 30); do
    if docker exec "${DB_CONTAINER}" pg_isready -U "${PG_USER}" &>/dev/null; then
      pg_ready=true; break
    fi
    sleep 1
  done
  ${pg_ready} || die "Postgres did not become ready within 30s. Check: docker compose logs postgres"
  success "Postgres is ready."

  # Stop the API before dropping the database: Postgres refuses to DROP a
  # database that still has active sessions, and the belune container holds live
  # connections (its worker runs there too). We bring it back up only on
  # success — if the restore fails, belune stays down so it can't connect to a
  # half-restored database.
  info "Stopping the belune service to release database connections..."
  docker compose stop belune >/dev/null 2>&1 || true
  success "belune service stopped."

  # Pre-restore safety snapshot: dump the CURRENT database before we drop it, so
  # a bad restore is recoverable. Best-effort — skipped if the DB doesn't exist
  # yet (fresh host).
  SNAPSHOT_DIR="${INSTALL_DIR}/backups"
  SNAPSHOT="${SNAPSHOT_DIR}/pre-restore-$(date -u +"%Y%m%dT%H%M%SZ").sql"
  mkdir -p "${SNAPSHOT_DIR}"
  if docker exec "${DB_CONTAINER}" psql -U "${PG_USER}" -d "${PG_DB}" -c '\q' &>/dev/null; then
    info "Snapshotting current database before overwrite..."
    if docker exec "${DB_CONTAINER}" pg_dump -U "${PG_USER}" -d "${PG_DB}" --no-password > "${SNAPSHOT}" 2>/dev/null; then
      success "Pre-restore snapshot saved: ${SNAPSHOT}"
      # If anything below fails, tell the user how to get back.
      trap 'rc=$?; rm -rf "${WORK_DIR}"; if [[ ${rc} -ne 0 ]]; then
        echo "" >&2
        echo "  [err]   Restore FAILED (exit ${rc}). The database may be in a partial state." >&2
        echo "  [err]   The belune service is stopped. Roll back to the pre-restore snapshot, then start it:" >&2
        echo "          docker exec -i ${DB_CONTAINER} psql -U ${PG_USER} -d ${PG_DB} < ${SNAPSHOT}" >&2
        echo "          docker compose up -d belune" >&2
      fi' EXIT
    else
      rm -f "${SNAPSHOT}"
      warn "Could not snapshot current database (continuing anyway)."
    fi
  else
    info "No existing database to snapshot (fresh install) — skipping snapshot."
  fi

  info "Restoring Postgres database..."
  # Drop and recreate the database to ensure a clean restore
  docker exec "${DB_CONTAINER}" \
    psql -U "${PG_USER}" -c "DROP DATABASE IF EXISTS ${PG_DB};" postgres
  docker exec "${DB_CONTAINER}" \
    psql -U "${PG_USER}" -c "CREATE DATABASE ${PG_DB};" postgres
  docker exec -i "${DB_CONTAINER}" \
    psql -U "${PG_USER}" -d "${PG_DB}" < "${BACKUP_DIR}/postgres.sql"
  success "Postgres restored."

  # Database is restored: the destructive window is closed. Drop the rollback
  # trap so a later (non-DB) step can't print a misleading "roll back" message.
  trap 'rm -rf "${WORK_DIR}"' EXIT
fi

# ── Restore Caddy TLS data ────────────────────────────────────────────────────

if ${have_caddy}; then
  info "Restoring Caddy TLS data..."
  CADDY_CONTAINER=$(docker compose ps -q caddy 2>/dev/null) || true
  if [[ -n "${CADDY_CONTAINER}" ]]; then
    docker exec -i "${CADDY_CONTAINER}" tar -xzf - -C / \
      < "${BACKUP_DIR}/caddy-data.tar.gz"
    success "Caddy data restored."
  else
    warn "Caddy container not running — skipping TLS restore."
  fi
fi

# ── Bring the belune service back up ────────────────────────────────────────────
# We stopped it earlier to release DB connections. `up -d` starts it (respecting
# the postgres/redis health dependencies) and picks up the restored data + .env.

info "Starting belune service..."
docker compose up -d belune \
  || warn "Could not start belune automatically. Start it manually: docker compose up -d belune"

echo ""
success "Restore complete!"
echo ""

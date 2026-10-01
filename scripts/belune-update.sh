#!/usr/bin/env bash
# Belune — Updater (in-image)
#
# This is /usr/local/bin/belune-update inside the Belune image, and it is the
# ONLY thing that moves an install between versions. It is not run directly:
# the dashboard (SpawnUpdateHelper) and scripts/update.sh both pull the TARGET
# image and run this file out of it, so the updater that runs is always the one
# shipped with the version being installed — a fix to it reaches the very update
# that installs it, instead of the one after.
#
# The contract with whatever spawns it is frozen and deliberately dumb:
#   /usr/local/bin/belune-update <target-version>
# Old releases' launchers are old code on every future update and cannot be
# repaired, so nothing may be added to that contract. The version is required:
# the launchers resolve "latest" themselves because they need it to pick the
# image to pull.
#
# An update is a deliberate version move, never a drift: it resolves a target
# version, takes a backup before touching anything, rewrites the pinned image in
# .env, reconciles the version-pinned infra files (compose, Caddy, BuildKit,
# systemd, scripts) so a release that changes a service definition actually takes
# effect, and tells you exactly how to roll back if the new version does not come
# up. Migrations run automatically at boot and are not reversible, which is why
# the backup happens first.
#
# The infra files are a matched set with the image: they are fetched from the
# target release's git ref — never main/latest — exactly as install.sh does, so
# updating to a version yields the same files every time and rollback can restore
# the set that ran with the old image. Because a changed compose can touch any
# service (not just belune), the restart is a full `docker compose up -d`.
set -euo pipefail

# ⚠️ Everything below lives inside main(), and the file ends with
# `main "$@"; exit $?` on ONE line. Do not "tidy" either.
#
# This file no longer overwrites ITSELF (it lives in the image, outside the
# install dir), but the swap loop still replaces scripts/update.sh — the
# launcher — and when an operator runs that on the host its bash is still
# reading it while this runs. The launcher carries the same wrap; it is kept
# here too so the two cannot drift apart. The hazard: bash reads a script incrementally, by byte
# offset, so overwriting the running file makes it resume at the same offset in
# different content and execute whatever lands there (v0.1.11-rc1 tried to run a
# box-drawing banner line as a command). It stayed hidden through v0.1.10 only
# because the file was byte-identical across releases, so every swap wrote the
# same bytes. A function body is parsed in full before it runs, and `exit` on the
# same line as the call means bash never reads the file again afterwards.
#
# Like any fix to a launcher, it only protects updates that START from a
# version that contains it.
main() {

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
  [[ -n "${TARGET_VERSION}" ]] || die "No target version given. Usage: belune-update <version>"

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

  # The launcher already pulled this (it had to, to run us out of it), so this is
  # normally a no-op. Kept so a direct run of this file stays safe; the cost is
  # one registry round trip.
  info "Pulling ${TARGET_IMAGE}..."
  docker pull "${TARGET_IMAGE}" || die "Could not pull ${TARGET_IMAGE}. Does that version exist?"

  # ── Fetch version-pinned infra files ───────────────────────────────────────────

  # Git tags keep the leading v; image tags drop it. TARGET_VERSION was normalised
  # without one above, so re-add it for the git ref — same construction install.sh
  # uses so the two stay in lockstep.
  GIT_REF="v${TARGET_VERSION}"
  RAW_URL="https://raw.githubusercontent.com/${GITHUB_REPO}/${GIT_REF}"

  # "<local path under install dir>|<path in repo>". The local docker-compose.yml is
  # the repo's infra/docker-compose.prod.yml; everything else keeps its path. This
  # is exactly the set install.sh lays down, so a fresh install and an updated one
  # converge on the same files.
  INFRA_FILES=(
    "docker-compose.yml|infra/docker-compose.prod.yml"
    "infra/caddy/Caddyfile.template|infra/caddy/Caddyfile.template"
    "infra/buildkit/buildkitd.toml|infra/buildkit/buildkitd.toml"
    ".env.example|.env.example"
    "infra/systemd/belune.service|infra/systemd/belune.service"
    "infra/systemd/belune-backup.service|infra/systemd/belune-backup.service"
    "scripts/backup.sh|scripts/backup.sh"
    "scripts/restore.sh|scripts/restore.sh"
    "scripts/update.sh|scripts/update.sh"
  )
  # Paths that must stay executable after the swap.
  EXEC_FILES=" scripts/backup.sh scripts/restore.sh scripts/update.sh "

  # Download everything to a staging dir first: a failed fetch must abort before a
  # single file on disk is touched, so a transient network error can never leave a
  # half-updated infra set. Cleaned up on any exit.
  # Between moving the pin and starting `docker compose up -d`, a failure would
  # leave .env and the infra files on the new version while the containers still
  # run the old image — and the next unrelated `up -d` would silently jump to it.
  # So a failure in that window puts both back. Deliberately NOT extended past
  # `up -d`: by then containers may be recreated and migrations applied, and
  # reverting the image over a migrated schema is worse than the printed rollback.
  REVERT_ARMED=0
  # shellcheck disable=SC2329 # invoked by the EXIT trap
  on_exit() {
    local rc=$?
    rm -rf "${STAGE_DIR:-}"
    if [[ "${REVERT_ARMED}" == "1" && ${rc} -ne 0 ]]; then
      REVERT_ARMED=0
      echo "  [warn]  The update failed before the restart — restoring the previous pin and infra files." >&2
      cp ".env.backup-${CURRENT_VERSION}" .env \
        && { [[ ! -d "${INFRA_BACKUP:-}" ]] || cp -a "${INFRA_BACKUP}/." .; } \
        && echo "  [warn]  Restored. Nothing has changed." >&2 \
        || echo "  [err]   Could not restore automatically; see .env.backup-${CURRENT_VERSION} and ${INFRA_BACKUP:-the infra backup}." >&2
    fi
  }

  info "Fetching infra files for ${TARGET_VERSION}..."
  STAGE_DIR="$(mktemp -d)"
  trap on_exit EXIT
  for entry in "${INFRA_FILES[@]}"; do
    local_path="${entry%%|*}"
    repo_path="${entry#*|}"
    dest="${STAGE_DIR}/${local_path}"
    mkdir -p "$(dirname "${dest}")"
    curl -sSfL "${RAW_URL}/${repo_path}" -o "${dest}" \
      || die "Could not fetch ${repo_path} for ${TARGET_VERSION}. Nothing has changed."
  done
  success "Infra files fetched."

  # ── Back up before migrating ───────────────────────────────────────────────────

  # 0.x minor releases may contain breaking changes, and migrations are
  # forward-only: this backup is the rollback path for the data, while the image
  # tag below is the rollback path for the code.
  # ask_continue_without_backup prompts on a terminal; with no stdin (the
  # dashboard runs this script in a detached helper container) it aborts instead
  # of reading EOF. Aborting stays the non-interactive default: silently updating
  # with no rollback point is worse than stopping. $1 is the cause, $2 the prompt.
  ask_continue_without_backup() {
    if [[ -t 0 ]]; then
      warn "$1"
      read -rp "  $2 [y/N] " reply
      [[ "${reply}" =~ ^[Yy]$ ]] || die "Aborted. Nothing has changed."
    else
      die "$1 There is no terminal to ask on, so the update was stopped and nothing has changed. To be asked whether to continue without a backup, run 'bash ${INSTALL_DIR}/scripts/update.sh' on the host."
    fi
  }

  if [[ -x "${INSTALL_DIR}/scripts/backup.sh" ]] || [[ -f "${INSTALL_DIR}/scripts/backup.sh" ]]; then
    info "Taking a pre-update backup..."
    # --local-only-ok: this backup is a local rollback point, so a failed remote
    # upload is a warning here. It is fatal for a manual or scheduled backup.
    # An older installed backup.sh would read the flag as its output directory,
    # so only pass it when the script advertises it.
    BACKUP_FLAGS=()
    grep -q -- '--local-only-ok' "${INSTALL_DIR}/scripts/backup.sh" && BACKUP_FLAGS=(--local-only-ok)
    if bash "${INSTALL_DIR}/scripts/backup.sh" ${BACKUP_FLAGS[@]+"${BACKUP_FLAGS[@]}"}; then
      success "Backup complete."
    else
      ask_continue_without_backup \
        "The pre-update backup failed (see the output above for the reason)." \
        "Continue updating without a backup?"
    fi
  else
    ask_continue_without_backup \
      "scripts/backup.sh was not found in ${INSTALL_DIR}, so no pre-update backup could be taken." \
      "Continue without a backup?"
  fi

  # ── Move the pin ───────────────────────────────────────────────────────────────

  cp .env ".env.backup-${CURRENT_VERSION}"
  REVERT_ARMED=1
  if grep -q '^BELUNE_IMAGE=' .env; then
    # Portable in-place edit: GNU and BSD sed disagree about -i.
    sed "s|^BELUNE_IMAGE=.*|BELUNE_IMAGE=${TARGET_IMAGE}|" .env > .env.tmp && mv .env.tmp .env
  else
    echo "BELUNE_IMAGE=${TARGET_IMAGE}" >> .env
  fi
  success "Pinned to ${TARGET_VERSION}."

  # Swap in the new infra files, keeping a per-version copy of the old ones so the
  # rollback can restore the exact set that ran with the previous image. The backup
  # preserves each file's path under the install dir.
  INFRA_BACKUP="${INSTALL_DIR}/.infra-backup-${CURRENT_VERSION}"
  mkdir -p "${INFRA_BACKUP}"
  for entry in "${INFRA_FILES[@]}"; do
    local_path="${entry%%|*}"
    if [[ -f "${local_path}" ]]; then
      mkdir -p "${INFRA_BACKUP}/$(dirname "${local_path}")"
      cp "${local_path}" "${INFRA_BACKUP}/${local_path}"
    fi
    mkdir -p "$(dirname "${local_path}")"
    cp "${STAGE_DIR}/${local_path}" "${local_path}"
    case "${EXEC_FILES}" in
      *" ${local_path} "*) chmod +x "${local_path}" ;;
    esac
  done
  success "Infra files updated (previous set saved in ${INFRA_BACKUP})."

  # ── Restart (migrations run automatically on startup) ──────────────────────────

  rollback_hint() {
    echo ""
    echo "  To roll back to ${CURRENT_VERSION}:"
    echo ""
    echo "      cd ${INSTALL_DIR}"
    echo "      sed -i 's|^BELUNE_IMAGE=.*|BELUNE_IMAGE=${CURRENT_IMAGE}|' .env"
    echo "      cp -a ${INFRA_BACKUP}/. ."
    echo "      docker compose up -d"
    echo ""
    echo "  The cp restores the previous version's compose and infra files; the"
    echo "  full 'up -d' reverts any service the new compose had changed."
    echo ""
    echo "  If the new version already applied migrations, restore the pre-update"
    echo "  backup as well — see docs/runbooks/disaster-recovery.md."
    echo ""
  }

  # Ensure the file-mounts directory exists and is owned by the belune container's
  # uid before the reconcile brings the (possibly newly-added) bind mount up. An
  # install from before this dir was managed won't have it; create + chown it here
  # so file mounts work after updating, matching install.sh. Idempotent.
  mkdir -p "${INSTALL_DIR}/filemounts"
  FM_UID=$(docker run --rm --entrypoint id "${TARGET_IMAGE}" -u 2>/dev/null || true)
  FM_GID=$(docker run --rm --entrypoint id "${TARGET_IMAGE}" -g 2>/dev/null || true)
  if [[ -n "${FM_UID}" && -n "${FM_GID}" ]]; then
    chown "${FM_UID}:${FM_GID}" "${INSTALL_DIR}/filemounts"
  fi

  # Same for the backups directory: the new compose bind-mounts it into the belune
  # container so the worker can write control-plane archives natively. An install
  # from before Shape A already has ./backups (root-owned, from belune-backup.timer
  # running backup.sh as root) — chown it so the non-root worker can write there too.
  mkdir -p "${INSTALL_DIR}/backups"
  if [[ -n "${FM_UID}" && -n "${FM_GID}" ]]; then
    chown "${FM_UID}:${FM_GID}" "${INSTALL_DIR}/backups"
  fi

  # Same for the remote-storage config file (Q1): a FILE bind mount, so `touch`
  # it (not mkdir) before the reconcile brings the newly-added mount up, then
  # chown it — an install predating this has no such file, and the container
  # needs to already own it since it can't create a new directory entry in the
  # root-owned install dir.
  touch "${INSTALL_DIR}/backup-remote.env"
  chmod 600 "${INSTALL_DIR}/backup-remote.env"
  if [[ -n "${FM_UID}" && -n "${FM_GID}" ]]; then
    chown "${FM_UID}:${FM_GID}" "${INSTALL_DIR}/backup-remote.env"
  fi

  # Full reconcile, not --no-deps belune: the refreshed compose may change any
  # service (a new dependency, a Caddy/Redis/BuildKit tweak), and only `up -d` over
  # the whole project applies those. Compose recreates only what actually changed,
  # so an image-only update still just replaces the belune container.
  # Past this point a failure is handled by rollback_hint, not an automatic revert.
  REVERT_ARMED=0
  info "Reconciling the stack (docker compose up -d)..."
  if ! docker compose up -d; then
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
  info "Previous infra files saved in ${INFRA_BACKUP}"

  # The systemd units are refreshed in the install dir, but the active copies live
  # in /etc/systemd/system (install.sh puts them there for a root+systemd install).
  # Only tell the operator to re-copy when they actually differ, so a Docker-only
  # install never sees an irrelevant instruction.
  if [[ -d /etc/systemd/system ]] && [[ -f /etc/systemd/system/belune.service ]]; then
    # Daily backups now run in-app (Server → Backups: cron + retention). An
    # install that predates that still has belune-backup.timer firing at 02:00,
    # which would race the in-app run on the same archive lockfile and double
    # the backup cadence — retire it. belune-backup.service is left alone: it is
    # still the manual/CLI DR fallback (`systemctl start belune-backup.service`).
    if [[ -f /etc/systemd/system/belune-backup.timer ]]; then
      info "Retiring belune-backup.timer — daily backups now run in-app (Server → Backups)."
      systemctl disable --now belune-backup.timer >/dev/null 2>&1 || true
      rm -f /etc/systemd/system/belune-backup.timer
      systemctl daemon-reload
      success "belune-backup.timer disabled and removed."
    fi

    for unit in belune.service belune-backup.service; do
      if ! cmp -s "infra/systemd/${unit}" "/etc/systemd/system/${unit}" 2>/dev/null; then
        echo ""
        warn "systemd units changed in this release. To apply them:"
        echo "      sudo cp infra/systemd/*.service /etc/systemd/system/"
        echo "      sudo systemctl daemon-reload"
        break
      fi
    done
  fi
  echo ""
}

main "$@"; exit $?

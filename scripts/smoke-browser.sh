#!/usr/bin/env bash
#
# Browser smoke drill.
#
# Runs the production smoke drill, keeps the stack it booted, and then drives
# that stack in a real browser with Playwright (apps/web/e2e).
#
# Why it is separate from the assertions in smoke-prod.sh: those probe the API,
# the proxy and the database with curl and psql, and are deliberately free of
# any toolchain. Everything here is about what a human SEES — a toast that is
# present in the DOM but composited at opacity 0 is invisible to a person and
# indistinguishable from a working one to every check that is not a browser.
# That is not hypothetical: it is what v0.1.16 shipped twice (`7d77fbd`), and
# what three hand-run drills were spent finding.
#
# Usage:
#   ./scripts/smoke-browser.sh                      # build, boot, drill
#   BELUNE_IMAGE=ghcr.io/weiliang79/belune:0.1.16 ./scripts/smoke-browser.sh
#   ./scripts/smoke-browser.sh --headed             # watch it happen
#   KEEP=1 ./scripts/smoke-browser.sh               # leave the stack up
#
# ⚠️ BELUNE_IMAGE must be an image stamped with a real VERSION. Pointing it at
# an unstamped build is caught by assertion 13 in smoke-prod.sh, because
# otherwise it looks exactly like "no update available" here.
set -euo pipefail

# Absolute, and every later cd goes through it. The compose invocation below
# uses repo-relative paths (`-f infra/...`, `--project-directory .`), so running
# it from anywhere else resolves nothing — and because teardown is `|| true`, a
# stray cd made the stack survive silently and poisoned the NEXT run with its
# database.
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

PROJECT=belunesmoke
ENV_FILE=".env.smoke"
BELUNE_URL=http://127.0.0.1:18081

# The release the install is told exists. It must NOT exist in the registry —
# that is the whole mechanism: the pull fails, so the update never starts, and
# nothing on the host is ever touched (the helper container is what touches
# things, and it is created only after a successful pull).
TARGET="${BELUNE_E2E_TARGET:-0.1.99}"

# Generated per run, handed to smoke-prod.sh to create the first admin with, and
# passed to Playwright to sign in with. It exists only in this process tree.
export SMOKE_ADMIN_EMAIL="${SMOKE_ADMIN_EMAIL:-admin@belune.invalid}"
export SMOKE_ADMIN_PASSWORD="${SMOKE_ADMIN_PASSWORD:-$(openssl rand -hex 16)}"

COMPOSE=(docker compose
  -p "$PROJECT"
  --project-directory .
  -f infra/docker-compose.prod.yml
  -f infra/docker-compose.smoke.yml
  --env-file "$ENV_FILE")

info() { printf '\n\033[1m%s\033[0m\n' "$1"; }

# This script owns the teardown, because it runs the inner drill with KEEP=1 and
# that suppresses the inner trap — including when the inner drill FAILS, which
# would otherwise leave a stack and a password file behind on every red run.
cleanup() {
  local code=$?
  cd "$ROOT"
  if [[ "${KEEP:-0}" == "1" ]]; then
    info "KEEP=1 — leaving the stack up. Tear down with:"
    echo "  docker compose -p $PROJECT --project-directory . -f infra/docker-compose.prod.yml -f infra/docker-compose.smoke.yml --env-file $ENV_FILE down -v"
    exit $code
  fi
  info "Tearing down"
  "${COMPOSE[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
  rm -f "$ENV_FILE"
  exit $code
}
trap cleanup EXIT

if [[ ! -d apps/web/node_modules ]]; then
  echo "apps/web/node_modules is missing — run 'npm ci' in apps/web first." >&2
  exit 1
fi

# KEEP=1 so the stack survives for the browser. Everything smoke-prod.sh asserts
# is a precondition for this drill, so a failure there stops us here rather than
# being reported as a missing button.
KEEP=1 ./scripts/smoke-prod.sh

info "Seeding an update that cannot be pulled (v$TARGET)"
# Written straight to settings, the same way the daily check writes its cache.
# These keys are deliberately absent from updatableSettings — a token must not
# be able to forge an available update — so SQL is the only way in, which is
# also why the drill cannot stage this through the API.
#
# update_check_enabled=false pins the scenario: the daily sweep's first
# activation is a full interval away so it would not fire anyway, but a drill
# whose premise can be overwritten mid-run by a background worker is a flake
# waiting to happen.
"${COMPOSE[@]}" exec -T postgres psql -U belune -d belune -v ON_ERROR_STOP=1 -q <<SQL
INSERT INTO settings (key, value) VALUES
  ('update_latest_version', '$TARGET'),
  ('update_latest_requires_host_update', 'false'),
  ('update_latest_breaking', 'false'),
  ('update_check_enabled', 'false')
  ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;
SQL

info "Driving the dashboard in a real browser"
# In a subshell, so the cd cannot outlive it — see ROOT above.
(
  cd "$ROOT/apps/web"
  # Idempotent and fast once cached. CI installs the system libraries too
  # (--with-deps), which needs root and is not this script's business.
  npx playwright install chromium >/dev/null
  BELUNE_E2E_URL="$BELUNE_URL" \
  BELUNE_E2E_EMAIL="$SMOKE_ADMIN_EMAIL" \
  BELUNE_E2E_PASSWORD="$SMOKE_ADMIN_PASSWORD" \
  BELUNE_E2E_TARGET="$TARGET" \
    npx playwright test "$@"
)

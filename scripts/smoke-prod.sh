#!/usr/bin/env bash
#
# Production smoke drill.
#
# Boots the real production stack — the real image, the real Caddy, a non-root
# user in a container — and asserts that features are actually ALIVE, not merely
# that the code compiles.
#
# Why this exists: every bug found during the v0.0.29 VPS trial was invisible in
# development and only appeared in production, because dev runs the API on the
# host against a Vite dev server while production serves one binary as a non-root
# user in a container behind Caddy. Two of them — request logging, and reporting
# why a certificate failed — were entire features that had been silently dead in
# production since the day they shipped. Nothing failed. Nothing logged an error
# a human would see. They just quietly did nothing.
#
# So every assertion below is a bug that actually happened. Each one is annotated
# with what it caught. Do not "simplify" an assertion into checking that a
# container is running: a running container is exactly what all of these bugs
# looked like.
#
# Usage:
#   ./scripts/smoke-prod.sh              # builds the image, runs the drill
#   BELUNE_IMAGE=belune:trial ./scripts/smoke-prod.sh   # reuse an existing image
#   KEEP=1 ./scripts/smoke-prod.sh       # leave the stack up for poking at
set -euo pipefail

cd "$(dirname "$0")/.."

PROJECT=belunesmoke
IMAGE="${BELUNE_IMAGE:-belune:smoke}"
# Fixed path, not a temp file: infra/docker-compose.smoke.yml has to reference
# it statically in env_file, which cannot interpolate a random name.
ENV_FILE=".env.smoke"
COMPOSE=(docker compose
  -p "$PROJECT"
  -f infra/docker-compose.prod.yml
  -f infra/docker-compose.smoke.yml
  --env-file "$ENV_FILE")

# The dashboard is reached through Caddy by Host header, so it needs a name. It
# never resolves in DNS and never should.
DASHBOARD_HOST=smoke.belune.internal
# An application domain, which is the only kind the access-log tailer records.
APP_HOST=app.smoke.internal
CADDY_URL=http://127.0.0.1:18080
ADMIN_URL=http://127.0.0.1:12019

PASS=0
FAIL=0

pass() { printf '  \033[32m✓\033[0m %s\n' "$1"; PASS=$((PASS + 1)); }
fail() { printf '  \033[31m✗\033[0m %s\n' "$1"; printf '      %s\n' "${2:-}"; FAIL=$((FAIL + 1)); }
info() { printf '\n\033[1m%s\033[0m\n' "$1"; }

cleanup() {
  local code=$?
  if [[ "${KEEP:-0}" == "1" ]]; then
    info "KEEP=1 — leaving the stack up. Tear down with:"
    echo "  docker compose -p $PROJECT -f infra/docker-compose.prod.yml -f infra/docker-compose.smoke.yml --env-file $ENV_FILE down -v"
    exit $code
  fi
  info "Tearing down"
  "${COMPOSE[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
  rm -f "$ENV_FILE"
  exit $code
}
trap cleanup EXIT

# --- environment -------------------------------------------------------------

# The container must be in the group that owns the Docker socket, or it starts,
# serves the UI, and can do nothing at all. The GID differs per host, which is
# precisely why hardcoding 999 broke the first VPS deploy (it was 988).
if [[ "$(uname)" == "Darwin" ]]; then
  # Docker Desktop exposes the VM's socket as root:root inside containers; the
  # host-side path is a user-owned symlink and tells us nothing.
  DOCKER_GID=0
else
  DOCKER_GID="$(stat -c '%g' /var/run/docker.sock)"
fi

cat >"$ENV_FILE" <<EOF
BELUNE_IMAGE=$IMAGE
DOCKER_GID=$DOCKER_GID

POSTGRES_USER=belune
POSTGRES_PASSWORD=smoke
POSTGRES_DB=belune

DATABASE_URL=postgres://belune:smoke@postgres:5432/belune?sslmode=disable
REDIS_URL=redis://redis:6379
PORT=8080

JWT_SECRET=$(openssl rand -hex 32)
ENCRYPTION_KEY=$(openssl rand -hex 32)

CADDY_ADMIN_URL=http://caddy:2019
CADDY_CONTAINER_NAME=${PROJECT}-caddy-1
BUILDKIT_HOST=tcp://buildkit:1234
SECURE_COOKIES=false
EOF

# --- boot --------------------------------------------------------------------

if [[ -z "${BELUNE_IMAGE:-}" ]]; then
  info "Building $IMAGE (pass BELUNE_IMAGE=... to skip)"
  docker build -t "$IMAGE" . >/dev/null
fi

info "Starting the production stack"
# stderr is NOT suppressed: a compose failure here (a container that cannot even
# be created) is silent otherwise, and "it printed nothing and did nothing" is
# the exact failure mode this drill exists to catch.
"${COMPOSE[@]}" up -d --wait --wait-timeout 240 >/dev/null || {
  echo "stack did not come up:"
  "${COMPOSE[@]}" ps
  "${COMPOSE[@]}" logs --tail 40 belune 2>&1 || true
  exit 1
}

psql() { "${COMPOSE[@]}" exec -T postgres psql -U belune -d belune -tAc "$1" 2>/dev/null | tr -d '[:space:]'; }
belune_logs() { "${COMPOSE[@]}" logs belune 2>&1; }

# Point the dashboard at a hostname so Caddy has a route to serve.
#
# Also seed one application with a domain. This is not decoration: the access-log
# tailer deliberately skips any hostname that is not a known *application* domain
# ("requests to the PaaS app itself"), so hitting the dashboard alone can never
# produce a request_logs row and an assertion built on it would be testing
# nothing. ssl_mode=off keeps Caddy from chasing a certificate for a name that
# does not resolve.
"${COMPOSE[@]}" exec -T postgres psql -U belune -d belune -v ON_ERROR_STOP=1 -q <<SQL
INSERT INTO settings (key, value) VALUES ('dashboard_domain', '$DASHBOARD_HOST')
  ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

INSERT INTO users (id, email, password_hash)
  VALUES ('11111111-1111-1111-1111-111111111111', 'smoke@belune.invalid', 'x');
INSERT INTO projects (id, name, slug, user_id)
  VALUES ('22222222-2222-2222-2222-222222222222', 'smoke', 'smoke',
          '11111111-1111-1111-1111-111111111111');
INSERT INTO applications (id, project_id, name, slug, type, build_type)
  VALUES ('33333333-3333-3333-3333-333333333333',
          '22222222-2222-2222-2222-222222222222', 'smoke', 'smoke', 'image', 'image');
INSERT INTO domains (id, application_id, hostname, ssl_mode, ssl_enabled)
  VALUES ('44444444-4444-4444-4444-444444444444',
          '33333333-3333-3333-3333-333333333333', '$APP_HOST', 'off', false);
SQL

# --- assertions --------------------------------------------------------------

info "1. The container is healthy"
# Caught: the Dockerfile HEALTHCHECK probed /health, but the route is /healthz —
# so the container was permanently "unhealthy" while working perfectly.
health="$("${COMPOSE[@]}" ps --format '{{.Service}} {{.Health}}' | grep '^belune ' | awk '{print $2}')"
[[ "$health" == "healthy" ]] \
  && pass "belune reports healthy (HEALTHCHECK hits a route that exists)" \
  || fail "belune health is '$health'" "the Dockerfile HEALTHCHECK is probing a route that does not exist"

info "2. Belune can reach the Docker daemon"
# Caught: the socket was never mounted in production, and later the group_add GID
# was wrong. Belune's entire job is managing containers; without this it serves
# the UI and can do nothing.
hz="$(curl -fsS "http://127.0.0.1:18081/healthz" 2>/dev/null || echo '{}')"
if grep -q '"docker"' <<<"$hz" && ! grep -qi '"docker":[^,}]*\(unhealthy\|error\|fail\)' <<<"$hz"; then
  pass "/healthz reports the Docker check passing"
else
  fail "/healthz does not show a healthy Docker check" "$hz"
fi

info "3. Caddy serves the dashboard"
# Caught: the catch-all route dialled localhost:8080 — which is Caddy itself, not
# Belune — so the dashboard was a connection refused behind a working proxy.
body="$(curl -fsS -H "Host: $DASHBOARD_HOST" "$CADDY_URL/" 2>/dev/null || true)"
grep -q 'id="root"' <<<"$body" \
  && pass "the SPA is served through Caddy (the catch-all reaches belune:8080)" \
  || fail "Caddy did not serve the dashboard" "the catch-all upstream is probably wrong"

info "4. The served page has no inline script"
# Caught: an inline <script> in index.html was blocked outright by the API's own
# Content-Security-Policy (script-src 'self'), so it never ran in production —
# silently, with only a console violation no one was reading.
if grep -qE '<script[^>]*>[^<]+</script>' <<<"$body"; then
  fail "index.html contains an inline script" "the CSP (script-src 'self') will block it in production"
else
  pass "no inline script (nothing for the CSP to block)"
fi

info "5. Caddy listens on both :80 and :443"
# Caught: the Caddyfile adapter emits one server per port, so ':80, :443' became
# srv0=:443 + srv1=:80 — stranding every route added later, all of which address
# srv0 by name.
listen="$(curl -fsS "$ADMIN_URL/config/apps/http/servers/srv0/listen" 2>/dev/null || true)"
[[ "$listen" == *':80'* && "$listen" == *':443'* ]] \
  && pass "srv0 holds both listeners ($listen)" \
  || fail "srv0 does not listen on :80 and :443" "got: ${listen:-<nothing>}"

info "6. Certificate sync works against a stock Caddy"
# Caught: a production Caddy has no tls app at all, so listing certificates
# returned 400 rather than the empty set dev returned. SyncCertificates bailed
# before the write — custom certificates were never pushed, i.e. the whole
# feature was dead in production. Dev hid it, because local_certs creates a tls app.
if belune_logs | grep -q "failed to sync certificates"; then
  fail "the proxy reconciler cannot sync certificates" "$(belune_logs | grep -m1 'failed to sync certificates')"
else
  pass "no certificate-sync errors against a stock Caddy"
fi

info "7. Caddy's own logs are being collected"
# Caught: the log collector only watched containers labelled managed-by=belune,
# and Caddy is not one (that label marks containers the cleanup worker may reap).
# So Caddy was never watched — and Caddy's log is where ACME failure reasons live.
# A domain whose certificate failed showed no reason at all.
deadline=$((SECONDS + 60))
sys=0
while [[ $SECONDS -lt $deadline ]]; do
  sys="$(psql "SELECT count(*) FROM container_logs WHERE source_type = 'system'")"
  [[ "${sys:-0}" -gt 0 ]] && break
  sleep 3
done
[[ "${sys:-0}" -gt 0 ]] \
  && pass "Caddy's logs reach container_logs ($sys rows) — ACME reasons can land" \
  || fail "no system logs collected" "the collector is not watching the Caddy container"

info "8. Request logging is alive"
# Caught twice. First, ACCESS_LOG_PATH defaulted to a repo-relative dev path that
# does not exist in the container. Then, once mounted, Caddy writes the log 0600
# root-owned and Belune runs as non-root — so every read failed with "permission
# denied", the tailer retried forever, and request_logs stayed empty. Both were
# silent: zero rows, no error a user would ever see.
# Hit the APPLICATION host: the tailer skips the dashboard's own hostname by
# design, so requests to it would never be recorded.
for _ in 1 2 3 4 5; do curl -sS -o /dev/null -H "Host: $APP_HOST" "$CADDY_URL/" 2>/dev/null || true; done
deadline=$((SECONDS + 60))
reqs=0
while [[ $SECONDS -lt $deadline ]]; do
  reqs="$(psql 'SELECT count(*) FROM request_logs')"
  [[ "${reqs:-0}" -gt 0 ]] && break
  sleep 3
done
if [[ "${reqs:-0}" -gt 0 ]]; then
  pass "requests reach request_logs ($reqs rows)"
else
  perm="$(belune_logs | grep -m1 'permission denied' || true)"
  fail "request_logs is empty after 5 requests" "${perm:-the access-log tailer is reading nothing}"
fi

info "9. Nothing is silently failing in a retry loop"
# The failure mode this whole drill is about: a warning, on a loop, that no one
# reads, while a feature does nothing.
noisy="$(belune_logs | grep -ciE 'permission denied|no such file or directory|tailer error' || true)"
[[ "${noisy:-0}" -eq 0 ]] \
  && pass "no tailer/permission/missing-file errors in the log" \
  || fail "$noisy tailer/permission/missing-file errors in the log" "$(belune_logs | grep -m1 -iE 'permission denied|no such file or directory|tailer error')"

# --- result ------------------------------------------------------------------

info "Result"
printf '  %d passed, %d failed\n' "$PASS" "$FAIL"

if [[ "$FAIL" -ne 0 ]]; then
  # Dump the logs here rather than from CI: the cleanup trap tears the stack down
  # on the way out, so anything that wants to look at it has to look now.
  info "belune logs (last 60 lines)"
  belune_logs | tail -60
  exit 1
fi
echo

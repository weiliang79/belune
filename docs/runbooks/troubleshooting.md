# Troubleshooting Runbook

Common problems and their resolutions for the self-hosted PaaS.

---

## Application stuck in "deploying" or "building"

**Symptoms:** Status badge never leaves `deploying`/`building`; no logs appear.

**Steps:**
1. Check the API worker logs: `docker compose logs -f belune`
2. Look for task queue errors (asynq). If the Redis container is unhealthy the task will never be picked up:
   ```
   docker compose ps redis
   docker compose logs redis --tail 50
   ```
3. If Redis is healthy but no worker is processing, restart the API service:
   ```
   docker compose restart belune
   ```
4. If the build itself failed, check for a recent deployment record in the Deployments tab — the build log should be attached.

---

## Container exits immediately after deploy

**Symptoms:** Application status flips from `running` → `stopped` within seconds.

**Steps:**
1. Open the Logs tab for the application — the last lines before exit usually reveal the root cause.
2. Check the exit code: `docker ps -a | grep <container-name>`
3. Common causes:
   - Missing environment variables — verify in the Env Vars tab.
   - Port mismatch — ensure the application listens on the port specified in the domain config (or `$PORT` if exposed).
   - Health check failing — verify the health check endpoint returns 2xx.

---

## WebSocket disconnects frequently

**Symptoms:** "Disconnected" badge in the logs/metrics view; live updates stop.

**Steps:**
1. Verify the reverse proxy (Caddy) is configured with `header_up Connection upgrade` for WebSocket routes.
2. Check for idle-timeout settings in any intermediate proxies (nginx, load balancers). The PaaS WebSocket client retries with exponential backoff up to 10 times before entering "failed" state.
3. Check the API logs for `websocket upgrade failed` messages.
4. Reload the page to reset the client connection state.

---

## Caddy not issuing TLS certificates

**Symptoms:** HTTPS shows a certificate error; a domain's TLS badge stays on
`Pending` or turns `Failed`.

**Start with the UI.** Click the domain's TLS badge — when issuance fails, the
reason from the ACME server is shown there, and **Settings → Certificates** lists
every domain's TLS status in one table. [`tls.md`](tls.md) explains each status,
the common ACME failures, and the Cloudflare Full (strict) setup. The steps below
are for when the badge itself tells you nothing.

**Steps:**
1. Check Caddy logs: `docker compose logs caddy --tail 100`
2. Ensure the domain DNS A/AAAA record points to the server's public IP. Setting
   `BELUNE_PUBLIC_IP` lets Belune check this for you and report a mismatch on the
   badge.
3. Ensure ports 80 and 443 are open in the firewall and not already bound by another process:
   ```
   ss -tlnp | grep -E ':80|:443'
   ```
4. Check for ACME rate-limit errors. If you've exceeded Let's Encrypt limits, wait or switch to a staging CA for testing.
5. If the `caddydata` volume is corrupt, stop Caddy, remove the volume, and restart — **this will revoke existing certificates**:
   ```
   docker compose stop caddy
   docker volume rm belune_caddydata
   docker compose up -d caddy
   ```

---

## Database connection errors (API fails to start)

**Symptoms:** `belune` container exits with `dial error` or `connection refused`.

**Steps:**
1. Check Postgres is running: `docker compose ps postgres`
2. Verify `DATABASE_URL` in `.env` matches the Postgres service name in `docker-compose.yml`.
3. Check Postgres logs for auth failures: `docker compose logs postgres --tail 50`
4. If the data volume is corrupt (rare), restore from a backup — see the [Disaster Recovery runbook](./disaster-recovery.md).

---

## "403 Forbidden" on WebSocket upgrade

**Symptoms:** Browser console shows `403` on the `/ws` endpoint.

**Cause:** The `Origin` header sent by the browser does not match `CORS_ORIGINS` in `.env`.

**Fix:** Add the frontend origin (e.g. `https://belune.example.com`) to `CORS_ORIGINS`:
```
CORS_ORIGINS=https://belune.example.com
```
Then restart: `docker compose restart belune`

---

## Audit log not recording events

**Symptoms:** The Audit Log page is empty despite activity.

**Steps:**
1. Confirm the `belune` container is healthy — audit writes happen synchronously in request handlers.
2. Check for database write errors in `docker compose logs belune | grep -i audit`.
3. Verify the acting user has a valid session (audit records are skipped if no user context is available).

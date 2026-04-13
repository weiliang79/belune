# Scaling Runbook

Guidance for scaling the self-hosted PaaS as load increases.

---

## Current architecture limits

The default single-node setup runs all services (API, worker, Postgres, Redis, Caddy) on one host. This covers most self-hosted workloads. The limits to be aware of:

| Resource | Default | Notes |
|---|---|---|
| API workers (asynq) | 10 concurrent | Set via `WORKER_CONCURRENCY` in `.env` |
| DB connection pool | 20 max | Set via `DB_MAX_OPEN_CONNS` in `.env` |
| WebSocket clients | Unbounded | Limited by OS file descriptors |
| Container builds | Sequential per queue | Each build runs in a separate goroutine |

---

## Vertical scaling (bigger machine)

The simplest option — move to a larger VM.

1. Create a backup before resizing: `bash scripts/backup.sh`
2. Resize the VM through your cloud provider.
3. After resize, restart services: `docker compose up -d`
4. Increase worker concurrency and DB pool to match the new CPU/RAM:
   ```
   # .env
   WORKER_CONCURRENCY=20
   DB_MAX_OPEN_CONNS=40
   ```
5. Restart: `docker compose restart paas`

---

## Offloading Postgres to a managed database

For production workloads, moving Postgres to a managed service (RDS, Supabase, etc.) is recommended.

1. Take a backup: `bash scripts/backup.sh`
2. Provision the managed Postgres instance and create the `paas` database.
3. Restore the dump to the managed instance:
   ```
   psql "$MANAGED_DATABASE_URL" < postgres.sql
   ```
4. Update `.env`:
   ```
   DATABASE_URL=postgres://user:pass@managed-host:5432/paas?sslmode=require
   ```
5. Remove the local Postgres service from `docker-compose.yml` (or comment it out).
6. Restart: `docker compose up -d`

---

## Offloading Redis to a managed cache

1. Provision a managed Redis instance (ElastiCache, Upstash, etc.).
2. Update `.env`:
   ```
   REDIS_URL=redis://:password@managed-redis-host:6379/0
   ```
3. Remove the local Redis service from `docker-compose.yml` (or comment it out).
4. Restart: `docker compose up -d`

   Note: The asynq task queue uses Redis. In-flight tasks at restart time will be re-queued automatically by asynq's at-least-once delivery guarantee.

---

## Increasing build throughput

If application builds queue up, increase worker concurrency:

```
# .env
WORKER_CONCURRENCY=20
```

Each build goroutine spawns a Docker build process, so ensure the host has enough CPU and disk I/O. Monitor with `docker stats` during peak build activity.

To keep the Docker layer cache warm and speed up repeated builds, ensure the `paas` container mounts the Docker socket from the host (already the default in `docker-compose.yml`).

---

## File descriptor limits

Each WebSocket connection consumes a file descriptor. On busy instances, raise the OS limit:

```bash
# /etc/security/limits.conf
paas  soft  nofile  65536
paas  hard  nofile  65536
```

Or set it in `docker-compose.yml` under the `paas` service:
```yaml
ulimits:
  nofile:
    soft: 65536
    hard: 65536
```

---

## Monitoring resource usage

There is no built-in infrastructure monitoring today. Recommended additions:

- **Container metrics:** Deploy Prometheus + cAdvisor on the same host.
- **Alerting:** Wire Alertmanager to notify on CPU > 80%, memory > 85%, or disk > 90%.
- **Log aggregation:** Ship `docker compose logs` to Loki or a hosted log service.

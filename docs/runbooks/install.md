# Install & First-Run Checklist

End-to-end runbook for bringing a fresh Self-Hosted PaaS install online: DNS,
TLS, first deploy, persistence, and updates. Pair with `scripts/install.sh`,
which handles the boring parts (compose, secrets, image pull) automatically.

---

## 1. Prerequisites

- Linux host with Docker Engine 24+ and Docker Compose v2.
- Root or sudo access on the host (for the systemd unit and port 80/443).
- A domain you control (for TLS via Let's Encrypt). A bare IP works for
  evaluation but not for Caddy automatic TLS.
- Outbound HTTPS to `ghcr.io`, your git host, and Let's Encrypt's ACME
  endpoints.

---

## 2. Run the installer

```sh
curl -sSL https://raw.githubusercontent.com/ungweiliang/selfhost-paas/main/scripts/install.sh | bash
```

This:

- Creates `/opt/paas` (override with `PAAS_DIR=/path bash install.sh`).
- Downloads `docker-compose.yml` and the Caddyfile template.
- Generates a `.env` containing fresh `JWT_SECRET`, `ENCRYPTION_KEY`, and
  Postgres password — keep this file out of version control.
- Pulls `ghcr.io/ungweiliang/selfhost-paas:latest` and runs `docker compose up -d`.
- Waits for `GET /healthz` to return 200.

When it finishes, the panel is reachable at `http://<host>:80` and the API
health probe at `http://<host>:8080/healthz`.

> Reference for every other config option: `.env.defaults` in the repo.

---

## 3. Install the systemd unit (recommended)

The compose stack uses `restart: unless-stopped`, so individual containers
respawn after Docker restarts. The systemd unit's job is to bring the stack
up on host reboot before anyone needs to log in.

```sh
sudo cp /opt/paas/infra/systemd/paas.service /etc/systemd/system/paas.service
sudo systemctl daemon-reload
sudo systemctl enable --now paas.service
```

Verify:

```sh
systemctl status paas.service        # should be 'active (exited)'
docker compose -f /opt/paas/docker-compose.yml ps  # all healthy
```

If you keep the install in a non-default directory, edit
`WorkingDirectory=` in the unit before installing.

---

## 4. DNS

You need at least two records. Replace `paas.example.com` with your
chosen panel hostname.

| Record           | Type  | Target              | Purpose                        |
| ---------------- | ----- | ------------------- | ------------------------------ |
| `paas.example.com` | A   | `<host IP>`         | Dashboard + API                |
| `*.example.com`  | A     | `<host IP>`         | Apps deployed via the platform |

If you plan to use **preview environments**, you also need a wildcard for
the preview subdomain template you configure on each app — typically
`*.preview.example.com → <host IP>`.

**TTL** can be low (300s) while you bootstrap; raise it once stable.

---

## 5. TLS

Caddy handles TLS automatically. Two paths:

### 5a. HTTP-01 (default, simplest)

Once the A records resolve, point a browser at `https://paas.example.com`
and Caddy issues a cert on first visit. Set `TLS_ENABLED=true` and
`SECURE_COOKIES=true` in `/opt/paas/.env`, then:

```sh
sudo systemctl restart paas.service
```

### 5b. DNS-01 (wildcard certs)

HTTP-01 cannot issue `*.example.com`. For wildcard certs (recommended for
preview envs) you need a DNS provider Caddy supports — Cloudflare, Route53,
DigitalOcean, etc. Build a Caddy image bundled with the matching plugin and
edit the Caddyfile to declare the issuer. See the
[Caddy DNS providers wiki](https://github.com/caddyserver/caddy/wiki/Configuring-Caddy-to-Use-a-DNS-Provider).

Verify TLS is live:

```sh
curl -sSI https://paas.example.com/healthz | head -1   # 200 OK
curl -sS https://paas.example.com/healthz              # {"status":"ok"}
```

---

## 6. Bootstrap the admin account

Visit `https://paas.example.com` in a browser. The first-run page asks you
to create an admin account. After that, login is required for everything.

Useful CLI sanity checks:

```sh
docker compose -f /opt/paas/docker-compose.yml logs --tail 50 paas
docker compose -f /opt/paas/docker-compose.yml exec postgres \
    psql -U paas -d paas -c 'SELECT count(*) FROM users;'
```

---

## 7. Sample deploy (smoke test)

Confirm the platform end-to-end with a static container:

1. Create a project named `demo`.
2. Add an application of type **Docker Image** with image `nginx:alpine`.
3. Add a domain on that app: `demo.example.com`.
4. Wait for the deployment to flip to **running** (green badge).
5. `curl -I https://demo.example.com` — expect `200 OK` from nginx and a
   valid TLS cert.

If anything sticks in `pending` or `building`, see
[`troubleshooting.md`](./troubleshooting.md).

---

## 8. Backups

Backups are not enabled by default. The `scripts/backup.sh` helper takes a
`pg_dump` of the `paas` database plus the encrypted secrets needed to
decrypt env vars, then writes a tarball to a directory you choose.

Schedule via cron (or a systemd timer) once you're past the smoke-test
stage:

```sh
# /etc/cron.d/paas-backup
0 3 * * * root /opt/paas/scripts/backup.sh /var/backups/paas >> /var/log/paas-backup.log 2>&1
```

Restore drill (do this at least once before you need it for real):
[`disaster-recovery.md`](./disaster-recovery.md).

---

## 9. Updating

```sh
cd /opt/paas
sudo bash scripts/update.sh
```

This pulls the latest pinned image (read from `PAAS_IMAGE` in `.env`),
re-runs migrations, and waits for `/healthz`. The systemd unit doesn't need
to be restarted — `update.sh` only bounces the `paas` container.

---

## 10. Where to go next

- [`key-rotation.md`](./key-rotation.md) — rotate `ENCRYPTION_KEY` without losing data.
- [`scaling.md`](./scaling.md) — vertical / horizontal headroom and resource caps.
- [`troubleshooting.md`](./troubleshooting.md) — first stop when something is wrong.
- [`disaster-recovery.md`](./disaster-recovery.md) — restore from backup.

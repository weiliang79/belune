# Trialling Belune on a real VPS (pre-release)

The one thing that cannot be verified on a laptop: **real certificate issuance
from Let's Encrypt**. Development uses Caddy's internal CA, because `*.belune.local`
has no public DNS and an ACME challenge could never reach it. Everything about
TLS is proven against that internal CA — the public ACME path is not.

This runbook is for the trial *before* the release pipeline exists. Once images
are published to GHCR, [`install.md`](./install.md) is the real path and this file
becomes redundant.

> **`install.sh` does not work yet.** It downloads compose files from GitHub and
> pulls `ghcr.io/weiliang79/belune:latest`. The repository is not pushed and no
> image is published, so both fail. Build from source instead, as below.

---

## 1. What you need

- A VPS with a **public IPv4**, 2 GB RAM minimum (the image builds Go + the web
  bundle; 1 GB will likely OOM). Ubuntu/Debian is fine.
- Docker Engine + Compose v2.
- **Ports 80 and 443 open to the internet.** Port 80 is not optional — it is how
  Let's Encrypt validates the domain, even though the site runs on 443. On a
  cloud VM check the provider's security group *and* the host firewall; they are
  separate, and the security group is nearly always the one that bites.
- A domain you control, with a DNS `A` record for the panel hostname (e.g.
  `belune.example.com`) pointing at the VPS IP. Let it resolve before you start:
  `dig +short belune.example.com` should return the VPS IP.

## 2. Get the code onto the box

There is no git remote yet, so either push the repo to a (private) GitHub repo and
clone it, or copy it straight from your machine:

```sh
rsync -az --delete \
  --exclude .git --exclude node_modules --exclude .paas-data \
  ./ root@<vps-ip>:/opt/belune/
```

## 3. Create `.env`

`docker-compose.prod.yml` reads `../.env`, so this goes at the repo root
(`/opt/belune/.env`). Generate real secrets — do not reuse the dev ones:

```sh
cd /opt/belune
PG_PASS=$(openssl rand -hex 32)
cat > .env <<EOF
POSTGRES_USER=belune
POSTGRES_PASSWORD=${PG_PASS}
POSTGRES_DB=belune
DATABASE_URL=postgres://belune:${PG_PASS}@postgres:5432/belune?sslmode=disable
REDIS_URL=redis://redis:6379

JWT_SECRET=$(openssl rand -hex 32)
ENCRYPTION_KEY=$(openssl rand -hex 32)

CADDY_ADMIN_URL=http://caddy:2019
PORT=8080

# The API container runs as a non-root user and must join the host group that
# owns the Docker socket, or it cannot manage containers at all.
DOCKER_GID=$(getent group docker | cut -d: -f3)

# Lets Belune check a domain's DNS against this box and tell you it is
# mispointed *before* Let's Encrypt does.
BELUNE_PUBLIC_IP=<vps-ip>

# Rehearse against the ACME staging directory first — see step 4.
BELUNE_CADDY_GLOBAL_OPTIONS=acme_ca https://acme-staging-v02.api.letsencrypt.org/directory
EOF
```

## 4. Rehearse against Let's Encrypt **staging** first

Production Let's Encrypt rate-limits failed validations *per domain, per week*,
and a misconfigured first attempt is easy. The staging directory has far looser
limits and issues an untrusted certificate — which is fine, because we are
testing that issuance *works*, not that a browser trusts it.

That is what the `BELUNE_CADDY_GLOBAL_OPTIONS` line above does. Leave it in for
the first run.

## 5. Build and start

```sh
cd /opt/belune/infra
docker compose -f docker-compose.prod.yml up -d --build
docker compose -f docker-compose.prod.yml ps
```

The API is deliberately **not** published on a public port — Caddy serves it on
:80/:443 and reaches it over the compose network.

If `docker compose ps` shows belune **unhealthy**, or the logs say
`Cannot connect to the Docker daemon at unix:///var/run/docker.sock`, then
`DOCKER_GID` is wrong or missing — the container is running as a non-root user
that is not in the socket's group. Check it with
`getent group docker | cut -d: -f3` and recreate: `docker compose up -d`.

## 6. Bootstrap, then claim the domain

1. Open `http://<vps-ip>` — plain HTTP on the bare IP. This is expected: the
   domain is configured from inside the dashboard, so the first login cannot be
   over HTTPS yet.
2. Create the admin account.
3. Go to **Server → Configuration → Dashboard domain**, enter
   `belune.example.com`, and **Save**.

Belune publishes the hostname to Caddy, which asks the ACME server for a
certificate. Watch the badge under the field:

- **Waiting for certificate** → in flight.
- **HTTPS active** → issued. With staging, the issuer will name Let's Encrypt's
  staging CA.
- **Failed** → the badge carries the actual reason from the ACME server. Work it
  through with [`tls.md`](./tls.md).

Follow along from the box if you want the detail:

```sh
docker compose -f docker-compose.prod.yml logs -f caddy | grep -i "tls\|acme"
```

## 7. Switch to production Let's Encrypt

Once staging issues cleanly, take the training wheels off. Remove the
`BELUNE_CADDY_GLOBAL_OPTIONS` line from `.env`, then **discard the staging
certificates** so Caddy issues fresh ones from the real CA:

```sh
cd /opt/belune/infra
docker compose -f docker-compose.prod.yml down
docker volume rm infra_caddydata      # staging certs live here
docker compose -f docker-compose.prod.yml up -d
```

Reload the dashboard on `https://belune.example.com`. A real browser padlock —
no warning — is the thing this whole exercise exists to prove.

Then set the canonical URL so invitations, password resets and webhooks generate
correct links:

```env
PUBLIC_BASE_URL=https://belune.example.com
TLS_ENABLED=true          # adds the HSTS header
```

Session cookies need no setting; they become `Secure` on their own once the panel
is served over HTTPS.

## 8. Also test an application domain

The dashboard and an app domain take slightly different paths through the proxy,
so prove both:

1. Deploy any app (an `nginx:alpine` image app is enough).
2. Add a domain to it — `demo.example.com`, with an `A` record pointing here.
3. Its TLS badge should reach **Active** within a minute, and
   `curl -I https://demo.example.com` should return a valid certificate.

Then try the failure path deliberately, because the reason-reporting is the whole
point of the feature: add a domain whose DNS points nowhere. It should turn
**Failed** and tell you so, rather than sitting on "pending" forever.

## 9. What a successful trial proves

- Automatic issuance works against the real Let's Encrypt, for both the dashboard
  and an application domain.
- The HTTP→HTTPS redirect is Belune's own, not Caddy's.
- The failure path surfaces the ACME server's actual complaint in the UI.

Until this has been done, do not claim that Automatic TLS works — everything else
is verified only against Caddy's internal CA.

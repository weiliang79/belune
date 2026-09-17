# Changelog

All notable changes to Belune are documented here.

Belune is pre-1.0. The versioning contract while it stays there:

- **Patch releases** (`0.1.0` → `0.1.1`) are always safe to apply **to your
  install**. They never change the deployment topology, never require a host
  action, and never ask anything of you beyond running `update.sh`. Your
  dashboard, your applications, and your data carry over untouched.
- **The API is the one exception, and only until it settles.** A patch release
  may tighten what a personal access token is allowed to reach, or move an
  endpoint that no published reference had promised yet. This reaches scripts
  only — never the dashboard, never your data. Now that the [API
  reference](https://belune.dev/docs/api) is published, a documented path is a
  contract and will not move without a deprecation period; token boundaries may
  still tighten before 1.0. Every such change is listed under **⚠️ Breaking
  changes** at the top of that release's notes, and explained in full here.
- **Minor releases** (`0.1.x` → `0.2.0`) may change the deployment topology
  itself, and may require a host action.
- **Always take a backup before upgrading.** `update.sh` does this for you;
  migrations are forward-only and cannot be undone by downgrading the image.

Release notes for each version are also published on the
[Releases page](https://github.com/weiliang79/belune/releases).

## [0.1.7]

### Belune now has a complete API reference

Every endpoint the API exposes is now documented at
[belune.dev/docs/api](https://belune.dev/docs/api) — what it takes, what it
returns, which scope reaches it, and whether it needs the Admin role. 0.1.6
shipped personal access tokens with no reference to use them against; this is
that gap closed.

- **One page per endpoint**, grouped by what it acts on: Account,
  Applications, Databases, Projects, and Platform.
- **Generated from the running server, not written by hand.** Scopes, role
  requirements, and session boundaries are read off the real router, so the
  reference cannot drift from what the code enforces — a build fails if it
  does.
- **The [OpenAPI 3.1 spec](https://belune.dev/openapi.json) is served at a
  stable public URL**, outside authentication. Point a client generator or a
  linter at it directly. It is more complete than the pages: it carries every
  route, session-only ones included.
- **Two guides for the streaming endpoints** — the multiplexed [WebSocket
  hub](https://belune.dev/docs/api/overview/websockets) behind `GET /api/ws`,
  and the seven [Server-Sent Events
  streams](https://belune.dev/docs/api/overview/server-sent-events) you can
  tail with `curl`.

Endpoints that no token can call are deliberately absent from the reference —
a page for something you cannot call from a script is a page for a reader who
cannot act on it. They are described in [What a token can never
do](https://belune.dev/docs/api#what-a-token-can-never-do) instead.

### Members can see which certificate serves their domains

The TLS status table was admin-only as a whole. A Member could read the
certificate serving one domain from that domain's own badge, but had to open
each application in turn to find which of their domains was failing — the
exact thing an at-a-glance table exists to prevent.

- **The Certificates page is now visible to Members**, showing the Domain TLS
  table for the domains they can already reach: their own projects and any
  shared with them. Uploading, replacing and deleting certificates stay
  admin-only, and the page says so.
- **The certificate picker when adding a domain is no longer empty for a
  Member.** It offers the certificates that can actually serve the hostname
  being added, wildcards included, rather than nothing at all.
- **Domains now show which certificate is serving them**, by name, instead of
  leaving the reader to match a bare id.

The certificate store itself stays admin-only. Listing every certificate
returns every hostname on it, which would hand a Member the domain names of
every other application on the instance — so a Member is given the
certificates that match their own hostname, never the full list.

### Sharing or transferring a project now requires a dashboard session

`PUT /api/projects/{id}/sharing` and `PUT /api/projects/{id}/transfer` no
longer accept a personal access token at any scope, joining [everything else a
token can never do](https://belune.dev/docs/api#what-a-token-can-never-do).
Transferring also now requires the Admin role at the route itself, which is
where the rest of the admin-only routes already declare it.

**Why this one matters more than its size suggests.** Sharing a project grants
every Member on the install the owner's working access to it — including
revealing environment variable values, webhook secrets, deploy-hook tokens,
and file mount contents. A single `{"shared": true}` from a leaked token
exposed all of it. Transfer is the targeted version: it hands one named user
owner access, removes the previous owner, who cannot undo it without an admin,
and moves quota accounting and alert recipients with it.

Both flags can be switched back. The secrets they expose in the meantime
cannot be un-exposed, which is why they belong behind a session rather than
behind a scope.

**Upgrading from 0.1.6: breaking.** Both were `write`-scope token-callable
there. A script that shares or transfers projects needs a session credential.

### Personal access tokens can no longer manage the account itself

A PAT manages infrastructure — deploying, editing environment variables,
running backups — not the account it belongs to. The following now require
a live dashboard session, joining [deleting or restoring data, reading a
stored secret, and minting or revoking tokens, which already
did](https://belune.dev/docs/api#what-a-token-can-never-do):

- Changing your password or profile, and logging out.
- Enrolling, disabling, or checking the status of two-factor authentication,
  and regenerating recovery codes.
- **Listing your own personal access tokens.** The list includes every
  token's scopes and last-used time — read-only, but still a credential
  inventory a leaked token could use to find the one that holds `write`.

Notification preferences were considered and left alone: they're
infrastructure config (deployment failures, resource thresholds — the same
things a token already manages), not account security, so a token can still
read and change them.

None of this closes an account-takeover path: password changes, disabling
TOTP, and regenerating recovery codes already required your current
password (and a current second factor, where one is enrolled) before this.
It's a narrower surface for a leaked token to see, not a vulnerability fix.

**Upgrading from 0.1.6:** all three bullets above were PAT-callable there. If
you have a script that changes its own password, manages TOTP, or lists its
own tokens using a personal access token, it will start getting `403`
instead of `200` — point it at a session credential instead, or drop the
call if it isn't essential. Notification-preference calls are unaffected.

### Platform configuration now requires a dashboard session

Reading or changing platform settings, restarting a service, or opening a
host shell now requires a live dashboard session — a personal access token
is rejected outright, joining [everything else a token can never
do](https://belune.dev/docs/api#what-a-token-can-never-do). Gated:
listing and updating instance settings, the SMTP configuration, restarting a
service, and starting a host shell session. `GET /api/maintenance/server-ip`
stays token-callable — a public fact, not configuration.

**The reason this matters: the settings endpoint writes any key by name.** A
handful of keys are validated (the dashboard's own domain and TLS mode, the
backup schedule, the server IP override); everything else passes straight
through to storage. `host_shell_enabled` — the flag that turns on the in-UI
host shell — is one of those keys. Before this, an admin's write-scoped
token could flip that flag with nobody at a keyboard; opening a session from
there still required the password and a second factor, but a token should
never have been able to reach the switch at all. The same endpoint also sets
the dashboard's own domain and TLS mode — where Caddy gets its certificate
from.

The SMTP and settings-listing endpoints are gated alongside it for
consistency, not because either leaks a credential: the SMTP password was
already masked to a presence flag on read, and the settings list already
skipped it. Reading them is still configuration disclosure and
security-posture reconnaissance — the host shell flag, the dashboard's
domain and TLS mode, the public IP override, the backup schedule — that a
leaked token shouldn't get for free, so they move with the write.

**Upgrading from 0.1.6: breaking.** If you have a script that reads or
writes instance settings, manages the SMTP config, restarts a service, or
opens a host shell using a personal access token, it will start getting
`403` instead of `200` — point it at a session credential instead, or drop
the call if it isn't essential. `GET /api/maintenance/server-ip` is
unaffected.

### Deleting a TLS certificate no longer requires a dashboard session

This loosens a boundary that shipped in 0.1.6. `DELETE /api/certificates/{id}`
was session-only — one of the things [a token could never
do](https://belune.dev/docs/api#what-a-token-can-never-do). It now
also accepts a personal access token that has `write` scope **and** belongs
to an Admin, the same bar as every other write in the platform-admin route
group.

Why this is a narrow relaxation and not a hole:

- `domains.certificate_id` is `ON DELETE RESTRICT`, so a certificate any
  domain still serves cannot be deleted at all — only an unused one is
  reachable.
- The route still requires the Admin role, so a leaked Member token, or any
  token without `write`, still gets `403`.
- A deleted certificate is re-uploadable from the same certificate and key
  that created it — unlike a dropped database, nothing is lost for good.

**Upgrading from 0.1.6:** nothing breaks — this only widens what a token is
allowed to do. If you audit the token boundary, this is the one place it has
moved outward.

### Some API paths have moved

Four paths changed. All four kept their scope and role requirements — the path
is the only thing that moved.

| Was                         | Is now                                  |
| --------------------------- | --------------------------------------- |
| `GET /api/metrics`          | `GET /api/summary`                      |
| `POST /api/cleanup`         | `POST /api/maintenance/cleanup`         |
| `GET /api/proxy/reconciler` | `GET /api/maintenance/proxy`            |
| `POST /api/proxy/reconcile` | `POST /api/maintenance/proxy/reconcile` |

`/api/metrics` returned resource **counts** — how many projects, applications,
databases and deployments exist — while `/api/metrics/*` held real host
time-series and `/metrics` was the Prometheus scrape endpoint. Three unrelated
meanings on one prefix. `/api/metrics/*` now holds only genuine series.

The other three were already described as maintenance operations while living
outside `/api/maintenance`. Note the status endpoint is
`/api/maintenance/proxy`, not `.../proxy/reconciler`: `GET` the noun for
status, `POST` the noun plus a verb for the action, matching
`/api/maintenance/queue` and `/api/maintenance/queue/clear`.

**Upgrading from 0.1.6: breaking, and worth grepping for.** These are the only
paths that have moved since 0.1.6, and they moved now rather than later
precisely because the API reference had not been published yet — no document
had promised them. After this release they are a published contract and will
not move again without a deprecation period. The dashboard is unaffected; it
ships with the binary and already calls the new paths.

### Fixed

- **Deleting an application destroyed its volume backups without saying so.**
  The dialog warned only that the container would stop and the application
  would be deleted. It also erases every volume backup the application had,
  local files and remote objects alike, with no way to keep them — unlike
  deleting a database, which offers that choice. The dialog now states it. The
  behaviour is unchanged; what changes is that you are told before you confirm.
- **An unknown `/api/` path returned `200` and an HTML page instead of a
  `404`.** Anything the API did not recognise fell through to the dashboard's
  own catch-all, which answers unknown paths with the app itself so that
  in-browser navigation works. A caller that typo'd an endpoint got a success
  status and a web page, which most clients fail to parse in some confusing
  way rather than reporting "not found". Unmatched `/api/` paths now return
  `404` with the same `{"error": "..."}` shape as every other API failure;
  dashboard links and page refreshes are unchanged.
- **The settings endpoint accepted any key you sent it.** Seven keys were
  validated and the rest were written as-is, so `host_shel_enabled` — one
  character off the host-shell gate — returned `200`, created a real row, read
  back correctly afterwards, and turned nothing on. `PUT /api/settings` now
  accepts only the 16 keys Belune actually reads, rejecting anything else with
  a `400` that names what it will take, and type-checks the nine keys that
  previously had no validation at all. This is only reachable by an admin with
  a dashboard session, so it is a correctness fix rather than a security one.

### Upgrading

This release adds **no migrations** and changes nothing about how Belune is
deployed — no new containers, no compose changes, no host action. `update.sh`
takes a backup first, as always.

**The dashboard is entirely unaffected.** It authenticates with a session
cookie, so every boundary change above is invisible to it. Nothing you do in
the browser changes.

**If you have a script using a personal access token, read the breaking
entries above.** In short: point anything that manages the account, reads or
writes platform configuration, or shares or transfers a project at a session
credential instead, and update the four moved paths. Everything else keeps
working unchanged.

## [0.1.6]

### Projects can now be shared with your team

**Two Members could not collaborate on a project before this.** A project had
one owner and nothing else, so a team reached for the only lever available:
promote everyone to Admin just to let them work together. A project owner can
now share a project with every Member on the install instead.

- A single switch on the project's Settings tab, off by default — nothing
  changes for an existing project until its owner turns it on.
- A shared Member gets full working access: deploy, create databases, edit
  environment variables, all of it.
- **Destructive rights stay owner-only.** Deleting an application, database,
  or domain, transferring the project, and unsharing it all still require
  being the owner or an admin — sharing widens who can use a project, not who
  can destroy it.
- A Member who owns a project can share it themselves; it doesn't need an
  admin.

### Personal access tokens

Belune now has API credentials that aren't your login. Create one from
**Account → Personal Access Tokens**, [documented here](https://belune.dev/docs/api):

- **Four scopes on a ladder** — `metrics` ⊂ `read` ⊂ `deploy` ⊂ `write` —
  each including everything narrower than it, so a token only ever needs one
  rung. A monitoring scraper wants `metrics`; a CI job that only deploys
  wants `deploy`.
- **Expiry: 1, 7, 14, 30, 60, or 90 days, or never** — 30 days by default.
- Shown exactly once at creation. There's no "regenerate" — create a new
  token, confirm it works, then delete the old one, so there's never a gap
  where nothing has access.
- The token list shows when each one was last used, so one nobody's touched
  in months is easy to spot and retire.
- Every audit-log entry now records which token performed an action, or that
  it was you directly.

By design, **no token can ever delete or restore anything, read a stored
secret** (database credentials, deploy-hook tokens, webhook secrets,
environment variables, file mount contents), **mint or revoke a token,
manage users, or open a terminal** — those always require a live dashboard
session, whatever scope the token carries. This isn't a capability being
taken away — personal access tokens are new in this release, so there's
nothing that previously worked with one. It's the ceiling the new credential
ships with: a leaked token is a bad day, not a catastrophe.

### Changed

- **Resetting another user's password now asks for the acting admin's own
  password first.** Resetting another user's two-factor authentication asks
  for the admin's password, and a fresh code too if the admin has 2FA
  enrolled themselves. Both show up as a new prompt on the Team page —
  expect it, it isn't a bug.
- **`GET /api/projects/{id}/databases/{id}` no longer returns connection
  credentials.** They live at a new endpoint,
  `.../databases/{id}/credentials/reveal`, which — like every endpoint that
  hands back a decrypted secret — requires a live session. The dashboard
  already calls the new endpoint; anyone who was reading the `credentials` or
  `connection_string` field off the old response, with a session cookie,
  needs to switch to it.

### Upgrading

This release adds two migrations (project sharing, personal access tokens).
`update.sh` takes a backup first, as always. Both are purely additive —
sharing defaults to off for every existing project, and there is nothing to
migrate for a credential type that didn't exist before.

## [0.1.5]

### Backups now outlive the database they came from

**Deleting a database no longer destroys its backups.** Until now it did, and
v0.1.3 only made that consented rather than silent — but consent to an
irreversible mistake is still irreversible, and the moment you want yesterday's
backup is right after deleting the database by accident.

- The delete dialog offers **"Also delete these backups"**, unchecked. The API
  mirrors it: backups are kept unless `delete_backups=true`, so a script or an
  older client keeps the data rather than destroying it.
- Kept backups appear under the project's **Backups** page, in a new section for
  backups whose database is gone. They would otherwise be invisible — the
  per-database page went with the database — while still costing storage.
- **Restore a replacement** from any of them. The database comes back under its
  **original name and credentials**, so applications in the project reconnect
  with no configuration change. This matters more than it looks: attaching a
  database injects no connection variables, so a replacement under a new name
  would leave every dependent application pointing at a host that is not there.
- Kept backups **expire 90 days after the database was deleted**, so keeping by
  default cannot quietly grow remote storage forever. Change it with the
  `orphaned_backup_retention_days` setting, or set it to `0` to keep everything
  and decide by hand.

**Deleting a project still destroys everything in it**, including kept backups —
unchanged, and now stated more fully in its confirmation dialog.

### Fixed

- **Deleting an application left its volume backups behind.** The database rows
  cascaded away with the volumes, but the archives stayed in their destination
  with nothing left recording where they were: unreachable, unprunable, and
  still billed. They are now erased with the application. If you have deleted
  applications that had volume backups, objects from before this release are
  still in your destination and need removing by hand — Belune no longer has a
  record of their keys.
- The daily orphan-container sweep now identifies containers by the labels they
  carry rather than by name, so a container whose name has changed is no longer
  invisible to it, and managed databases are covered structurally rather than by
  a list someone has to remember to update. It also sweeps each server against
  what is placed on it, rather than one host against every row.
- Live log, metric and notification streams no longer die with a nil-pointer
  panic when their Redis subscription ends — which happens on any ordinary API
  restart while a stream is open.

### Upgrading

This release adds a migration. `update.sh` takes a backup first, as always.
Nothing existing is rewritten: the new columns are empty for every backup you
already have, and every one of them keeps reading exactly as it did before.

## [0.1.0] — first public release

The first release published to GHCR and the first version installable with the
one-line installer. Everything below already existed across 36 alpha
iterations; this is what Belune _is_ at launch, not a list of what changed.

### Deploying applications

- Deploy from a **git repository** — GitHub, GitLab, Gitea, or Bitbucket — via
  GitHub App installations, OAuth connections, or a personal access token.
- Builds with **Railpack** (default), **Cloud Native Buildpacks**, or your own
  **Dockerfile**, with a persistent build cache per application.
- Deploy from a **prebuilt image**, with the digest pinned so a redeploy is
  reproducible.
- **Automatic deploys**: push webhooks with per-branch filtering, and per-app
  **deploy hooks** — a tokenised URL your CI can call after publishing an image.
- Staged deploys with **health verification and automatic rollback**: the new
  image is built before the running container is replaced, and a failed health
  check reverts to the previous deployment.
- Rollback to any previous deployment, and per-deployment build logs.

### One-click apps

- A catalog of **app templates** that instantiate native Belune objects —
  applications, managed databases, volumes, environment variables, and a domain
  — so a templated app gets the same backups, upgrades, and observability as
  anything else.
- Templates are declarative manifests; adding one needs no Go or React.

### Managed databases

- **PostgreSQL, MySQL, MongoDB and Redis**, plus a generic "other" type for any
  database image.
- **Scheduled backups** to S3-compatible storage with retention, plus manual
  backups and in-app restore.
- **Guarded major-version upgrades**: dump, verify, migrate, and roll back
  automatically if anything fails.
- Optional external access over a loopback-bound port for SSH tunnelling.

### Networking and TLS

- **Automatic HTTPS** via Caddy and Let's Encrypt, plus upload of custom
  certificates (Cloudflare Origin CA and friends).
- **Per-domain TLS status** — issued, expiring, failed — with the actual reason
  for a failure, including DNS misconfiguration detected before issuance is
  attempted.
- Path-based routing, HTTP→HTTPS redirects, and per-project network isolation.

### Storage

- **Volumes** and **file mounts** with content managed in Belune.
- Volume **snapshot backups** to S3 on a schedule, with restore.

### Operations

- Per-container **logs** grouped into deployment sessions, with severity levels
  and search; platform logs for Belune's own services.
- **Metrics** for host and containers, with history.
- A browser **terminal** into any application container.
- **Notifications** to Discord, Telegram, Slack, ntfy, Gotify, generic webhooks,
  or email when deploys, backups, restores, or certificates need attention.
- **Audit log**, per-user quotas, role-based access, disk cleanup, and
  read-only Docker views.
- System backup and documented disaster recovery.

### Security

- Envelope encryption with key rotation for all secrets at rest.
- Refresh tokens, login lockout, session revocation, CSRF protection.
- Containers run with dropped capabilities, no new privileges, and a read-only
  root filesystem by default.
- Optional, off-by-default host shell for break-glass recovery — admin-only,
  re-authenticated, and fully audited.

### Known limitations at 0.1.0

- **Single server.** Multi-server support is planned but not present.
- **No Docker Compose import.** Templates cover common stacks natively; compose
  import is on the roadmap.
- **Preview environments** exist but are unlinked in the UI pending completion.
- Registry credentials for private images, monorepo subdirectory builds, and
  custom start commands are not yet configurable.

[0.1.7]: https://github.com/weiliang79/belune/releases/tag/v0.1.7
[0.1.6]: https://github.com/weiliang79/belune/releases/tag/v0.1.6
[0.1.5]: https://github.com/weiliang79/belune/releases/tag/v0.1.5
[0.1.0]: https://github.com/weiliang79/belune/releases/tag/v0.1.0

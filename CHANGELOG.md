# Changelog

All notable changes to Belune are documented here.

Belune is pre-1.0. The versioning contract while it stays there:

- **Patch releases** (`0.1.0` → `0.1.1`) are always safe to apply.
- **Minor releases** (`0.1.x` → `0.2.0`) may contain breaking changes.
- **Always take a backup before upgrading.** `update.sh` does this for you;
  migrations are forward-only and cannot be undone by downgrading the image.

Release notes for each version are also published on the
[Releases page](https://github.com/weiliang79/belune/releases).

## [Unreleased]

<!-- Entries land here between releases; the release workflow generates the
     published notes from the conventional-commit log. -->

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

[Unreleased]: https://github.com/weiliang79/belune/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/weiliang79/belune/releases/tag/v0.1.0

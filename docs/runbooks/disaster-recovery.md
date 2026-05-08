# Disaster Recovery Runbook

Procedures for recovering the self-hosted PaaS from data loss, host failure, or corruption.

---

## Recovery Time / Point Objectives (targets)

| Scenario | RTO target | RPO target |
|---|---|---|
| Accidental data deletion | < 30 min | Last backup |
| Host hardware failure | < 1 hour | Last backup |
| Full datacenter loss | < 2 hours | Last backup |

These targets assume a recent backup exists and a replacement host is available.

---

## Backup strategy

The platform runs `scripts/backup.sh` on a built-in daily schedule (default
02:00 UTC) with automatic rotation. Archives contain:
- Full Postgres SQL dump
- Caddy TLS data (certificates + config)
- `.env` file

**Local backups** are written to `PAAS_DIR/backups/` (default
`/opt/paas/backups/`) and rotated according to `BACKUP_RETAIN_DAYS` (30) and
`BACKUP_RETAIN_COUNT` (14).

**Remote backups** (recommended for production) upload each archive to
S3-compatible object storage automatically. Enable with `BACKUP_REMOTE_ENABLED=true`
and the `BACKUP_S3_*` variables — see [`install.md` § 8](./install.md#8-backups)
for the full variable list.

Backup status and a manual trigger are available in the dashboard under
**Settings → Backups**.

### Encrypted backups

Set `BACKUP_ENCRYPTION_KEY` in `.env` to an `age` public key to encrypt archives:

```bash
# Generate a keypair
age-keygen -o ~/.age/paas.key         # keep the private key safe, off-server
# ~/.age/paas.key contains the public key on the first line as a comment

# Add the public key to .env
BACKUP_ENCRYPTION_KEY=age1ql3z7hjy54pw...
```

Encrypted archives have the `.tar.gz.age` extension. Store the private key (`~/.age/paas.key`) separately from the encrypted backups.

---

## Scenario 1: Accidental data deletion

Single table or row was deleted accidentally; the host is still running.

1. **Stop the API** to prevent further writes:
   ```bash
   docker compose stop paas
   ```
2. Identify the most recent backup containing the data.
3. Extract the Postgres dump from the backup to a temporary location:
   ```bash
   tar -xzf paas-backup-<timestamp>.tar.gz
   # or decrypt first:
   # bash scripts/restore.sh paas-backup-<timestamp>.tar.gz.age ~/.age/paas.key
   ```
4. Restore only the affected table from the dump (using `pg_restore` selective restore, or by copying INSERT statements manually).
5. Restart the API: `docker compose start paas`

---

## Scenario 2: Full host restore (new server)

The original host is lost. A new server has been provisioned.

### Prerequisites
- New host with Docker and Docker Compose installed.
- A copy of the most recent backup archive.
- The age private key file (if backups are encrypted).

### Steps

1. **Transfer the backup** to the new host:
   ```bash
   scp paas-backup-<timestamp>.tar.gz[.age] user@new-host:/tmp/
   ```

2. **Run the installer** to set up the directory structure:
   ```bash
   bash scripts/install.sh
   ```
   This creates `/opt/paas` with `docker-compose.yml` and a template `.env`.

3. **Restore the backup**:
   ```bash
   # Unencrypted:
   bash /opt/paas/scripts/restore.sh /tmp/paas-backup-<timestamp>.tar.gz

   # Encrypted:
   bash /opt/paas/scripts/restore.sh /tmp/paas-backup-<timestamp>.tar.gz.age ~/.age/paas.key
   ```

   `restore.sh` will:
   - Decrypt the archive (if `.age`)
   - Restore `.env`
   - Drop and recreate the Postgres database from the dump
   - Restore Caddy TLS data
   - Restart the `paas` service

4. **Verify DNS** points to the new host's IP for all configured domains.

5. **Verify TLS** — Caddy will attempt to renew certificates if they are near expiry. Check logs:
   ```bash
   docker compose logs caddy --tail 50
   ```

6. **Smoke test** — log in, check the dashboard, deploy a test application.

---

## Scenario 3: Corrupt Postgres volume

Symptoms: Postgres container crash-loops, logs show `invalid page` or `could not read block`.

1. **Stop all services**: `docker compose down`
2. **Remove the corrupt volume**: `docker volume rm paas_pgdata`
3. **Start Postgres only**: `docker compose up -d postgres`
4. **Restore from the latest backup**:
   ```bash
   bash scripts/restore.sh paas-backup-<timestamp>.tar.gz[.age] [identity-file]
   ```
5. Bring up remaining services: `docker compose up -d`

---

## Verifying a backup

Before you need it, verify a backup is readable:

```bash
# Check archive integrity
tar -tzf paas-backup-<timestamp>.tar.gz | head

# Decrypt and check (encrypted)
age --decrypt -i ~/.age/paas.key paas-backup-<timestamp>.tar.gz.age | tar -tz | head

# Check SQL dump is non-empty
tar -xzf paas-backup-<timestamp>.tar.gz --strip-components=1 '*/postgres.sql' -O | head -20
```

---

## Backup retention policy

Rotation is handled automatically by the built-in scheduler. The defaults keep
the 14 most recent backups and any backup newer than 30 days (whichever is
more). Override in `.env`:

```env
BACKUP_RETAIN_DAYS=30
BACKUP_RETAIN_COUNT=14
```

For S3 remote backups the same policy applies to objects in the configured
bucket prefix — old objects are deleted automatically after each successful run.

If you need to trigger a one-off backup outside the schedule, either use the
**Settings → Backups → Run Backup Now** button in the dashboard or run the
script directly:

```bash
bash /opt/paas/scripts/backup.sh
```

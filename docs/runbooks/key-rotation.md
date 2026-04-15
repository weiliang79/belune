# Encryption Key Rotation

This runbook covers rotating the KEK (Key Encryption Key) used to wrap
per-row DEKs (Data Encryption Keys) for all encrypted secrets: git
credentials, application git tokens, database credentials, domain SSL
credentials, and project/app environment variables.

## When to rotate

- A KEK is suspected to be compromised.
- Scheduled rotation per compliance policy (e.g. yearly).
- A staff member with KEK access has left.

## How the keyring works

The API reads its keyring from two environment variables:

- `ENCRYPTION_KEYS` — comma-separated list of versioned KEKs, e.g.
  `v1:<64-hex>,v2:<64-hex>`. Each value is exactly 32 bytes (64 hex chars).
- `ENCRYPTION_KEY_CURRENT` — which version to use for **new** encryptions,
  e.g. `v2`. If unset, the highest version in `ENCRYPTION_KEYS` is used.

For backward compatibility, a single legacy `ENCRYPTION_KEY=<64-hex>` env
var is still accepted and treated as `v1`.

Each encrypted value in the database is an envelope containing:

```
"PaaS\x01" || kek_ver || dek_nonce || wrapped_dek || data_nonce || data_ct
```

Rotation only needs to rewrap the DEK under the new KEK — the underlying
plaintext never moves through memory except at the boundary, and rows
still tagged with the old KEK continue to decrypt transparently until
rewrapped.

## Rotation procedure

### 1. Generate a new KEK

```bash
openssl rand -hex 32
```

Record the output as the new KEK material. Do **not** paste it into chat
or shared scratchpads.

### 2. Add the new KEK to the environment

Update the API process environment so both old and new keys are present:

```bash
# /etc/paas.env or equivalent
ENCRYPTION_KEYS=v1:<old_hex>,v2:<new_hex>
ENCRYPTION_KEY_CURRENT=v2
```

Restart (or `kill -HUP`, depending on your deployment) the API process.
New writes now use v2; old rows still decrypt under v1.

Verify the API started cleanly — a bad key format will fail fast at boot.

### 3. Dry-run the rewrap tool

```bash
task encryption:rewrap -- --dry-run
```

Expected output: per-table scanned/rewrapped/skipped counts. `skipped_current`
should equal rows that were written **after** the v2 switch in step 2;
`rewrapped` should equal rows still tagged with v1 (or the pre-keyring
legacy format).

### 4. Run the rewrap

```bash
task encryption:rewrap
```

The tool:

- Scans `git_credentials`, `applications`, `databases`, `domains`,
  `env_vars`, `project_env_vars`.
- For each row with a non-current KEK tag (including legacy rows), decrypts
  under the old KEK, re-seals under the current KEK, and updates in place.
- Skips rows already tagged with the current KEK (idempotent — safe to
  re-run).

Re-run `task encryption:rewrap -- --dry-run` afterward to confirm
`rewrapped` is now 0 for every table.

### 5. Retire the old KEK

Once every row reports as current, remove the old KEK from the environment:

```bash
ENCRYPTION_KEYS=v2:<new_hex>
# ENCRYPTION_KEY_CURRENT=v2 (optional now; v2 is the only key)
```

Restart the API. If any row was missed, the API will fail to decrypt it
and log an error — run the rewrap tool again with both keys present before
retiring the old KEK.

**Important:** keep the old KEK in a secure secret store for at least as
long as the oldest restorable backup. Point-in-time restores will bring
back v1-tagged rows and require v1 to decrypt them.

## Verification

After rotation, smoke-test an encrypted flow end-to-end:

1. Open an application's env vars page — non-secret values should render
   correctly.
2. Trigger a deploy for an application with git credentials — clone must
   succeed.
3. Open a provisioned database — its credentials should render.

Any decryption error in these flows is a rotation failure — investigate
before removing the old KEK.

## Disaster: lost the new KEK

If you added v2 to `ENCRYPTION_KEYS` but the new KEK material is lost
before the rewrap runs, **no data is lost**: rows are still v1-tagged.
Remove v2 from `ENCRYPTION_KEYS`, restart, and start over from step 1 with
fresh v2 material.

If you removed v1 from the environment **before** completing the rewrap,
restore v1 from your secret store and re-run the rewrap. Any row still
tagged with v1 will be unreadable until v1 is back in the keyring.

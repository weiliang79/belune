-- The webhook secret was stored in plaintext, so a database dump handed over
-- working push-deploy credentials. It joins the other secrets under the
-- keyring (git credentials, deploy-hook token, file-mount contents).
--
-- The plaintext column stays for now and is emptied by the backfill that runs
-- at startup: encryption needs the keyring, which SQL cannot reach. Reads fall
-- back to the plaintext column while any row is still un-backfilled, so an
-- interrupted upgrade cannot break push deploys. Dropping the old column is a
-- later migration, once no deployment can still be mid-backfill.

ALTER TABLE applications
    ADD COLUMN webhook_secret_encrypted BYTEA;

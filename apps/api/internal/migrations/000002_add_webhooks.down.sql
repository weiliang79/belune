ALTER TABLE services
  DROP COLUMN IF EXISTS webhook_secret,
  DROP COLUMN IF EXISTS auto_deploy_branch;

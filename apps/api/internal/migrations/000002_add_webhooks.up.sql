ALTER TABLE services
  ADD COLUMN webhook_secret VARCHAR(255),
  ADD COLUMN auto_deploy_branch VARCHAR(255) DEFAULT 'main';

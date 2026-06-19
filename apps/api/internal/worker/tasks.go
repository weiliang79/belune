package worker

const (
	TypeDeploy              = "deploy"
	TypeBuild               = "build"
	TypeCleanup             = "cleanup"
	TypeProvisionDB         = "provision_db"
	TypeReconfigureDB       = "reconfigure_db"
	TypeBackupDB            = "backup_db"
	TypeRestoreDB           = "restore_db"
	TypeUpgradeDB           = "upgrade_db"
	TypeRetentionCleanup    = "retention_cleanup"
	TypeEmailSend           = "email:send"
	TypeAuthTokenCleanup    = "auth:token_cleanup"
	TypeQuotaThresholdSweep = "quota:threshold_sweep"
	TypeBackupNow           = "backup:now"
	TypeBackupRotate        = "backup:rotate"
)

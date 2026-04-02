package status

// Deployment statuses
const (
	DeploymentPending   = "pending"
	DeploymentBuilding  = "building"
	DeploymentDeploying = "deploying"
	DeploymentSuccess   = "success"
	DeploymentFailed    = "failed"
)

// Application statuses
const (
	ApplicationInactive = "inactive"
	ApplicationRunning  = "running"
	ApplicationStopped  = "stopped"
	ApplicationError    = "error"
)

// Database statuses
const (
	DatabaseCreating = "creating"
	DatabaseRunning  = "running"
	DatabaseStopped  = "stopped"
	DatabaseFailed   = "failed"
)

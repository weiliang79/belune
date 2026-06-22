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
	DatabaseCreating  = "creating"
	DatabaseRunning   = "running"
	DatabaseStopped   = "stopped"
	DatabaseFailed    = "failed"
	DatabaseUpgrading = "upgrading"
	DatabaseBackingUp = "backing_up"
)

// validDeploymentTransitions defines the allowed deployment status state machine.
// The zero value (empty from-state) acts as a wildcard allowing any → failed.
var validDeploymentTransitions = map[string]map[string]bool{
	DeploymentPending:   {DeploymentBuilding: true, DeploymentDeploying: true},
	DeploymentBuilding:  {DeploymentDeploying: true, DeploymentFailed: true},
	DeploymentDeploying: {DeploymentSuccess: true, DeploymentFailed: true},
	// Terminal states have no valid forward transitions.
	DeploymentSuccess: {},
	DeploymentFailed:  {},
}

// ValidTransition reports whether transitioning a deployment from `from` to `to`
// is a legal state machine step. Any state may transition to failed (error catch-all).
func ValidTransition(from, to string) bool {
	if to == DeploymentFailed {
		return true // any → failed is always permitted
	}
	allowed, ok := validDeploymentTransitions[from]
	if !ok {
		return false
	}
	return allowed[to]
}

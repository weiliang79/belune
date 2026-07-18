// Package template implements the app-template catalog: declarative manifests
// that instantiate native Belune objects (prebuilt-image applications, managed
// databases, volumes, env vars, domains). Manifests are embedded into the
// binary via go:embed so the catalog works offline/air-gapped.
//
// A manifest is deliberately NOT a compose file: templated apps get backups,
// guarded upgrades, logs and TLS automatically because every object is a real
// first-class Belune resource. The future compose importer will translate INTO
// this same representation.
package template

// SchemaVersion is the only manifest schema version this build understands.
const SchemaVersion = 1

// Manifest is one catalog entry. Field names stay compose-adjacent (image / env
// / volumes) so authoring feels familiar and a future compose importer maps 1:1.
type Manifest struct {
	Schema      int    `json:"schema" yaml:"schema"`
	ID          string `json:"id" yaml:"id"`
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Category    string `json:"category" yaml:"category"`
	// LogoURL is author-provided; the frontend falls back to a lucide icon when
	// absent. v1 passes the URL through; build-time logo vendoring is a
	// post-launch follow-up.
	LogoURL string `json:"logo_url,omitempty" yaml:"logo_url,omitempty"`
	Website string `json:"website,omitempty" yaml:"website,omitempty"`
	// Version is a human-facing display version (e.g. "5" for Ghost 5.x). It is
	// author-maintained catalog metadata, decoupled from the pinned image tags —
	// necessary because a multi-service template has several images with no single
	// canonical version.
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
	// DocsURL points at the app's setup/usage documentation.
	DocsURL   string     `json:"docs_url,omitempty" yaml:"docs_url,omitempty"`
	Tags      []string   `json:"tags,omitempty" yaml:"tags,omitempty"`
	Inputs    []Input    `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	Databases []Database `json:"databases,omitempty" yaml:"databases,omitempty"`
	Services  []Service  `json:"services" yaml:"services"`
	// Notes is markdown shown on the completion screen (placeholders resolved).
	Notes string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

// Input is a value the wizard asks the user for, referenced as {{input.KEY}}.
type Input struct {
	Key         string `json:"key" yaml:"key"`
	Label       string `json:"label" yaml:"label"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool   `json:"required,omitempty" yaml:"required,omitempty"`
	// Validation is one of "", "email", "url".
	Validation string `json:"validation,omitempty" yaml:"validation,omitempty"`
	Default    string `json:"default,omitempty" yaml:"default,omitempty"`
}

// Database is a managed database the template provisions. Its declared name is
// the second segment of {{db.NAME.*}} placeholders.
type Database struct {
	Name string `json:"name" yaml:"name"`
	// Engine is one of postgres, mysql, redis, mongo, other.
	Engine  string `json:"engine" yaml:"engine"`
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
}

// Volume is a persistent named volume mounted into a service.
type Volume struct {
	Name      string `json:"name" yaml:"name"`
	MountPath string `json:"mount_path" yaml:"mount_path"`
}

// Service is a prebuilt-image application. The domain (when the user supplies a
// hostname) routes to the first service that declares a port.
type Service struct {
	Name            string            `json:"name" yaml:"name"`
	Image           string            `json:"image" yaml:"image"`
	Port            int32             `json:"port,omitempty" yaml:"port,omitempty"`
	Env             map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	Volumes         []Volume          `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	HealthCheckPath string            `json:"health_check_path,omitempty" yaml:"health_check_path,omitempty"`
	// DependsOn lists database or service names that must be provisioned/deployed
	// first. It only affects ordering.
	DependsOn []string `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
}

// validEngines is the set of managed-database engines a template may request.
var validEngines = map[string]bool{
	"postgres": true,
	"mysql":    true,
	"redis":    true,
	"mongo":    true,
	"other":    true,
}

// validValidations is the set of input validation rules.
var validValidations = map[string]bool{
	"":      true,
	"email": true,
	"url":   true,
}

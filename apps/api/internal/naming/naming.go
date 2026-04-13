package naming

import (
	"fmt"
	"regexp"
	"strings"
)

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify converts a name to a URL/container-safe slug.
func Slugify(name string) string {
	s := strings.ToLower(name)
	s = nonAlphanumeric.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// ContainerName returns the container name.
// The appSlug stored in the DB is already the fully-qualified container name
// in the format "{projectSlug}-{baseSlug}-{applicationID[:8]}", so it is
// returned directly. The other parameters are kept for call-site compatibility.
func ContainerName(_, appSlug, _ string) string {
	return appSlug
}

// IntermediateContainerName returns the previous naming format: "{projectSlug}-{applicationID[:8]}"
func IntermediateContainerName(projectSlug, applicationID string) string {
	return fmt.Sprintf("%s-%s", projectSlug, applicationID[:8])
}

// OldContainerName returns the legacy container name: "paas-{applicationID[:8]}"
func OldContainerName(applicationID string) string {
	return fmt.Sprintf("paas-%s", applicationID[:8])
}

// ImageTag returns the image tag: "{appSlug}:{deploymentID[:8]}"
// appSlug is already the fully-qualified name "{projectSlug}-{baseSlug}-{applicationID[:8]}".
func ImageTag(_, appSlug, _, deploymentID string) string {
	return fmt.Sprintf("%s:%s", appSlug, deploymentID[:8])
}

// ProjectNetworkName returns the Docker network name scoped to a project: "paas-{projectSlug}"
func ProjectNetworkName(projectSlug string) string {
	return fmt.Sprintf("paas-%s", projectSlug)
}

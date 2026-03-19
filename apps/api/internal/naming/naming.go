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

// ContainerName returns the container name: "{projectSlug}-{appSlug}-{applicationID[:8]}"
func ContainerName(projectSlug, appSlug, applicationID string) string {
	return fmt.Sprintf("%s-%s-%s", projectSlug, appSlug, applicationID[:8])
}

// IntermediateContainerName returns the previous naming format: "{projectSlug}-{applicationID[:8]}"
func IntermediateContainerName(projectSlug, applicationID string) string {
	return fmt.Sprintf("%s-%s", projectSlug, applicationID[:8])
}

// OldContainerName returns the legacy container name: "paas-{applicationID[:8]}"
func OldContainerName(applicationID string) string {
	return fmt.Sprintf("paas-%s", applicationID[:8])
}

// ImageTag returns the image tag: "{projectSlug}-{appSlug}-{applicationID[:8]}:{deploymentID[:8]}"
func ImageTag(projectSlug, appSlug, applicationID, deploymentID string) string {
	return fmt.Sprintf("%s-%s-%s:%s", projectSlug, appSlug, applicationID[:8], deploymentID[:8])
}

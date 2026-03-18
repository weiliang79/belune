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

// ContainerName returns the container name: "{projectSlug}-{appSlug}-{serviceID[:8]}"
func ContainerName(projectSlug, appSlug, serviceID string) string {
	return fmt.Sprintf("%s-%s-%s", projectSlug, appSlug, serviceID[:8])
}

// IntermediateContainerName returns the previous naming format: "{projectSlug}-{serviceID[:8]}"
func IntermediateContainerName(projectSlug, serviceID string) string {
	return fmt.Sprintf("%s-%s", projectSlug, serviceID[:8])
}

// OldContainerName returns the legacy container name: "paas-{serviceID[:8]}"
func OldContainerName(serviceID string) string {
	return fmt.Sprintf("paas-%s", serviceID[:8])
}

// ImageTag returns the image tag: "{projectSlug}-{appSlug}-{serviceID[:8]}:{deploymentID[:8]}"
func ImageTag(projectSlug, appSlug, serviceID, deploymentID string) string {
	return fmt.Sprintf("%s-%s-%s:%s", projectSlug, appSlug, serviceID[:8], deploymentID[:8])
}

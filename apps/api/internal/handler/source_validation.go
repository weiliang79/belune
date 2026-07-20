package handler

import (
	"errors"
	"fmt"
	"strings"
)

// Where an application comes from is described by four fields that have to
// agree with each other. Nothing enforced that agreement: the database CHECKs
// each column's value in isolation, and the deploy worker switches on `type`
// alone, so a field belonging to the other kind of source was accepted, stored,
// and then silently ignored forever.
//
// The two failures this closes, both observed rather than theorised:
//
//   - Clearing source_image on an image application stored NULL, because the
//     update maps "" to NULL. The save succeeded and the next deploy failed on
//     an empty pull, far away from the edit that caused it.
//
//   - Setting source_repo on an image application was accepted and ignored. A
//     push webhook then matched the app by repository URL and "successfully"
//     deployed it — by re-pulling the same image, never touching the repo. A
//     green deployment that built nothing is worse than an error.
//
// Rejecting at the API boundary keeps the failure next to the edit, and keeps
// the invariant in one place instead of spread across every consumer that has
// to guess which fields are meaningful.

var (
	validTypes      = []string{"git", "image"}
	validBuildTypes = []string{"dockerfile", "buildpacks", "railpack", "image"}
)

func oneOf(value string, allowed []string) bool {
	for _, a := range allowed {
		if value == a {
			return true
		}
	}
	return false
}

// sourceFields is the effective state being validated. On create every field
// comes from the request; on update, type and build_type come from the stored
// row, because neither is updatable (until item 6 makes the switch an explicit
// action).
type sourceFields struct {
	Type              string
	BuildType         string
	BuildTypeOverride string
	DockerfilePath    string
	SourceRepo        string
	SourceImage       string
}

// validateSource reports the first incoherence, phrased so the message says
// what to do rather than what is wrong with the payload.
func validateSource(f sourceFields) error {
	if !oneOf(f.Type, validTypes) {
		return fmt.Errorf("type must be one of: %s", strings.Join(validTypes, ", "))
	}
	if !oneOf(f.BuildType, validBuildTypes) {
		return fmt.Errorf("build_type must be one of: %s", strings.Join(validBuildTypes, ", "))
	}
	if f.BuildTypeOverride != "" && !oneOf(f.BuildTypeOverride, validBuildTypes) {
		return fmt.Errorf("build_type_override must be one of: %s", strings.Join(validBuildTypes, ", "))
	}

	switch f.Type {
	case "git":
		if f.SourceRepo == "" {
			return errors.New("a git application needs a source_repo")
		}
		if f.SourceImage != "" {
			return errors.New("a git application builds its own image; remove source_image")
		}
		// build_type selects the builder, so "image" would leave nothing to
		// build the checkout with.
		if f.BuildType == "image" || f.BuildTypeOverride == "image" {
			return errors.New("build_type 'image' is only for image applications; choose dockerfile, buildpacks, or railpack")
		}
	case "image":
		if f.SourceImage == "" {
			return errors.New("an image application needs a source_image")
		}
		if f.SourceRepo != "" {
			return errors.New("an image application is not built from source; remove source_repo")
		}
		// Anything else would be accepted and then ignored, because the deploy
		// worker pulls without consulting a builder for image applications.
		if f.BuildType != "image" {
			return errors.New("an image application must use build_type 'image'")
		}
		if f.BuildTypeOverride != "" {
			return errors.New("build_type_override does not apply to image applications; remove it")
		}
		if f.DockerfilePath != "" {
			return errors.New("dockerfile_path does not apply to image applications; remove it")
		}
	}
	return nil
}

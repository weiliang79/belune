package handler

import "testing"

func TestValidateSource(t *testing.T) {
	git := sourceFields{Type: "git", BuildType: "railpack", SourceRepo: "https://github.com/a/b"}
	image := sourceFields{Type: "image", BuildType: "image", SourceImage: "nginx:1.25"}

	valid := map[string]sourceFields{
		"git app":                  git,
		"git app with dockerfile":  {Type: "git", BuildType: "dockerfile", SourceRepo: git.SourceRepo, DockerfilePath: "docker/Dockerfile"},
		"git app with an override": {Type: "git", BuildType: "railpack", SourceRepo: git.SourceRepo, BuildTypeOverride: "dockerfile"},
		"image app":                image,
		"image app with a digest":  {Type: "image", BuildType: "image", SourceImage: "nginx@sha256:abc"},
		"git app with buildpacks":  {Type: "git", BuildType: "buildpacks", SourceRepo: git.SourceRepo},
	}
	for name, f := range valid {
		t.Run(name, func(t *testing.T) {
			if err := validateSource(f); err != nil {
				t.Errorf("validateSource() = %v, want nil", err)
			}
		})
	}

	invalid := map[string]sourceFields{
		// The two failures observed in practice.
		"image app with no image":   {Type: "image", BuildType: "image"},
		"image app carrying a repo": {Type: "image", BuildType: "image", SourceImage: "nginx:1.25", SourceRepo: git.SourceRepo},
		"git app with no repo":      {Type: "git", BuildType: "railpack"},
		"git app carrying an image": {Type: "git", BuildType: "railpack", SourceRepo: git.SourceRepo, SourceImage: "nginx:1.25"},
		// build_type 'image' leaves nothing to build a checkout with.
		"git app built as image":      {Type: "git", BuildType: "image", SourceRepo: git.SourceRepo},
		"git app overridden to image": {Type: "git", BuildType: "railpack", SourceRepo: git.SourceRepo, BuildTypeOverride: "image"},
		// Accepted and then ignored, because image apps are pulled, not built.
		"image app with a builder":    {Type: "image", BuildType: "image", SourceImage: "nginx:1.25", BuildTypeOverride: "dockerfile"},
		"image app with a dockerfile": {Type: "image", BuildType: "image", SourceImage: "nginx:1.25", DockerfilePath: "Dockerfile"},
		"image app built from source": {Type: "image", BuildType: "railpack", SourceImage: "nginx:1.25"},
		// Enum membership, which previously reached the database and surfaced
		// as a 500 rather than a 400.
		"unknown type":       {Type: "compose", BuildType: "image", SourceImage: "nginx:1.25"},
		"unknown build type": {Type: "git", BuildType: "make", SourceRepo: git.SourceRepo},
		"unknown override":   {Type: "git", BuildType: "railpack", SourceRepo: git.SourceRepo, BuildTypeOverride: "make"},
	}
	for name, f := range invalid {
		t.Run(name, func(t *testing.T) {
			if err := validateSource(f); err == nil {
				t.Errorf("validateSource() = nil, want an error")
			}
		})
	}
}

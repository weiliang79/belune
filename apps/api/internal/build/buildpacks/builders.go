package buildpacks

// DefaultBuilderImage is the default CNB builder image.
const DefaultBuilderImage = "heroku/builder:24"

// KnownBuilder represents a known CNB builder image with metadata.
type KnownBuilder struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Image       string   `json:"image"`
	Description string   `json:"description"`
	Languages   []string `json:"languages"`
}

// KnownBuilders is the registry of well-known builder images.
var KnownBuilders = []KnownBuilder{
	{
		ID:          "heroku-24",
		Name:        "Heroku Builder 24",
		Image:       "heroku/builder:24",
		Description: "Heroku's official builder based on Ubuntu 24.04",
		Languages:   []string{"ruby", "python", "node", "go", "java", "php", "scala", "clojure"},
	},
	{
		ID:          "heroku-22",
		Name:        "Heroku Builder 22",
		Image:       "heroku/builder:22",
		Description: "Heroku's builder based on Ubuntu 22.04",
		Languages:   []string{"ruby", "python", "node", "go", "java", "php", "scala", "clojure"},
	},
	{
		ID:          "paketo-base",
		Name:        "Paketo Base",
		Image:       "paketobuildpacks/builder-jammy-base",
		Description: "Paketo Buildpacks base builder (Ubuntu Jammy)",
		Languages:   []string{"java", "node", "go", "python", "ruby", "dotnet", "php", "rust"},
	},
	{
		ID:          "paketo-full",
		Name:        "Paketo Full",
		Image:       "paketobuildpacks/builder-jammy-full",
		Description: "Paketo Buildpacks full builder with all dependencies",
		Languages:   []string{"java", "node", "go", "python", "ruby", "dotnet", "php", "rust"},
	},
	{
		ID:          "google",
		Name:        "Google Cloud Buildpacks",
		Image:       "gcr.io/buildpacks/builder:v1",
		Description: "Google Cloud's builder optimized for GCP workloads",
		Languages:   []string{"go", "java", "node", "python", "dotnet"},
	},
}

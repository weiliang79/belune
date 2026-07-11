package handler

import (
	"net/http"

	"github.com/weiliang79/belune/internal/build/railpack"
)

func (h *Handler) GetFeatures(w http.ResponseWriter, r *http.Request) {
	buildkitAvailable := railpack.CheckBuildKit() == nil

	writeJSON(w, http.StatusOK, map[string]any{
		"buildkit_available": buildkitAvailable,
		"instance_name":      h.instanceName(r.Context()),
	})
}

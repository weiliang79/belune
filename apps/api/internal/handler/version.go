package handler

import (
	"net/http"

	"github.com/weiliang79/belune/internal/version"
)

// GetVersion reports the running build's version.
//
// Unauthenticated by design. The value is already inferable from the served
// assets and response behaviour, and both the UI's identity block and the
// update checker need it before a session exists. A build that was not stamped
// at link time reports "dev" rather than guessing at a release.
func (h *Handler) GetVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"version": version.Version,
	})
}

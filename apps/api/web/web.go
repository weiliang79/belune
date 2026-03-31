// Package web embeds the compiled frontend assets for single-binary distribution.
// The dist/ directory is populated by `task build:web` before compilation.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var files embed.FS

// Handler returns an http.Handler that serves the embedded SPA assets.
// Any path that does not resolve to a known file is served index.html
// so that client-side routing works correctly.
// Returns nil when the build has not been run (dist/ contains only .gitkeep).
func Handler() http.Handler {
	fsys, err := fs.Sub(files, "dist")
	if err != nil {
		return nil
	}

	// Check if the frontend has been built (index.html must exist)
	if _, err := fsys.Open("index.html"); err != nil {
		return nil
	}

	fileServer := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Strip leading slash for fs.Open
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		f, err := fsys.Open(path)
		if err != nil {
			// Unknown path — serve index.html for SPA routing
			r.URL.Path = "/index.html"
		} else {
			f.Close()
		}
		fileServer.ServeHTTP(w, r)
	})
}

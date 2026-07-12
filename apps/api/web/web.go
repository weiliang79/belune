// Package web embeds the compiled frontend assets for single-binary distribution.
// The dist/ directory is populated by `task build:web` before compilation.
package web

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
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

	index, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		return nil
	}
	modTime := time.Now()

	// serveIndex writes index.html itself rather than rewriting the request path
	// and handing it to the file server.
	//
	// That rewrite is what the fallback used to do, and it is a trap:
	// http.FileServer redirects any path ending in "/index.html" to "./" — so a
	// rewritten request never served the page, it bounced. A refresh on /server
	// redirected to / (and the router then sent the user to /projects), while a
	// deep path bounced to its own parent forever:
	//
	//	/projects/x/applications/y  →  ./  →  /projects/x/applications/  →  ./  →  …
	//
	// which the browser reports as TOO_MANY_REDIRECTS. Dev never saw it, because
	// Vite does the SPA fallback itself.
	//
	// The shell must not be cached: it names the hashed asset bundles, and a stale
	// copy points at files a new deploy has already deleted.
	serveIndex := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeContent(w, r, "index.html", modTime, bytes.NewReader(index))
	}

	fileServer := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" || name == "." {
			serveIndex(w, r)
			return
		}

		f, err := fsys.Open(name)
		if err != nil {
			// Not a real file: a client-side route. Serve the shell.
			serveIndex(w, r)
			return
		}
		defer f.Close()

		// A directory would make the file server either list it or redirect to add
		// a trailing slash. Neither is ever what a SPA route wants.
		if st, err := f.Stat(); err != nil || st.IsDir() {
			serveIndex(w, r)
			return
		}

		fileServer.ServeHTTP(w, r)
	})
}

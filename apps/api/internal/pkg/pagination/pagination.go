package pagination

import (
	"net/http"
	"strconv"
)

type Params struct {
	Cursor string
	Limit  int
}

// Parse extracts pagination parameters from the request query string.
func Parse(r *http.Request) Params {
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	return Params{
		Cursor: r.URL.Query().Get("cursor"),
		Limit:  limit,
	}
}

package caddy

import (
	"net/http"
	"time"
)

// Client manages Caddy reverse proxy via the Admin API.
type Client struct {
	adminURL   string
	httpClient *http.Client
}

func New(adminURL string) *Client {
	return &Client{
		adminURL: adminURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

package caddy

// Client manages Caddy reverse proxy via the Admin API.
type Client struct {
	adminURL string
}

func New(adminURL string) *Client {
	return &Client{adminURL: adminURL}
}

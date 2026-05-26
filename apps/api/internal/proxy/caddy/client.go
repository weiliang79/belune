package caddy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// Client manages Caddy reverse proxy via the Admin API.
//
// Two transports are supported:
//
//   - TCP HTTP, e.g. CADDY_ADMIN_URL=http://caddy:2019. Default for the
//     dev compose stack.
//   - Unix socket, e.g. CADDY_ADMIN_URL=unix:///run/caddy/admin.sock. The
//     admin API is then unreachable from the network, eliminating the
//     class of attacks where an app container in the same network plane
//     could reconfigure routes or TLS. Recommended for production.
//
// In socket mode, requests are sent against http://unix internally — the
// hostname is ignored by the dialer, which always opens the configured
// socket path.
type Client struct {
	adminURL   string
	httpClient *http.Client
}

const unixScheme = "unix://"

// New builds a Caddy admin client. If adminURL begins with `unix://`, the
// remainder is treated as a filesystem path and all admin requests are
// proxied to that Unix domain socket.
func New(adminURL string) *Client {
	httpClient := &http.Client{Timeout: 10 * time.Second}

	if strings.HasPrefix(adminURL, unixScheme) {
		socketPath := strings.TrimPrefix(adminURL, unixScheme)
		httpClient.Transport = &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socketPath)
			},
		}
		// The hostname `unix` is a placeholder; only the path matters once
		// DialContext routes the connection to the socket. Using a stable
		// pseudo-host keeps the URL parseable elsewhere in the codebase.
		adminURL = "http://unix"
	}

	return &Client{
		adminURL:   adminURL,
		httpClient: httpClient,
	}
}

// Ping checks that the Caddy admin API is reachable and responding.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.adminURL+"/config/", nil)
	if err != nil {
		return fmt.Errorf("caddy ping: build request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("caddy ping: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("caddy ping: HTTP %d", resp.StatusCode)
	}
	return nil
}

package caddy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
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

	// skipMu guards autoHTTPSSkip: the set of hostnames Caddy must not attempt
	// to obtain a certificate for (ssl_mode=off). It is mirrored into srv0's
	// automatic_https.skip on every change. Holding it in memory rather than
	// read-modify-writing Caddy's copy keeps concurrent AddRoute callers from
	// clobbering each other; the reconciler re-asserts the set from DB truth on
	// every pass, which is also how it survives a Caddy restart.
	skipMu        sync.Mutex
	autoHTTPSSkip map[string]struct{}
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
		adminURL:      adminURL,
		httpClient:    httpClient,
		autoHTTPSSkip: make(map[string]struct{}),
	}
}

// Ping checks that the Caddy admin API is reachable and serving Belune's server.
// It reads srv0's listener list: a small response that proves both liveness and
// that our config is loaded, without pulling the whole config tree down as
// GET /config/ would. (The admin API has no root endpoint — GET / is a 404.)
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.adminURL+"/config/apps/http/servers/srv0/listen", nil)
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

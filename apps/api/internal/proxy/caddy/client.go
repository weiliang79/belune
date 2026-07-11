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

	// skipMu guards the two auto-HTTPS exclusion sets, mirrored into srv0 on every
	// change. Holding them in memory rather than read-modify-writing Caddy's copy
	// keeps concurrent AddRoute callers from clobbering each other; the reconciler
	// re-asserts both from DB truth on every pass, which is also how they survive
	// a Caddy restart.
	skipMu sync.Mutex
	// autoHTTPSSkip: ssl_mode=off — Caddy does no automatic HTTPS at all here.
	autoHTTPSSkip map[string]struct{}
	// autoHTTPSSkipCerts: ssl_mode=custom — the operator supplies the certificate,
	// so Caddy must not manage (and cache) one of its own for the name, but the
	// rest of automatic HTTPS still applies.
	autoHTTPSSkipCerts map[string]struct{}

	// dashboardUpstream is where Caddy dials to reach the API when serving the
	// dashboard. Resolved from *Caddy's* network, not ours: "localhost" here is
	// the Caddy container itself, which is why the previous default silently
	// refused every connection.
	dashboardUpstream string

	// tlsErrorSink records why TLS could not be set up for a hostname. Without it
	// a SetupTLS failure is invisible: the route goes live on :80 and the user is
	// left to guess why HTTPS never came up. The client has no database of its
	// own, so the owner of the domain rows injects this.
	tlsErrorSink TLSErrorSink
}

// TLSErrorSink records a TLS failure against the domain it concerns.
type TLSErrorSink func(ctx context.Context, hostname, reason string)

// SetTLSErrorSink installs the sink for SetupTLS failures.
func (c *Client) SetTLSErrorSink(sink TLSErrorSink) {
	c.tlsErrorSink = sink
}

// reportTLSError is nil-safe, so a client without a sink (tests, dev without a
// database) behaves exactly as before.
func (c *Client) reportTLSError(ctx context.Context, hostname, reason string) {
	if c.tlsErrorSink != nil {
		c.tlsErrorSink(ctx, hostname, reason)
	}
}

const unixScheme = "unix://"

// defaultDashboardUpstream matches the production compose service name.
const defaultDashboardUpstream = "belune:8080"

// SetDashboardUpstream overrides where Caddy dials to reach the API. Dev runs the
// API on the host rather than in a container, so it needs a different address.
func (c *Client) SetDashboardUpstream(addr string) {
	if addr != "" {
		c.dashboardUpstream = addr
	}
}

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
		adminURL:           adminURL,
		httpClient:         httpClient,
		autoHTTPSSkip:      make(map[string]struct{}),
		autoHTTPSSkipCerts: make(map[string]struct{}),
		dashboardUpstream:  defaultDashboardUpstream,
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

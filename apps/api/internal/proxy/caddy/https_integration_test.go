package caddy

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weiling79/belune/internal/proxy"
)

// These tests drive a real Caddy admin API — they assert the parts of the HTTPS
// foundation that only a live Caddy can prove: that PATCHing srv0 actually
// rebinds :443, that a TLS connection policy is what makes the handshake
// complete, and that automatic_https.skip really does suppress issuance.
//
// Run against the dev stack (`task dev:infra`), which sets local_certs so Caddy
// issues from its internal CA instead of failing ACME against *.belune.local:
//
//	BELUNE_CADDY_INTEGRATION=1 go test ./internal/proxy/caddy/ -run RealCaddy -v

const (
	autoHost = "phase1-auto.belune.local"
	offHost  = "phase1-off.belune.local"
)

func liveClient(t *testing.T) *Client {
	t.Helper()
	if os.Getenv("BELUNE_CADDY_INTEGRATION") == "" {
		t.Skip("set BELUNE_CADDY_INTEGRATION=1 to run against a real Caddy admin API")
	}
	adminURL := os.Getenv("CADDY_ADMIN_URL")
	if adminURL == "" {
		adminURL = "http://localhost:2019"
	}
	c := New(adminURL)
	require.NoError(t, c.Ping(context.Background()), "caddy admin API not reachable — is `task dev:infra` up?")
	return c
}

// routeCfg builds a minimal RouteConfig; the upstream never has to answer, since
// every assertion here is about TLS and Caddy's config tree, not proxying.
func routeCfg(hostname, sslMode string) proxy.RouteConfig {
	return proxy.RouteConfig{
		Hostname:  hostname,
		TargetURL: "http://127.0.0.1:9",
		TLS:       true,
		SSLMode:   sslMode,
	}
}

func serverConfig(t *testing.T, c *Client) map[string]any {
	t.Helper()
	server, err := c.fetchServer(context.Background())
	require.NoError(t, err)
	return server
}

// leafFor completes a TLS handshake against the local Caddy with the given SNI
// and returns the leaf certificate. Issuance is asynchronous, so poll until the
// timeout — which callers asserting the *absence* of a cert keep short.
func leafFor(t *testing.T, hostname string, timeout time.Duration) (*tls.ConnectionState, error) {
	t.Helper()
	addr := os.Getenv("CADDY_HTTPS_ADDR")
	if addr == "" {
		addr = "localhost:443"
	}

	var lastErr error
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := tls.DialWithDialer(
			&net.Dialer{Timeout: 3 * time.Second},
			"tcp", addr,
			&tls.Config{ServerName: hostname, InsecureSkipVerify: true}, //nolint:gosec // inspecting the leaf, not trusting it
		)
		if err == nil {
			state := conn.ConnectionState()
			conn.Close()
			if len(state.PeerCertificates) > 0 {
				return &state, nil
			}
			lastErr = fmt.Errorf("handshake completed with no peer certificate")
		} else {
			lastErr = err
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil, lastErr
}

// TestRealCaddy_HTTPSFoundation is the Phase 1 contract: after Belune writes its
// server config, srv0 terminates TLS on :443 and Caddy has issued a certificate
// for an ssl_mode=automatic hostname purely from the route's host matcher.
func TestRealCaddy_HTTPSFoundation(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()
	t.Cleanup(func() { _ = c.RemoveRoute(ctx, autoHost) })

	c.InitCatchAll(ctx)
	require.NoError(t, c.AddRoute(ctx, routeCfg(autoHost, "automatic")))

	server := serverConfig(t, c)
	assert.Equal(t, []any{":80", ":443"}, server["listen"], "srv0 must listen on both ports")
	assert.NotEmpty(t, server["tls_connection_policies"], "without a connection policy Caddy accepts TCP on :443 but never completes a handshake")

	autoHTTPS, _ := server["automatic_https"].(map[string]any)
	require.NotNil(t, autoHTTPS)
	assert.Equal(t, true, autoHTTPS["disable_redirects"], "Belune renders its own per-domain HTTP→HTTPS redirect")

	state, err := leafFor(t, autoHost, 15*time.Second)
	require.NoError(t, err, "TLS handshake against local Caddy failed")
	leaf := state.PeerCertificates[0]
	assert.Contains(t, leaf.DNSNames, autoHost, "Caddy should have issued for the hostname in the route matcher")
	t.Logf("issued by %q, valid until %s", leaf.Issuer.CommonName, leaf.NotAfter.Format(time.RFC3339))
}

// TestRealCaddy_SSLModeOffIsSkipped proves the skip list does its job: an
// ssl_mode=off hostname gets no certificate (so Caddy never starts a doomed ACME
// order for it), and the entry is pruned again when the route goes away.
func TestRealCaddy_SSLModeOffIsSkipped(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()
	t.Cleanup(func() { _ = c.RemoveRoute(ctx, offHost) })

	c.InitCatchAll(ctx)
	require.NoError(t, c.AddRoute(ctx, routeCfg(offHost, proxy.SSLModeOff)))

	autoHTTPS, _ := serverConfig(t, c)["automatic_https"].(map[string]any)
	require.NotNil(t, autoHTTPS)
	assert.Contains(t, autoHTTPS["skip"], offHost, "ssl_mode=off must be in automatic_https.skip")

	// No certificate exists for this SNI, so the handshake cannot produce a leaf.
	_, err := leafFor(t, offHost, 3*time.Second)
	assert.Error(t, err, "a skipped hostname must not have a certificate")

	require.NoError(t, c.RemoveRoute(ctx, offHost))
	autoHTTPS, _ = serverConfig(t, c)["automatic_https"].(map[string]any)
	require.NotNil(t, autoHTTPS)
	assert.NotContains(t, autoHTTPS["skip"], offHost, "skip entry must be pruned with the route")
}

// TestRealCaddy_SyncRestoresServerAfterRestart simulates what a Caddy restart
// does — the config reverts to the static Caddyfile, losing the :443 listener,
// the TLS policy and the skip list — and asserts the reconciler's sync call
// rebuilds all of it. Without this, HTTPS would silently stay down until the API
// itself was restarted.
func TestRealCaddy_SyncRestoresServerAfterRestart(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	c.InitCatchAll(ctx)

	// Roll srv0 back to its Caddyfile shape.
	require.NoError(t, patchJSON(ctx, c, "/config/apps/http/servers/srv0/listen", []string{":80"}))
	require.NoError(t, deleteJSON(ctx, c, "/config/apps/http/servers/srv0/tls_connection_policies"))
	before := serverConfig(t, c)
	require.Equal(t, []any{":80"}, before["listen"], "precondition: HTTPS listener is gone")

	require.NoError(t, c.SyncAutoHTTPSSkip(ctx, []string{offHost}))

	server := serverConfig(t, c)
	assert.Equal(t, []any{":80", ":443"}, server["listen"], "sync must restore the HTTPS listener")
	assert.NotEmpty(t, server["tls_connection_policies"], "sync must restore the TLS connection policy")
	autoHTTPS, _ := server["automatic_https"].(map[string]any)
	require.NotNil(t, autoHTTPS)
	assert.Equal(t, []any{offHost}, autoHTTPS["skip"], "sync must re-assert the skip list from DB truth")
}

// TestRealCaddy_ServerWriteKeepsAccessLogs guards the interaction that made
// ConfigureAccessLogs fragile: ensureServer PATCHes the whole srv0 object, so it
// must merge into the live config rather than replace it, or every route change
// would silently drop access logging.
func TestRealCaddy_ServerWriteKeepsAccessLogs(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	c.InitCatchAll(ctx)
	require.NoError(t, c.ConfigureAccessLogs(ctx))
	// Idempotent: re-running (as an API restart does) must not 409 out.
	require.NoError(t, c.ConfigureAccessLogs(ctx))

	require.NoError(t, c.SyncAutoHTTPSSkip(ctx, nil))

	logs, _ := serverConfig(t, c)["logs"].(map[string]any)
	require.NotNil(t, logs, "server write dropped the access-log config")
	assert.Equal(t, "http.access.srv0", logs["default_logger_name"])
}

func patchJSON(ctx context.Context, c *Client, path string, payload any) error {
	_, err := c.doConfig(ctx, http.MethodPatch, path, payload)
	return err
}

func deleteJSON(ctx context.Context, c *Client, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.adminURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var body json.RawMessage
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return nil
}

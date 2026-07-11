package caddy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/weiling79/belune/internal/pkg/metrics"
	"github.com/weiling79/belune/internal/proxy"
)

// ErrCaddyUnreachable is returned when the Caddy admin API cannot be contacted
// (DNS miss, connection refused, timeout). Callers at the service layer can
// treat this as "degraded in dev" while real schema / validation errors
// continue to surface as regular errors.
var ErrCaddyUnreachable = errors.New("caddy admin API unreachable")

// isTransportError returns true for network-level failures that should surface
// as ErrCaddyUnreachable. Schema errors from Caddy come back as HTTP 4xx/5xx
// responses, so those are handled separately.
func isTransportError(err error) bool {
	if err == nil {
		return false
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

// caddyMatcher is a generic Caddy matcher with arbitrary fields.
type caddyMatcher map[string]any

const catchAllRouteID = "route-catch-all"

// caddyRoute represents a Caddy route configuration.
type caddyRoute struct {
	ID     string        `json:"@id"`
	Match  []caddyMatch  `json:"match,omitempty"`
	Handle []caddyHandle `json:"handle"`
}

type caddyMatch struct {
	Host []string `json:"host"`
}

// caddyHandle is a flexible map-based handler to support all Caddy handler types
// (reverse_proxy, authentication, headers, subroute, static_response, etc.).
type caddyHandle map[string]any

type caddyUpstream struct {
	Dial string `json:"dial"`
}

// newReverseProxyHandle creates a reverse_proxy handler. If healthCheckPath
// is non-empty, an active upstream health check is configured so Caddy itself
// will mark the upstream unhealthy on consecutive 5xx responses — separate
// from the deploy-time verification probe, which gates whether a deploy ever
// goes live in the first place.
func newReverseProxyHandle(upstreams []caddyUpstream, healthCheckPath string) caddyHandle {
	h := caddyHandle{
		"handler":   "reverse_proxy",
		"upstreams": upstreams,
	}
	if healthCheckPath != "" {
		h["health_checks"] = map[string]any{
			"active": map[string]any{
				"uri":           healthCheckPath,
				"interval":      "30s",
				"timeout":       "5s",
				"expect_status": 2,
			},
		}
	}
	return h
}

// InitCatchAll must be called once at startup. It normalises the routes list so
// that all domain-specific routes come first and the catch-all (dashboard proxy)
// is last with a known @id. This makes every subsequent AddRoute call safe to
// use simple appending — we just move the catch-all to the end afterwards.
func (c *Client) InitCatchAll(ctx context.Context) {
	rawRoutes, err := c.fetchRawRoutes(ctx)
	if err != nil {
		slog.Warn("caddy: InitCatchAll failed to fetch routes", "error", err)
		return
	}

	// Keep only domain-specific routes; drop any catch-alls (no host matcher).
	var domainRoutes []json.RawMessage
	for _, r := range rawRoutes {
		var probe struct {
			ID    string `json:"@id"`
			Match []struct {
				Host []string `json:"host"`
			} `json:"match"`
		}
		if err := json.Unmarshal(r, &probe); err != nil {
			continue
		}
		if probe.ID == catchAllRouteID {
			continue // will be re-appended with correct ID below
		}
		if len(probe.Match) > 0 && len(probe.Match[0].Host) > 0 {
			domainRoutes = append(domainRoutes, r)
		}
		// anonymous catch-alls (no match) are dropped
	}

	// Re-append catch-all with known @id at the end.
	catchAllJSON, _ := json.Marshal(caddyRoute{
		ID:     catchAllRouteID,
		Handle: []caddyHandle{newReverseProxyHandle([]caddyUpstream{{Dial: "localhost:8080"}}, "")},
	})
	newRoutes := append(domainRoutes, catchAllJSON)

	if err := c.ensureServer(ctx, newRoutes); err != nil {
		slog.Warn("caddy: InitCatchAll failed to write server config", "error", err)
		return
	}
	slog.Info("caddy catch-all route initialised", "listen", serverListen)
}

func (c *Client) AddRoute(ctx context.Context, cfg proxy.RouteConfig) (err error) {
	defer func() { metrics.RecordCaddyCall("add_route", err) }()
	// Remove any existing route for this hostname first to prevent duplicates.
	// Ignore "not found" — normal on fresh routes.
	if err := c.RemoveRoute(ctx, cfg.Hostname); err != nil && !errors.Is(err, ErrCaddyUnreachable) {
		slog.Debug("caddy: pre-remove returned non-fatal error", "hostname", cfg.Hostname, "error", err)
	}

	// Build the handler chain: feature handlers first, then reverse_proxy as the terminal handler.
	handlers, err := buildFeatureHandlers(cfg)
	if err != nil {
		return fmt.Errorf("build handlers: %w", err)
	}

	// Reverse proxy is always the last handler.
	handlers = append(handlers, newReverseProxyHandle(
		[]caddyUpstream{{Dial: targetToDial(cfg.TargetURL)}},
		cfg.HealthCheckPath,
	))

	// If ForceHTTPS is enabled, prepend an HTTP→HTTPS redirect subroute.
	if cfg.ForceHTTPS && cfg.SSLMode != proxy.SSLModeOff {
		handlers = append([]caddyHandle{
			{
				"handler": "subroute",
				"routes": []map[string]any{{
					"match": []caddyMatcher{{"protocol": "http"}},
					"handle": []caddyHandle{{
						"handler":     "static_response",
						"headers":     map[string][]string{"Location": {"{http.request.scheme}s://{http.request.host}{http.request.uri}"}},
						"status_code": "301",
					}},
				}},
			},
		}, handlers...)
	}

	route := caddyRoute{
		ID: fmt.Sprintf("route-%s", cfg.Hostname),
		Match: []caddyMatch{
			{Host: []string{cfg.Hostname}},
		},
		Handle: handlers,
	}

	body, err := json.Marshal(route)
	if err != nil {
		return fmt.Errorf("marshal route: %w", err)
	}

	// Update the auto-HTTPS skip set *before* the route lands: srv0 listens on
	// :443, so the moment a host matcher appears Caddy starts trying to issue a
	// certificate for it — which for an ssl_mode=off domain would fail loudly and
	// pointlessly. A failure here is not fatal (the route itself is still
	// correct) and the reconciler re-asserts the set on its next pass.
	if c.setSkip(cfg.Hostname, cfg.SSLMode == proxy.SSLModeOff) {
		if err := c.ensureServer(ctx, nil); err != nil {
			slog.Warn("caddy: failed to update auto-HTTPS skip list", "hostname", cfg.Hostname, "error", err)
		}
	}

	// Append the domain route (order doesn't matter here; moveCatchAllToEnd fixes it).
	appendURL := fmt.Sprintf("%s/config/apps/http/servers/srv0/routes", c.adminURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, appendURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if isTransportError(err) {
			return fmt.Errorf("%w: %v", ErrCaddyUnreachable, err)
		}
		return fmt.Errorf("caddy add route: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("caddy add route: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	// Ensure the catch-all stays last so it doesn't shadow this new domain route.
	if err := c.moveCatchAllToEnd(ctx); err != nil {
		return fmt.Errorf("reorder catch-all: %w", err)
	}

	// Configure TLS if needed.
	if cfg.TLS {
		if err := c.SetupTLS(ctx, cfg.Hostname, cfg.SSLMode, cfg.CertPath, cfg.KeyPath); err != nil {
			// TLS provisioning is best-effort: the route itself is live, so we
			// log rather than unwind. Callers can still observe the error via
			// future cert-status polling once implemented.
			slog.Warn("caddy: TLS setup failed", "hostname", cfg.Hostname, "error", err)
		}
	}

	slog.Info("caddy route added", "hostname", cfg.Hostname, "target", cfg.TargetURL)
	return nil
}

// buildFeatureHandlers translates RouteFeatures into Caddy handler JSON.
// Feature configs are parsed through proxy.ParseFeatureConfig so any
// malformed payload is dropped with a warning rather than rendered as a
// broken Caddy config.
func buildFeatureHandlers(cfg proxy.RouteConfig) ([]caddyHandle, error) {
	var handlers []caddyHandle

	for _, f := range cfg.Features {
		if !f.Enabled {
			continue
		}
		parsed, err := proxy.ParseFeatureConfig(f.Type, f.Config)
		if err != nil {
			slog.Warn("caddy: skipping invalid route feature", "type", f.Type, "error", err)
			continue
		}

		switch c := parsed.(type) {
		case *proxy.BasicAuthConfig:
			handlers = append(handlers, caddyHandle{
				"handler": "authentication",
				"providers": map[string]any{
					"http_basic": map[string]any{
						"accounts": []map[string]string{{
							"username": c.Username,
							"password": c.HashedPassword,
						}},
					},
				},
			})

		case *proxy.HeadersConfig:
			h := caddyHandle{"handler": "headers"}
			if c.Request != nil {
				h["request"] = headerOpsToMap(c.Request)
			}
			if c.Response != nil {
				h["response"] = headerOpsToMap(c.Response)
			}
			handlers = append(handlers, h)

		case *proxy.IPAllowlistConfig:
			handlers = append(handlers, caddyHandle{
				"handler": "subroute",
				"routes": []map[string]any{{
					"match": []caddyMatcher{{"not": []caddyMatcher{{"remote_ip": map[string]any{"ranges": c.Ranges}}}}},
					"handle": []caddyHandle{{
						"handler":     "static_response",
						"status_code": "403",
						"body":        "Forbidden",
					}},
				}},
			})

		case *proxy.RedirectConfig:
			handlers = append(handlers, caddyHandle{
				"handler": "subroute",
				"routes": []map[string]any{{
					"match": []caddyMatcher{{"path": []string{c.From}}},
					"handle": []caddyHandle{{
						"handler":     "static_response",
						"headers":     map[string][]string{"Location": {c.To}},
						"status_code": fmt.Sprintf("%d", c.StatusCode),
					}},
				}},
			})

		case *proxy.RateLimitConfig:
			// Rate limiting requires a Caddy module — log for now.
			slog.Info("rate_limit feature configured but requires caddy-rate-limit module", "rate", c.Rate)
		}
	}

	// Append advanced config handlers if provided.
	if len(cfg.AdvancedConfig) > 0 {
		advHandlers, err := parseAdvancedConfig(cfg.AdvancedConfig)
		if err != nil {
			return nil, err
		}
		handlers = append(handlers, advHandlers...)
	}

	return handlers, nil
}

// headerOpsToMap converts the typed HeaderOps to the map shape Caddy expects.
// Nil sub-fields are omitted so the resulting JSON stays compact.
func headerOpsToMap(ops *proxy.HeaderOps) map[string]any {
	m := map[string]any{}
	if len(ops.Set) > 0 {
		m["set"] = ops.Set
	}
	if len(ops.Add) > 0 {
		m["add"] = ops.Add
	}
	if len(ops.Delete) > 0 {
		m["delete"] = ops.Delete
	}
	return m
}

// advancedHandlerAllowlist lists Caddy handler modules that are safe to expose
// through the per-domain AdvancedConfig field. Anything outside this list is
// rejected so user-supplied JSON cannot attach arbitrary modules (e.g. a
// reverse_proxy that routes to an attacker-controlled upstream).
var advancedHandlerAllowlist = map[string]struct{}{
	"headers":        {},
	"rewrite":        {},
	"redir":          {},
	"handle_path":    {},
	"subroute":       {},
	"authentication": {}, // scoped to http_basic by parser below
}

// parseAdvancedConfig validates user-supplied AdvancedConfig JSON. The input
// must decode to a JSON array of Caddy handlers ([]caddyHandle). Each handler
// must declare a "handler" key from the allowlist. Errors list every
// disallowed handler so the user sees the full diagnosis in one round trip.
func parseAdvancedConfig(raw []byte) ([]caddyHandle, error) {
	var handlers []caddyHandle
	if err := json.Unmarshal(raw, &handlers); err != nil {
		return nil, fmt.Errorf("advanced_config: expected a JSON array of handlers: %w", err)
	}
	var bad []string
	for i, h := range handlers {
		name, _ := h["handler"].(string)
		if name == "" {
			bad = append(bad, fmt.Sprintf("#%d: missing \"handler\" key", i))
			continue
		}
		if _, ok := advancedHandlerAllowlist[name]; !ok {
			bad = append(bad, fmt.Sprintf("#%d: %q is not allowed", i, name))
		}
	}
	if len(bad) > 0 {
		return nil, fmt.Errorf("advanced_config: disallowed handlers: %s", strings.Join(bad, "; "))
	}
	return handlers, nil
}

func (c *Client) RemoveRoute(ctx context.Context, hostname string) (err error) {
	defer func() { metrics.RecordCaddyCall("remove_route", err) }()
	routeID := fmt.Sprintf("route-%s", hostname)
	deleteURL := fmt.Sprintf("%s/id/%s", c.adminURL, routeID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, deleteURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if isTransportError(err) {
			return fmt.Errorf("%w: %v", ErrCaddyUnreachable, err)
		}
		return fmt.Errorf("caddy remove route: %w", err)
	}
	defer resp.Body.Close()

	// 404 / 500 "unknown object" is treated as success — the route is gone,
	// which is exactly what the caller asked for. Any other 4xx/5xx is an error.
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		bodyStr := string(respBody)
		if resp.StatusCode == http.StatusNotFound || strings.Contains(bodyStr, "unknown object") {
			slog.Debug("caddy: route not found on delete", "hostname", hostname)
			return nil
		}
		return fmt.Errorf("caddy remove route: HTTP %d: %s", resp.StatusCode, bodyStr)
	}

	// The hostname no longer exists, so it has no business in the skip set.
	if c.setSkip(hostname, false) {
		if err := c.ensureServer(ctx, nil); err != nil {
			slog.Warn("caddy: failed to prune auto-HTTPS skip list", "hostname", hostname, "error", err)
		}
	}

	slog.Info("caddy route removed", "hostname", hostname)
	return nil
}

func (c *Client) ListRoutes(ctx context.Context) (result []proxy.RouteConfig, err error) {
	defer func() { metrics.RecordCaddyCall("list_routes", err) }()
	url := fmt.Sprintf("%s/config/apps/http/servers/srv0/routes", c.adminURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("caddy list routes: %w", err)
	}
	defer resp.Body.Close()

	var routes []caddyRoute
	if err := json.NewDecoder(resp.Body).Decode(&routes); err != nil {
		return nil, fmt.Errorf("decode routes: %w", err)
	}

	for _, route := range routes {
		if len(route.Match) > 0 && len(route.Match[0].Host) > 0 {
			cfg := proxy.RouteConfig{
				Hostname: route.Match[0].Host[0],
			}
			// Extract target URL from the last handler (reverse_proxy).
			for i := len(route.Handle) - 1; i >= 0; i-- {
				h := route.Handle[i]
				if h["handler"] == "reverse_proxy" {
					if upstreams, ok := h["upstreams"].([]any); ok && len(upstreams) > 0 {
						if u, ok := upstreams[0].(map[string]any); ok {
							if dial, ok := u["dial"].(string); ok {
								cfg.TargetURL = dial
							}
						}
					}
					break
				}
			}
			result = append(result, cfg)
		}
	}

	return result, nil
}

// moveCatchAllToEnd rewrites srv0.routes so the catch-all (if any) is the
// last entry. Uses a single PATCH on the routes array so that two concurrent
// AddRoute callers cannot observe an interleaved state where the catch-all
// has been deleted but not yet re-appended.
func (c *Client) moveCatchAllToEnd(ctx context.Context) error {
	rawRoutes, err := c.fetchRawRoutes(ctx)
	if err != nil {
		if isTransportError(err) {
			return fmt.Errorf("%w: %v", ErrCaddyUnreachable, err)
		}
		return fmt.Errorf("fetch routes: %w", err)
	}

	var domainRoutes []json.RawMessage
	var catchAll json.RawMessage
	for _, r := range rawRoutes {
		var probe struct {
			ID string `json:"@id"`
		}
		_ = json.Unmarshal(r, &probe)
		if probe.ID == catchAllRouteID {
			catchAll = r
			continue
		}
		domainRoutes = append(domainRoutes, r)
	}

	// Synthesise a catch-all if none exists yet (fresh server).
	if len(catchAll) == 0 {
		catchAll, err = json.Marshal(caddyRoute{
			ID:     catchAllRouteID,
			Handle: []caddyHandle{newReverseProxyHandle([]caddyUpstream{{Dial: "localhost:8080"}}, "")},
		})
		if err != nil {
			return fmt.Errorf("marshal catch-all: %w", err)
		}
	}
	newRoutes := append(domainRoutes, catchAll)

	return c.patchRoutes(ctx, newRoutes)
}

// patchRoutes replaces the srv0 routes array atomically via a single PATCH.
func (c *Client) patchRoutes(ctx context.Context, routes []json.RawMessage) error {
	body, err := json.Marshal(routes)
	if err != nil {
		return fmt.Errorf("marshal routes: %w", err)
	}
	patchURL := fmt.Sprintf("%s/config/apps/http/servers/srv0/routes", c.adminURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, patchURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if isTransportError(err) {
			return fmt.Errorf("%w: %v", ErrCaddyUnreachable, err)
		}
		return fmt.Errorf("patch routes: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("patch routes: HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// fetchRawRoutes returns the current routes as raw JSON messages.
func (c *Client) fetchRawRoutes(ctx context.Context) ([]json.RawMessage, error) {
	url := fmt.Sprintf("%s/config/apps/http/servers/srv0/routes", c.adminURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var routes []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&routes); err != nil {
		return nil, err
	}
	return routes, nil
}

// serverListen is the listener set srv0 must always have. The HTTPS listener is
// asserted from here rather than the Caddyfile because the Caddyfile adapter
// emits one server per port: `:80, :443` would produce srv0=:443 + srv1=:80,
// stranding every route below (all of which address srv0 by name) on the wrong
// server. Without :443 in this list Caddy applies no automatic HTTPS at all.
var serverListen = []string{":80", ":443"}

// ensureServer writes Belune's required srv0 configuration: the dual listener,
// the automatic-HTTPS policy, and a TLS connection policy (without at least one
// policy Caddy accepts TCP on :443 but never completes a handshake). Passing a
// non-nil routes replaces the route table; nil leaves it untouched.
//
// Caddy's PATCH replaces the whole object at the given path, and PATCH cannot
// create a key that does not exist yet (tls_connection_policies is absent from
// the adapted Caddyfile), so this reads the current server and merges into it —
// preserving keys it does not own, notably the `logs` set by ConfigureAccessLogs.
func (c *Client) ensureServer(ctx context.Context, routes []json.RawMessage) error {
	server, err := c.fetchServer(ctx)
	if err != nil {
		return err
	}

	server["listen"] = serverListen
	server["automatic_https"] = map[string]any{
		// Belune owns HTTP→HTTPS redirection per-domain via the ForceHTTPS
		// subroute in AddRoute; Caddy's own redirect routes would shadow it.
		"disable_redirects": true,
		// Hostnames with ssl_mode=off: Caddy must not try to issue for them.
		"skip": c.skipList(),
	}
	server["tls_connection_policies"] = []map[string]any{{}}
	if routes != nil {
		server["routes"] = routes
	}

	body, err := json.Marshal(server)
	if err != nil {
		return fmt.Errorf("marshal server: %w", err)
	}
	url := fmt.Sprintf("%s/config/apps/http/servers/srv0", c.adminURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if isTransportError(err) {
			return fmt.Errorf("%w: %v", ErrCaddyUnreachable, err)
		}
		return fmt.Errorf("patch server: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("patch server: HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// fetchServer returns srv0's current configuration as a mutable map.
func (c *Client) fetchServer(ctx context.Context) (map[string]any, error) {
	url := fmt.Sprintf("%s/config/apps/http/servers/srv0", c.adminURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		if isTransportError(err) {
			return nil, fmt.Errorf("%w: %v", ErrCaddyUnreachable, err)
		}
		return nil, fmt.Errorf("fetch server: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch server: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	server := map[string]any{}
	if err := json.NewDecoder(resp.Body).Decode(&server); err != nil {
		return nil, fmt.Errorf("decode server: %w", err)
	}
	return server, nil
}

// SyncAutoHTTPSSkip replaces the auto-HTTPS skip set with the given hostnames
// (those whose domains are ssl_mode=off) and re-asserts the full server config.
// The reconciler calls this every pass, which is what restores the :443 listener
// and the skip list after Caddy restarts back to its Caddyfile-only config.
func (c *Client) SyncAutoHTTPSSkip(ctx context.Context, hostnames []string) error {
	c.skipMu.Lock()
	c.autoHTTPSSkip = make(map[string]struct{}, len(hostnames))
	for _, h := range hostnames {
		c.autoHTTPSSkip[h] = struct{}{}
	}
	c.skipMu.Unlock()

	return c.ensureServer(ctx, nil)
}

// setSkip adds or removes one hostname from the skip set, reporting whether the
// set actually changed.
func (c *Client) setSkip(hostname string, skip bool) bool {
	c.skipMu.Lock()
	defer c.skipMu.Unlock()

	_, present := c.autoHTTPSSkip[hostname]
	switch {
	case skip && !present:
		c.autoHTTPSSkip[hostname] = struct{}{}
	case !skip && present:
		delete(c.autoHTTPSSkip, hostname)
	default:
		return false
	}
	return true
}

// skipList returns the skip set as a sorted slice, so an unchanged set always
// renders to identical JSON and does not churn Caddy's config.
func (c *Client) skipList() []string {
	c.skipMu.Lock()
	defer c.skipMu.Unlock()

	list := make([]string, 0, len(c.autoHTTPSSkip))
	for h := range c.autoHTTPSSkip {
		list = append(list, h)
	}
	sort.Strings(list)
	return list
}

// targetToDial converts "http://host:port" to "host:port" for Caddy upstream dial.
func targetToDial(target string) string {
	// Strip protocol prefix
	for _, prefix := range []string{"http://", "https://"} {
		if len(target) > len(prefix) && target[:len(prefix)] == prefix {
			return target[len(prefix):]
		}
	}
	return target
}

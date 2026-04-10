package caddy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/ungweiliang/selfhost-paas/internal/proxy"
)

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

// newReverseProxyHandle creates a reverse_proxy handler.
func newReverseProxyHandle(upstreams []caddyUpstream) caddyHandle {
	return caddyHandle{
		"handler":   "reverse_proxy",
		"upstreams": upstreams,
	}
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
		Handle: []caddyHandle{newReverseProxyHandle([]caddyUpstream{{Dial: "localhost:8080"}})},
	})
	newRoutes := append(domainRoutes, catchAllJSON)

	c.replaceAllRoutes(ctx, newRoutes)
	slog.Info("caddy catch-all route initialised")
}

func (c *Client) AddRoute(ctx context.Context, cfg proxy.RouteConfig) error {
	// Remove any existing route for this hostname first to prevent duplicates.
	c.RemoveRoute(ctx, cfg.Hostname)

	// Build the handler chain: feature handlers first, then reverse_proxy as the terminal handler.
	handlers := buildFeatureHandlers(cfg)

	// Reverse proxy is always the last handler.
	handlers = append(handlers, newReverseProxyHandle([]caddyUpstream{{Dial: targetToDial(cfg.TargetURL)}}))

	// If ForceHTTPS is enabled, prepend an HTTP→HTTPS redirect subroute.
	if cfg.ForceHTTPS && cfg.SSLMode != "off" {
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

	// Append the domain route (order doesn't matter here; moveCatchAllToEnd fixes it).
	url := fmt.Sprintf("%s/config/apps/http/servers/srv0/routes", c.adminURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Warn("caddy add route failed (caddy may not be running)", "error", err, "hostname", cfg.Hostname)
		return nil // Non-fatal: caddy might not be running in dev
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		slog.Warn("caddy add route returned error", "status", resp.StatusCode, "body", string(respBody))
	}

	// Ensure the catch-all stays last so it doesn't shadow this new domain route.
	c.moveCatchAllToEnd(ctx)

	// Configure TLS if needed.
	if cfg.TLS {
		if err := c.SetupTLS(ctx, cfg.Hostname, cfg.SSLMode, cfg.CertPath, cfg.KeyPath); err != nil {
			slog.Warn("caddy: TLS setup failed", "hostname", cfg.Hostname, "error", err)
		}
	}

	slog.Info("caddy route added", "hostname", cfg.Hostname, "target", cfg.TargetURL)
	return nil
}

// buildFeatureHandlers translates RouteFeatures into Caddy handler JSON.
func buildFeatureHandlers(cfg proxy.RouteConfig) []caddyHandle {
	var handlers []caddyHandle

	for _, f := range cfg.Features {
		if !f.Enabled {
			continue
		}
		switch f.Type {
		case "basic_auth":
			// Config expected: {"username": "...", "hashed_password": "..."}
			username, _ := f.Config["username"].(string)
			hashedPw, _ := f.Config["hashed_password"].(string)
			if username != "" && hashedPw != "" {
				handlers = append(handlers, caddyHandle{
					"handler": "authentication",
					"providers": map[string]any{
						"http_basic": map[string]any{
							"accounts": []map[string]string{{
								"username": username,
								"password": hashedPw,
							}},
						},
					},
				})
			}

		case "headers":
			// Config expected: {"response": {"set": {"X-Custom": ["value"]}}}
			if resp, ok := f.Config["response"]; ok {
				handlers = append(handlers, caddyHandle{
					"handler":  "headers",
					"response": resp,
				})
			}

		case "ip_allowlist":
			// Config expected: {"ranges": ["192.168.1.0/24", "10.0.0.0/8"]}
			// IP allowlist is applied as a matcher on a subroute that returns 403.
			if ranges, ok := f.Config["ranges"].([]any); ok && len(ranges) > 0 {
				var rangeStrs []string
				for _, r := range ranges {
					if s, ok := r.(string); ok {
						rangeStrs = append(rangeStrs, s)
					}
				}
				handlers = append(handlers, caddyHandle{
					"handler": "subroute",
					"routes": []map[string]any{{
						"match": []caddyMatcher{{"not": []caddyMatcher{{"remote_ip": map[string]any{"ranges": rangeStrs}}}}},
						"handle": []caddyHandle{{
							"handler":     "static_response",
							"status_code": "403",
							"body":        "Forbidden",
						}},
					}},
				})
			}

		case "redirect":
			// Config expected: {"from": "/old", "to": "/new", "status_code": 301}
			from, _ := f.Config["from"].(string)
			to, _ := f.Config["to"].(string)
			statusCode := "301"
			if sc, ok := f.Config["status_code"]; ok {
				statusCode = fmt.Sprintf("%v", sc)
			}
			if from != "" && to != "" {
				handlers = append(handlers, caddyHandle{
					"handler": "subroute",
					"routes": []map[string]any{{
						"match": []caddyMatcher{{"path": []string{from}}},
						"handle": []caddyHandle{{
							"handler":     "static_response",
							"headers":     map[string][]string{"Location": {to}},
							"status_code": statusCode,
						}},
					}},
				})
			}

		case "rate_limit":
			// Rate limiting requires a Caddy module — log for now.
			slog.Info("rate_limit feature configured but requires caddy-rate-limit module", "config", f.Config)
		}
	}

	// Append advanced config handlers if provided.
	if len(cfg.AdvancedConfig) > 0 {
		var advHandlers []caddyHandle
		if err := json.Unmarshal(cfg.AdvancedConfig, &advHandlers); err != nil {
			slog.Warn("failed to parse advanced_config as handlers", "error", err)
		} else {
			handlers = append(handlers, advHandlers...)
		}
	}

	return handlers
}

func (c *Client) RemoveRoute(ctx context.Context, hostname string) error {
	routeID := fmt.Sprintf("route-%s", hostname)
	url := fmt.Sprintf("%s/id/%s", c.adminURL, routeID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Warn("caddy remove route failed", "error", err)
		return nil
	}
	defer resp.Body.Close()

	slog.Info("caddy route removed", "hostname", hostname)
	return nil
}

func (c *Client) ListRoutes(ctx context.Context) ([]proxy.RouteConfig, error) {
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

	var result []proxy.RouteConfig
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

// moveCatchAllToEnd deletes the catch-all route by its known @id and re-appends
// it so it always evaluates after all domain-specific routes.
func (c *Client) moveCatchAllToEnd(ctx context.Context) {
	deleteURL := fmt.Sprintf("%s/id/%s", c.adminURL, catchAllRouteID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, deleteURL, nil)
	if err == nil {
		resp, err := c.httpClient.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}

	route := caddyRoute{
		ID:     catchAllRouteID,
		Handle: []caddyHandle{newReverseProxyHandle([]caddyUpstream{{Dial: "localhost:8080"}})},
	}
	body, _ := json.Marshal(route)
	appendURL := fmt.Sprintf("%s/config/apps/http/servers/srv0/routes", c.adminURL)
	req, err = http.NewRequestWithContext(ctx, http.MethodPost, appendURL, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Warn("caddy: failed to re-append catch-all", "error", err)
		return
	}
	resp.Body.Close()
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

// replaceAllRoutes replaces the entire routes array in srv0 by patching the
// server object. A direct PUT on the routes path can fail with "key already
// exists", so we PATCH the parent server instead.
func (c *Client) replaceAllRoutes(ctx context.Context, routes []json.RawMessage) {
	wrapper := map[string]any{
		"listen": []string{":80"},
		"routes": routes,
	}
	body, err := json.Marshal(wrapper)
	if err != nil {
		slog.Warn("caddy: failed to marshal server for route replacement", "error", err)
		return
	}
	url := fmt.Sprintf("%s/config/apps/http/servers/srv0", c.adminURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Warn("caddy: replaceAllRoutes failed", "error", err)
		return
	}
	resp.Body.Close()
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

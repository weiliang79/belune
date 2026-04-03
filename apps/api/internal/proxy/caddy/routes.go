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

type caddyHandle struct {
	Handler   string          `json:"handler"`
	Upstreams []caddyUpstream `json:"upstreams,omitempty"`
}

type caddyUpstream struct {
	Dial string `json:"dial"`
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
		ID:    catchAllRouteID,
		Handle: []caddyHandle{{Handler: "reverse_proxy", Upstreams: []caddyUpstream{{Dial: "localhost:8080"}}}},
	})
	newRoutes := append(domainRoutes, catchAllJSON)

	c.patchAllRoutes(ctx, newRoutes)
	slog.Info("caddy catch-all route initialised")
}

func (c *Client) AddRoute(ctx context.Context, cfg proxy.RouteConfig) error {
	route := caddyRoute{
		ID: fmt.Sprintf("route-%s", cfg.Hostname),
		Match: []caddyMatch{
			{Host: []string{cfg.Hostname}},
		},
		Handle: []caddyHandle{
			{
				Handler:   "reverse_proxy",
				Upstreams: []caddyUpstream{{Dial: targetToDial(cfg.TargetURL)}},
			},
		},
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

	slog.Info("caddy route added", "hostname", cfg.Hostname, "target", cfg.TargetURL)
	return nil
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
			if len(route.Handle) > 0 && len(route.Handle[0].Upstreams) > 0 {
				cfg.TargetURL = route.Handle[0].Upstreams[0].Dial
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
		ID:    catchAllRouteID,
		Handle: []caddyHandle{{Handler: "reverse_proxy", Upstreams: []caddyUpstream{{Dial: "localhost:8080"}}}},
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

// patchAllRoutes replaces the entire routes array in srv0.
func (c *Client) patchAllRoutes(ctx context.Context, routes []json.RawMessage) {
	body, err := json.Marshal(routes)
	if err != nil {
		slog.Warn("caddy: failed to marshal routes for patch", "error", err)
		return
	}
	url := fmt.Sprintf("%s/config/apps/http/servers/srv0/routes", c.adminURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Warn("caddy: patchAllRoutes failed", "error", err)
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

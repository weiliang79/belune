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

// caddyRoute represents a Caddy route configuration.
type caddyRoute struct {
	ID     string        `json:"@id"`
	Match  []caddyMatch  `json:"match"`
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

	// Insert at index 0 so domain-specific routes take priority over the catch-all :80 route.
	url := fmt.Sprintf("%s/config/apps/http/servers/srv0/routes/0", c.adminURL)
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

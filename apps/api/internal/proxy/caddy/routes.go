package caddy

import (
	"context"

	"github.com/ungweiliang/selfhost-paas/internal/proxy"
)

func (c *Client) AddRoute(ctx context.Context, cfg proxy.RouteConfig) error {
	// TODO: POST to Caddy Admin API to add route
	return nil
}

func (c *Client) RemoveRoute(ctx context.Context, hostname string) error {
	// TODO: DELETE from Caddy Admin API
	return nil
}

func (c *Client) ListRoutes(ctx context.Context) ([]proxy.RouteConfig, error) {
	// TODO: GET from Caddy Admin API
	return nil, nil
}

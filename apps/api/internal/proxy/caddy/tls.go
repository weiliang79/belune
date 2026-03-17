package caddy

import (
	"context"
	"log/slog"
)

func (c *Client) SetupTLS(ctx context.Context, hostname string) error {
	// Caddy automatically provisions TLS certificates via ACME when
	// a route with a hostname is added. No additional configuration
	// is needed for standard HTTPS.
	slog.Info("TLS will be auto-provisioned by Caddy", "hostname", hostname)
	return nil
}

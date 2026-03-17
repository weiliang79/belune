package caddy

import "context"

func (c *Client) SetupTLS(ctx context.Context, hostname string) error {
	// TODO: Configure Caddy to provision TLS cert for hostname
	return nil
}

package proxy

import "context"

type RouteConfig struct {
	Hostname  string
	TargetURL string
	TLS       bool
}

// ProxyManager abstracts reverse proxy operations.
type ProxyManager interface {
	AddRoute(ctx context.Context, cfg RouteConfig) error
	RemoveRoute(ctx context.Context, hostname string) error
	SetupTLS(ctx context.Context, hostname string) error
	ListRoutes(ctx context.Context) ([]RouteConfig, error)
}

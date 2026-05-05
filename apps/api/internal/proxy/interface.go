package proxy

import (
	"context"
	"encoding/json"
)

// RouteFeature represents a middleware feature applied to a route.
// Config is kept as raw JSON so it can be validated against a typed schema
// via ParseFeatureConfig at the route-builder boundary.
type RouteFeature struct {
	Type    string          `json:"type"`    // basic_auth, redirect, headers, ip_allowlist, rate_limit
	Config  json.RawMessage `json:"config"`
	Enabled bool            `json:"enabled"`
}

// RouteConfig describes a reverse-proxy route with optional TLS and middleware.
type RouteConfig struct {
	Hostname        string          `json:"hostname"`
	TargetURL       string          `json:"target_url"`
	TLS             bool            `json:"tls"`
	ForceHTTPS      bool            `json:"force_https"`
	SSLMode         string          `json:"ssl_mode"`          // automatic, dns_challenge, custom, off
	SSLProvider     string          `json:"ssl_provider"`      // e.g. cloudflare
	CertPath        string          `json:"cert_path"`         // for custom SSL mode
	KeyPath         string          `json:"key_path"`          // for custom SSL mode
	Features        []RouteFeature  `json:"features"`          // middleware features
	AdvancedConfig  json.RawMessage `json:"advanced_config"`   // raw Caddy JSON for power users
	HealthCheckPath string          `json:"health_check_path"` // optional path Caddy probes for upstream liveness
}

// ProxyManager abstracts reverse proxy operations.
type ProxyManager interface {
	AddRoute(ctx context.Context, cfg RouteConfig) error
	RemoveRoute(ctx context.Context, hostname string) error
	SetupTLS(ctx context.Context, hostname string, sslMode, certPath, keyPath string) error
	ListRoutes(ctx context.Context) ([]RouteConfig, error)
}

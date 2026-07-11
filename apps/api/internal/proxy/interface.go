package proxy

import (
	"context"
	"encoding/json"
)

const (
	// SSLModeOff serves plain HTTP only: no certificate is obtained for the
	// hostname and no HTTP→HTTPS redirect is rendered.
	SSLModeOff = "off"
	// SSLModeCustom serves an operator-uploaded certificate from the store.
	SSLModeCustom = "custom"
)

// RouteFeature represents a middleware feature applied to a route.
// Config is kept as raw JSON so it can be validated against a typed schema
// via ParseFeatureConfig at the route-builder boundary.
type RouteFeature struct {
	Type    string          `json:"type"` // basic_auth, redirect, headers, ip_allowlist, rate_limit
	Config  json.RawMessage `json:"config"`
	Enabled bool            `json:"enabled"`
}

// RouteConfig describes a reverse-proxy route with optional TLS and middleware.
type RouteConfig struct {
	Hostname    string `json:"hostname"`
	TargetURL   string `json:"target_url"`
	TLS         bool   `json:"tls"`
	ForceHTTPS  bool   `json:"force_https"`
	SSLMode     string `json:"ssl_mode"`     // automatic, dns_challenge, custom, off
	SSLProvider string `json:"ssl_provider"` // e.g. cloudflare
	// CertPEM/KeyPEM carry the decrypted certificate for ssl_mode=custom. They are
	// pushed to Caddy in-band (load_pem) rather than written to disk, so the key
	// never touches the proxy's filesystem. Treat as secret: never log them.
	CertPEM         string          `json:"-"`
	KeyPEM          string          `json:"-"`
	Features        []RouteFeature  `json:"features"`          // middleware features
	AdvancedConfig  json.RawMessage `json:"advanced_config"`   // raw Caddy JSON for power users
	HealthCheckPath string          `json:"health_check_path"` // optional path Caddy probes for upstream liveness
}

// HostCertificate is a decrypted certificate bound to the hostname it serves.
// The proxy tags loaded certificates by hostname so it can tell which of them it
// owns, and Caddy then selects between them by SNI.
type HostCertificate struct {
	Hostname string
	CertPEM  string
	KeyPEM   string
}

// Decryptor unwraps the envelope-encrypted certificate PEM stored in the
// database. *crypto.Keyring satisfies it; the proxy package takes the narrow
// interface so it does not depend on the keyring implementation.
type Decryptor interface {
	Decrypt(ciphertext []byte) ([]byte, error)
}

// ProxyManager abstracts reverse proxy operations.
type ProxyManager interface {
	AddRoute(ctx context.Context, cfg RouteConfig) error
	RemoveRoute(ctx context.Context, hostname string) error
	SetupTLS(ctx context.Context, hostname string, sslMode, certPEM, keyPEM string) error
	ListRoutes(ctx context.Context) ([]RouteConfig, error)

	// SyncCertificates makes the proxy's loaded certificates match the given set,
	// keyed by hostname. Like routes, certificates pushed over the admin API are
	// lost when Caddy restarts, so the reconciler re-asserts them each pass.
	SyncCertificates(ctx context.Context, certs []HostCertificate) error

	// SyncAutoHTTPS declares the hostnames excluded from automatic HTTPS — skip
	// for ssl_mode=off (no certificate at all) and skipCertificates for
	// ssl_mode=custom (the operator supplies it) — and re-asserts the server-level
	// config automatic HTTPS depends on. The reconciler calls it every pass: a
	// restarted Caddy comes back up with only its static Caddyfile, so the HTTPS
	// listener and these lists have to be re-applied the same way routes are.
	SyncAutoHTTPS(ctx context.Context, skip, skipCertificates []string) error
}

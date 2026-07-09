package caddy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/weiling79/belune/internal/pkg/metrics"
)

// SetupTLS error handling: transport-level failures are wrapped with
// ErrCaddyUnreachable so the service layer can tell "Caddy is down" apart
// from "Caddy rejected the config".

// tlsAutomationPolicy represents a Caddy TLS automation policy.
type tlsAutomationPolicy struct {
	Subjects []string     `json:"subjects"`
	Issuers  []tlsIssuer  `json:"issuers,omitempty"`
}

type tlsIssuer struct {
	Module    string            `json:"module"`
	Challenges *tlsChallenges   `json:"challenges,omitempty"`
}

type tlsChallenges struct {
	DNS *tlsDNSChallenge `json:"dns,omitempty"`
}

type tlsDNSChallenge struct {
	Provider tlsDNSProvider `json:"provider"`
}

type tlsDNSProvider struct {
	Name     string `json:"name"`
	APIToken string `json:"api_token,omitempty"`
}

// tlsManualCert represents a manually loaded certificate in Caddy.
type tlsManualCert struct {
	CertFile string `json:"certificate"`
	KeyFile  string `json:"key"`
	Tags     []string `json:"tags,omitempty"`
}

func (c *Client) SetupTLS(ctx context.Context, hostname, sslMode, certPath, keyPath string) (err error) {
	defer func() { metrics.RecordCaddyCall("setup_tls", err) }()
	switch sslMode {
	case "", "automatic":
		// Caddy automatically provisions TLS certificates via ACME.
		slog.Info("TLS will be auto-provisioned by Caddy", "hostname", hostname)
		return nil

	case "dns_challenge":
		// Add a TLS automation policy with DNS challenge.
		// The actual API token must be configured in Caddy's environment.
		policy := tlsAutomationPolicy{
			Subjects: []string{hostname},
			Issuers: []tlsIssuer{{
				Module: "acme",
				Challenges: &tlsChallenges{
					DNS: &tlsDNSChallenge{
						Provider: tlsDNSProvider{Name: "cloudflare"},
					},
				},
			}},
		}
		return c.addTLSPolicy(ctx, policy)

	case "custom":
		if certPath == "" || keyPath == "" {
			return fmt.Errorf("custom SSL mode requires cert_path and key_path")
		}
		// Load manual certificate into Caddy's TLS app.
		cert := tlsManualCert{
			CertFile: certPath,
			KeyFile:  keyPath,
			Tags:     []string{hostname},
		}
		return c.loadManualCert(ctx, cert)

	case "off":
		slog.Info("TLS disabled for hostname", "hostname", hostname)
		return nil

	default:
		return fmt.Errorf("unknown ssl_mode: %s", sslMode)
	}
}

// addTLSPolicy appends a TLS automation policy via the Caddy admin API.
func (c *Client) addTLSPolicy(ctx context.Context, policy tlsAutomationPolicy) error {
	body, err := json.Marshal(policy)
	if err != nil {
		return fmt.Errorf("marshal tls policy: %w", err)
	}

	url := fmt.Sprintf("%s/config/apps/tls/automation/policies", c.adminURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create tls policy request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if isTransportError(err) {
			return fmt.Errorf("%w: %v", ErrCaddyUnreachable, err)
		}
		return fmt.Errorf("add TLS policy: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("add TLS policy: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	slog.Info("caddy: TLS policy added", "subjects", policy.Subjects)
	return nil
}

// loadManualCert loads a manual TLS certificate via the Caddy admin API.
func (c *Client) loadManualCert(ctx context.Context, cert tlsManualCert) error {
	body, err := json.Marshal(cert)
	if err != nil {
		return fmt.Errorf("marshal manual cert: %w", err)
	}

	url := fmt.Sprintf("%s/config/apps/tls/certificates/load_files", c.adminURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create manual cert request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if isTransportError(err) {
			return fmt.Errorf("%w: %v", ErrCaddyUnreachable, err)
		}
		return fmt.Errorf("load manual cert: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("load manual cert: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	slog.Info("caddy: manual cert loaded", "tags", cert.Tags)
	return nil
}

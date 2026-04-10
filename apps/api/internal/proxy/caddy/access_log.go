package caddy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

// ConfigureAccessLogs configures the Caddy HTTP server to write structured JSON
// access logs. The logPath written to Caddy is always the container-internal
// path (/var/log/caddy/access.log); the host-side path used by the log tailer
// is a separate concern handled by the caller.
func (c *Client) ConfigureAccessLogs(ctx context.Context) error {
	const loggerName = "http.access.srv0"
	// Path inside the Caddy container — the bind mount makes this visible on
	// the host at a different path (e.g. infra/caddy/logs/access.log).
	const caddyLogPath = "/var/log/caddy/access.log"

	// Step 1: create/replace the named logger using PUT.
	// The logging.logs path may not exist yet, so PATCH would fail — use PUT on
	// the parent to create the entire logging section at once.
	loggingCfg := map[string]any{
		"logs": map[string]any{
			loggerName: map[string]any{
				"writer": map[string]any{
					"output":   "file", // Caddy uses "output" not "module" for writers
					"filename": caddyLogPath,
				},
				"encoder": map[string]any{
					"format": "json",
				},
				"level": "INFO",
			},
		},
	}
	if err := c.putConfig(ctx, "/config/logging", loggingCfg); err != nil {
		return fmt.Errorf("configure access log writer: %w", err)
	}

	// Step 2: point srv0 to use that logger.
	// By the time this runs, replaceAllRoutes (called from InitCatchAll) has already
	// patched srv0 with {listen, routes}, which clears the logs key — so PUT is safe.
	serverLogsCfg := map[string]any{
		"default_logger_name": loggerName,
	}
	if err := c.putConfig(ctx, "/config/apps/http/servers/srv0/logs", serverLogsCfg); err != nil {
		return fmt.Errorf("configure server access logs: %w", err)
	}

	slog.Info("caddy access logging configured", "caddy_path", caddyLogPath)
	return nil
}

func (c *Client) putConfig(ctx context.Context, path string, payload any) error {
	return c.doConfig(ctx, http.MethodPut, path, payload)
}

func (c *Client) patchConfig(ctx context.Context, path string, payload any) error {
	return c.doConfig(ctx, http.MethodPatch, path, payload)
}

func (c *Client) doConfig(ctx context.Context, method, path string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	url := c.adminURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Non-fatal in dev: Caddy may not be running
		slog.Warn("caddy config update failed (caddy may not be running)", "method", method, "path", path, "error", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		slog.Warn("caddy config update returned error", "method", method, "path", path, "status", resp.StatusCode, "body", string(respBody))
	}
	return nil
}

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
// access logs to the given file path for all virtual hosts in srv0.
// This is called at startup so that dynamically-added routes are also captured.
func (c *Client) ConfigureAccessLogs(ctx context.Context, logPath string) error {
	// Step 1: register the logger that writes to the file
	loggerName := "http.access.srv0"
	loggerCfg := map[string]any{
		"writer": map[string]any{
			"module":   "file",
			"filename": logPath,
		},
		"encoder": map[string]any{
			"format": "json",
		},
		"level": "INFO",
	}
	if err := c.patchConfig(ctx, fmt.Sprintf("/config/logging/logs/%s", loggerName), loggerCfg); err != nil {
		return fmt.Errorf("configure access log writer: %w", err)
	}

	// Step 2: point srv0 to use that logger for all access logs
	serverLogsCfg := map[string]any{
		"default_logger_name": loggerName,
	}
	if err := c.patchConfig(ctx, "/config/apps/http/servers/srv0/logs", serverLogsCfg); err != nil {
		return fmt.Errorf("configure server access logs: %w", err)
	}

	slog.Info("caddy access logging configured", "path", logPath)
	return nil
}

func (c *Client) patchConfig(ctx context.Context, path string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	url := c.adminURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Non-fatal in dev: Caddy may not be running
		slog.Warn("caddy config patch failed (caddy may not be running)", "path", path, "error", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		slog.Warn("caddy config patch returned error", "path", path, "status", resp.StatusCode, "body", string(respBody))
	}
	return nil
}

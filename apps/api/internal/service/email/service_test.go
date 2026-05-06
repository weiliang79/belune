package email_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ungweiliang/selfhost-paas/internal/config"
	"github.com/ungweiliang/selfhost-paas/internal/service/email"
)

// logOnlyCfg returns a config with no SMTP host (log-only mode).
func logOnlyCfg() *config.Config {
	return &config.Config{
		SMTPHost:      "",
		SMTPPort:      587,
		SMTPFromEmail: "noreply@example.com",
		SMTPFromName:  "Self-Hosted PaaS",
		SMTPTLSMode:   "starttls",
		PublicBaseURL: "https://paas.example.com",
	}
}

func TestSend_LogOnly(t *testing.T) {
	svc := email.New(logOnlyCfg())
	err := svc.Send(context.Background(), email.Message{
		To:       "user@example.com",
		Subject:  "Test",
		TextBody: "Hello",
	})
	require.NoError(t, err, "log-only mode must not return an error")
}

func TestTemplates_PasswordReset(t *testing.T) {
	svc := email.New(logOnlyCfg())

	vars := map[string]any{
		"FirstName": "Alice",
		"ResetURL":  "https://paas.example.com/reset-password?token=abc123",
	}

	// Use SendTemplate in log-only mode to exercise rendering without SMTP.
	err := svc.SendTemplate(context.Background(), "password_reset", "alice@example.com", vars)
	require.NoError(t, err)
}

func TestTemplates_UserInvitation(t *testing.T) {
	svc := email.New(logOnlyCfg())

	vars := map[string]any{
		"Role":      "operator",
		"InviteURL": "https://paas.example.com/accept-invite?token=xyz789",
	}

	err := svc.SendTemplate(context.Background(), "user_invitation", "new@example.com", vars)
	require.NoError(t, err)
}

func TestTemplates_AlertDeployFailed(t *testing.T) {
	svc := email.New(logOnlyCfg())

	vars := map[string]any{
		"AppName":      "my-app",
		"ProjectName":  "my-project",
		"DeploymentID": "deploy-uuid-123",
		"FailedAt":     "2026-05-06T12:00:00Z",
		"ErrorMessage": "container exited with code 1",
	}

	err := svc.SendTemplate(context.Background(), "alert_deploy_failed", "owner@example.com", vars)
	require.NoError(t, err)
}

func TestTemplates_AlertBuildFailed(t *testing.T) {
	svc := email.New(logOnlyCfg())

	vars := map[string]any{
		"AppName":      "my-app",
		"ProjectName":  "my-project",
		"BuildID":      "build-uuid-456",
		"FailedAt":     "2026-05-06T12:00:00Z",
		"ErrorMessage": "",
	}

	err := svc.SendTemplate(context.Background(), "alert_build_failed", "owner@example.com", vars)
	require.NoError(t, err)
}

func TestTemplates_AlertQuotaThreshold(t *testing.T) {
	svc := email.New(logOnlyCfg())

	vars := map[string]any{
		"ProjectName":      "my-project",
		"QuotaType":        "applications",
		"UsagePercent":     82,
		"ThresholdPercent": 80,
	}

	err := svc.SendTemplate(context.Background(), "alert_quota_threshold", "owner@example.com", vars)
	require.NoError(t, err)
}

func TestTemplates_UnknownTemplate(t *testing.T) {
	svc := email.New(logOnlyCfg())
	err := svc.SendTemplate(context.Background(), "nonexistent_template", "user@example.com", nil)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "unknown template"), "expected unknown template error, got: %s", err)
}

func TestNew_WarnOnMissingPublicBaseURL(t *testing.T) {
	// No panic — just a WARN in slog. Service should be constructed and be usable.
	cfg := &config.Config{
		SMTPHost:      "smtp.example.com",
		SMTPPort:      587,
		SMTPFromEmail: "noreply@example.com",
		SMTPFromName:  "Test",
		SMTPTLSMode:   "starttls",
		PublicBaseURL: "", // missing
	}
	svc := email.New(cfg)
	require.NotNil(t, svc)

	// Send must be a no-op (warn + return nil) not a crash.
	err := svc.Send(context.Background(), email.Message{
		To:      "user@example.com",
		Subject: "Test",
	})
	require.NoError(t, err)
}

func TestNew_WarnOnInvalidPublicBaseURL(t *testing.T) {
	cfg := &config.Config{
		SMTPHost:      "smtp.example.com",
		SMTPPort:      587,
		SMTPFromEmail: "noreply@example.com",
		SMTPFromName:  "Test",
		SMTPTLSMode:   "starttls",
		PublicBaseURL: "not-a-url",
	}
	svc := email.New(cfg)
	require.NotNil(t, svc)

	err := svc.Send(context.Background(), email.Message{
		To:      "user@example.com",
		Subject: "Test",
	})
	require.NoError(t, err)
}

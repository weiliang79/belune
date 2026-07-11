package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// HandleQuotaThresholdSweep walks every project, computes application-count
// usage %, and enqueues a quota-threshold alert email when the rising-edge
// condition is met for the project owner's configured threshold.
func (h *TaskHandler) HandleQuotaThresholdSweep(ctx context.Context, _ *asynq.Task) error {
	if h.QuotaService == nil || h.EmailService == nil || h.Enqueuer == nil {
		return nil
	}

	projects, err := h.Queries.ListAllProjects(ctx)
	if err != nil {
		return fmt.Errorf("list projects: %w", err)
	}

	var sweepErrs []error
	for _, proj := range projects {
		if err := h.maybeSendQuotaAlert(ctx, proj.ID, proj.UserID); err != nil {
			sweepErrs = append(sweepErrs, fmt.Errorf("project %v: %w", proj.ID, err))
		}
	}

	if len(sweepErrs) > 0 {
		slog.WarnContext(ctx, "quota threshold sweep: some projects had errors",
			"error_count", len(sweepErrs),
			"first_error", sweepErrs[0],
		)
	}
	return nil
}

// maybeSendQuotaAlert checks one project and fires an alert email if needed.
func (h *TaskHandler) maybeSendQuotaAlert(ctx context.Context, projectID, ownerUserID pgtype.UUID) error {
	// Get owner's alert preferences (default to threshold=80 if no row exists).
	threshold := 80
	prefs, err := h.Queries.GetAlertPreferences(ctx, ownerUserID)
	if err != nil && !isNoRows(err) {
		return fmt.Errorf("get alert preferences: %w", err)
	}
	if err == nil {
		if !prefs.QuotaThreshold {
			return nil
		}
		threshold = int(prefs.QuotaThresholdPercent)
	}

	alert, err := h.QuotaService.MaybeAlertProjectThreshold(ctx, projectID, threshold)
	if err != nil {
		return fmt.Errorf("check threshold: %w", err)
	}
	if alert == nil {
		return nil
	}

	// Get owner info for the email.
	ownerInfo, err := h.Queries.GetProjectOwnerInfo(ctx, projectID)
	if err != nil {
		return fmt.Errorf("get project owner info: %w", err)
	}

	task, err := NewEmailSendTask("alert_quota_threshold", ownerInfo.Email, map[string]any{
		"ProjectName":      ownerInfo.ProjectName,
		"QuotaType":        "applications",
		"UsagePercent":     alert.CurrentPercent,
		"ThresholdPercent": alert.Threshold,
	})
	if err != nil {
		return fmt.Errorf("build email task: %w", err)
	}
	if _, err := h.Enqueuer.Enqueue(task); err != nil {
		return fmt.Errorf("enqueue email task: %w", err)
	}

	slog.InfoContext(ctx, "quota threshold alert sent",
		"project_id", fmt.Sprintf("%v", projectID),
		"usage_percent", alert.CurrentPercent,
		"threshold", alert.Threshold,
	)

	if h.AuditLog != nil {
		h.AuditLog.Log("system", "", "alert_sent", "project",
			fmt.Sprintf("%v", projectID),
			map[string]any{"kind": "quota_threshold", "recipient_id": fmt.Sprintf("%v", ownerUserID)},
		)
	}

	return nil
}

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

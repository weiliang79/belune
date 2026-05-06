package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

// EmailSendPayload is the task payload for TypeEmailSend.
type EmailSendPayload struct {
	TemplateID string `json:"template_id"`
	To         string `json:"to"`
	Vars       any    `json:"vars"`
}

// NewEmailSendTask creates a TypeEmailSend asynq task.
func NewEmailSendTask(templateID, to string, vars any) (*asynq.Task, error) {
	payload, err := json.Marshal(EmailSendPayload{
		TemplateID: templateID,
		To:         to,
		Vars:       vars,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal email task payload: %w", err)
	}
	return asynq.NewTask(TypeEmailSend, payload,
		asynq.MaxRetry(3),
		asynq.Queue("default"),
	), nil
}

// HandleEmailSendTask processes a TypeEmailSend task.
// On final retry exhaustion, failure is dead-lettered to slog by the worker
// error handler (see worker.go ErrorHandler).
func (h *TaskHandler) HandleEmailSendTask(ctx context.Context, t *asynq.Task) error {
	var p EmailSendPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal email task payload: %w", err)
	}

	// Vars is decoded as map[string]any from JSON — pass directly to template renderer.
	if err := h.EmailService.SendTemplate(ctx, p.TemplateID, p.To, p.Vars); err != nil {
		return fmt.Errorf("send email (template=%s to=%s): %w", p.TemplateID, p.To, err)
	}

	return nil
}

// HandleAuthTokenCleanup deletes expired password-reset and invitation tokens.
// Safe to run frequently — DELETE WHERE is cheap on small tables.
func (h *TaskHandler) HandleAuthTokenCleanup(ctx context.Context, _ *asynq.Task) error {
	if err := h.Queries.DeleteExpiredPasswordResetTokens(ctx); err != nil {
		return fmt.Errorf("delete expired password reset tokens: %w", err)
	}
	if err := h.Queries.DeleteExpiredInvitations(ctx); err != nil {
		return fmt.Errorf("delete expired invitations: %w", err)
	}
	return nil
}

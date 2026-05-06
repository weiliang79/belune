package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

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

	if h.EmailService == nil {
		slog.WarnContext(ctx, "email task: no email service configured, skipping",
			"template_id", p.TemplateID,
			"to", p.To,
		)
		return nil
	}

	// Vars is decoded as map[string]any from JSON — pass directly to template renderer.
	if err := h.EmailService.SendTemplate(ctx, p.TemplateID, p.To, p.Vars); err != nil {
		return fmt.Errorf("send email (template=%s to=%s): %w", p.TemplateID, p.To, err)
	}

	return nil
}

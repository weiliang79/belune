package service

import (
	"context"
	"errors"

	"github.com/weiliang79/belune/internal/notify"
	"github.com/weiliang79/belune/internal/service/email"
)

// NewNotifyRegistry builds a notify.Registry wired to the app's SMTP mailer, so
// the email notification provider reuses the same delivery path as the rest of
// the app. Constructed in both the worker and the HTTP handler wiring; the
// registry is stateless, so separate instances are equivalent.
func NewNotifyRegistry(emailSvc *email.Service) *notify.Registry {
	return notify.NewRegistry(emailMailer{svc: emailSvc})
}

// emailMailer adapts *email.Service to notify.Mailer.
type emailMailer struct{ svc *email.Service }

func (m emailMailer) Send(ctx context.Context, msg notify.MailMessage) error {
	if m.svc == nil {
		return errors.New("email: no mailer configured")
	}
	return m.svc.Send(ctx, email.Message{
		To:       msg.To,
		Subject:  msg.Subject,
		TextBody: msg.TextBody,
	})
}

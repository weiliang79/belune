package service

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

const auditBufferSize = 256

// auditEntry is an internal type for the buffered channel.
type auditEntry struct {
	UserID       string
	Action       string
	ResourceType string
	ResourceID   string
	Details      map[string]any
	IPAddress    string
}

// AuditService provides non-blocking audit log capture.
// It writes entries to a buffered channel and a single goroutine drains them to the DB.
type AuditService struct {
	queries *generated.Queries
	ch      chan auditEntry
}

func NewAuditService(queries *generated.Queries) *AuditService {
	return &AuditService{
		queries: queries,
		ch:      make(chan auditEntry, auditBufferSize),
	}
}

// Run drains the audit channel and inserts entries to the database.
// Must be called in a goroutine. Stops when ctx is cancelled (drains remaining entries first).
func (s *AuditService) Run(ctx context.Context) {
	for {
		select {
		case entry, ok := <-s.ch:
			if !ok {
				return
			}
			s.insert(ctx, entry)
		case <-ctx.Done():
			// Drain remaining entries
			for {
				select {
				case entry, ok := <-s.ch:
					if !ok {
						return
					}
					s.insert(context.Background(), entry)
				default:
					return
				}
			}
		}
	}
}

func (s *AuditService) insert(ctx context.Context, entry auditEntry) {
	var userUUID pgtype.UUID
	if entry.UserID != "" {
		_ = userUUID.Scan(entry.UserID)
	}

	var detailsJSON []byte
	if entry.Details != nil {
		detailsJSON, _ = json.Marshal(entry.Details)
	}

	err := s.queries.CreateAuditLog(ctx, generated.CreateAuditLogParams{
		UserID:       userUUID,
		Action:       entry.Action,
		ResourceType: entry.ResourceType,
		ResourceID:   pgtype.Text{String: entry.ResourceID, Valid: entry.ResourceID != ""},
		Details:      detailsJSON,
		IpAddress:    pgtype.Text{String: entry.IPAddress, Valid: entry.IPAddress != ""},
	})
	if err != nil {
		slog.Warn("audit: failed to insert log", "action", entry.Action, "error", err)
	}
}

// Log sends an audit entry to the buffered channel (non-blocking).
// Caller provides userID and ipAddress directly (avoids import cycle with middleware).
func (s *AuditService) Log(userID, ipAddress, action, resourceType, resourceID string, details map[string]any) {
	entry := auditEntry{
		UserID:       userID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Details:      details,
		IPAddress:    ipAddress,
	}

	select {
	case s.ch <- entry:
	default:
		slog.Warn("audit: channel full, dropping entry", "action", action)
	}
}

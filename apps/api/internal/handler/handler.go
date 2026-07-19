package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/weiliang79/belune/internal/config"
	"github.com/weiliang79/belune/internal/proxy"
	"github.com/weiliang79/belune/internal/quota"
	"github.com/weiliang79/belune/internal/runtime"
	"github.com/weiliang79/belune/internal/server/middleware"
	"github.com/weiliang79/belune/internal/service"
	"github.com/weiliang79/belune/internal/service/email"
	"github.com/weiliang79/belune/internal/store/generated"
	"github.com/weiliang79/belune/internal/terminal"
	"github.com/weiliang79/belune/internal/ws"
)

// TaskEnqueuer abstracts task queue operations for testability.
type TaskEnqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// QueueInspector is the narrow slice of *asynq.Inspector the handler uses for
// queue maintenance: reporting depth and clearing dead-letter/retry tasks.
// Concrete impl is *asynq.Inspector; mocked in tests.
type QueueInspector interface {
	GetQueueInfo(queue string) (*asynq.QueueInfo, error)
	DeleteAllArchivedTasks(queue string) (int, error)
	DeleteAllRetryTasks(queue string) (int, error)
	// DeleteAllPendingTasks removes queued-but-not-started tasks. It never
	// touches active (running) tasks, so an in-flight deploy survives.
	DeleteAllPendingTasks(queue string) (int, error)
	// DeleteTask removes a single task by id. It fails for an active (running)
	// task, which lets callers distinguish a genuinely in-progress task from a
	// stale one (pending/retry/archived) holding a unique TaskID.
	DeleteTask(queue, id string) error
}

// ReconcilerStatusProvider is the narrow interface the handler consumes to
// surface proxy reconciler state and trigger an on-demand reconcile from the
// admin API. Concrete type lives in internal/proxy.
type ReconcilerStatusProvider interface {
	Status() proxy.ReconcilerStatus
	ReconcileNow(ctx context.Context) error
}

type Handler struct {
	cfg               *config.Config
	db                *pgxpool.Pool
	queries           *generated.Queries
	asynq             TaskEnqueuer
	inspector         QueueInspector
	runtime           runtime.ContainerRuntime
	proxy             proxy.ProxyManager
	reconciler        ReconcilerStatusProvider
	auth              *service.AuthService
	rdb               *redis.Client
	appService        *service.ApplicationService
	projService       *service.ProjectService
	dbService         *service.DatabaseService
	gitProviderSvc    *service.GitProviderConfigService
	gitIntegrationSvc *service.GitIntegrationService
	backupDestSvc     *service.BackupDestinationService
	hub               *ws.Hub
	auditSvc          *service.AuditService
	notifySvc         *service.NotificationService
	termManager       *terminal.Manager
	// docker system df is far too slow to run inside a request (33s on a small
	// VPS), so the Docker overview reads it from here.
	diskUsage diskUsageCache
	quotaSvc  *quota.Service
	emailSvc  *email.Service
	// certSvc is built here rather than injected: it needs only queries and the
	// keyring, both already held above, and nothing outside the handler uses it.
	certSvc *service.CertificateService
	// notifyChannelSvc is built here (like certSvc) from queries, keyring and the
	// email service. The worker holds its own equivalent instance for delivery.
	notifyChannelSvc *service.NotificationChannelService
}

func New(
	cfg *config.Config,
	db *pgxpool.Pool,
	queries *generated.Queries,
	asynqClient TaskEnqueuer,
	inspector QueueInspector,
	rt runtime.ContainerRuntime,
	pm proxy.ProxyManager,
	reconciler ReconcilerStatusProvider,
	auth *service.AuthService,
	rdb *redis.Client,
	appSvc *service.ApplicationService,
	projSvc *service.ProjectService,
	dbSvc *service.DatabaseService,
	gitProviderSvc *service.GitProviderConfigService,
	gitIntegrationSvc *service.GitIntegrationService,
	backupDestSvc *service.BackupDestinationService,
	hub *ws.Hub,
	auditSvc *service.AuditService,
	notifySvc *service.NotificationService,
	termMgr *terminal.Manager,
	quotaSvc *quota.Service,
	emailSvc *email.Service,
) *Handler {
	return &Handler{
		cfg:               cfg,
		db:                db,
		queries:           queries,
		asynq:             asynqClient,
		inspector:         inspector,
		runtime:           rt,
		proxy:             pm,
		reconciler:        reconciler,
		auth:              auth,
		rdb:               rdb,
		appService:        appSvc,
		projService:       projSvc,
		dbService:         dbSvc,
		gitProviderSvc:    gitProviderSvc,
		gitIntegrationSvc: gitIntegrationSvc,
		backupDestSvc:     backupDestSvc,
		hub:               hub,
		auditSvc:          auditSvc,
		notifySvc:         notifySvc,
		termManager:       termMgr,
		quotaSvc:          quotaSvc,
		emailSvc:          emailSvc,
		certSvc:           service.NewCertificateService(queries, cfg.Keyring),
		notifyChannelSvc:  service.NewNotificationChannelService(queries, cfg.Keyring, service.NewNotifyRegistry(emailSvc), cfg.PublicBaseURL),
	}
}

// audit is a nil-safe wrapper for audit logging. Extracts user ID + real client IP from request.
func (h *Handler) audit(r *http.Request, action, resourceType, resourceID string, details map[string]any) {
	if h.auditSvc != nil {
		userID := middleware.UserIDFromContext(r.Context())
		h.auditSvc.Log(userID, middleware.ClientIP(r), action, resourceType, resourceID, details)
	}
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Debug("writeJSON: encode error", "error", err)
	}
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// notImplemented returns a 501 stub response.
func notImplemented(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}

package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/ungweiliang/selfhost-paas/internal/config"
	"github.com/ungweiliang/selfhost-paas/internal/proxy"
	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/server/middleware"
	"github.com/ungweiliang/selfhost-paas/internal/service"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
	"github.com/ungweiliang/selfhost-paas/internal/terminal"
	"github.com/ungweiliang/selfhost-paas/internal/ws"
)

// TaskEnqueuer abstracts task queue operations for testability.
type TaskEnqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

type Handler struct {
	cfg         *config.Config
	db          *pgxpool.Pool
	queries     *generated.Queries
	asynq       TaskEnqueuer
	runtime     runtime.ContainerRuntime
	proxy       proxy.ProxyManager
	auth        *service.AuthService
	rdb         *redis.Client
	appService  *service.ApplicationService
	projService *service.ProjectService
	dbService   *service.DatabaseService
	gitCredSvc  *service.GitCredentialService
	hub         *ws.Hub
	auditSvc    *service.AuditService
	termManager *terminal.Manager
}

func New(
	cfg *config.Config,
	db *pgxpool.Pool,
	queries *generated.Queries,
	asynqClient TaskEnqueuer,
	rt runtime.ContainerRuntime,
	pm proxy.ProxyManager,
	auth *service.AuthService,
	rdb *redis.Client,
	appSvc *service.ApplicationService,
	projSvc *service.ProjectService,
	dbSvc *service.DatabaseService,
	gitCredSvc *service.GitCredentialService,
	hub *ws.Hub,
	auditSvc *service.AuditService,
	termMgr *terminal.Manager,
) *Handler {
	return &Handler{
		cfg:         cfg,
		db:          db,
		queries:     queries,
		asynq:       asynqClient,
		runtime:     rt,
		proxy:       pm,
		auth:        auth,
		rdb:         rdb,
		appService:  appSvc,
		projService: projSvc,
		dbService:   dbSvc,
		gitCredSvc:  gitCredSvc,
		hub:         hub,
		auditSvc:    auditSvc,
		termManager: termMgr,
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

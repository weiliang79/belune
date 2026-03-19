package handler

import (
	"encoding/json"
	"net/http"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/ungweiliang/selfhost-paas/internal/config"
	"github.com/ungweiliang/selfhost-paas/internal/proxy"
	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/service"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

// TaskEnqueuer abstracts task queue operations for testability.
type TaskEnqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

type Handler struct {
	cfg     *config.Config
	db      *pgxpool.Pool
	queries *generated.Queries
	asynq   TaskEnqueuer
	runtime runtime.ContainerRuntime
	proxy   proxy.ProxyManager
	auth    *service.AuthService
	rdb     *redis.Client
}

func New(cfg *config.Config, db *pgxpool.Pool, queries *generated.Queries, asynqClient TaskEnqueuer, rt runtime.ContainerRuntime, pm proxy.ProxyManager, auth *service.AuthService, rdb *redis.Client) *Handler {
	return &Handler{
		cfg:     cfg,
		db:      db,
		queries: queries,
		asynq:   asynqClient,
		runtime: rt,
		proxy:   pm,
		auth:    auth,
		rdb:     rdb,
	}
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// notImplemented returns a 501 stub response.
func notImplemented(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}

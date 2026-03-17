package handler

import (
	"encoding/json"
	"net/http"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ungweiliang/selfhost-paas/internal/config"
	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

type Handler struct {
	cfg     *config.Config
	db      *pgxpool.Pool
	queries *generated.Queries
	asynq   *asynq.Client
	runtime runtime.ContainerRuntime
}

func New(cfg *config.Config, db *pgxpool.Pool, queries *generated.Queries, asynqClient *asynq.Client, rt runtime.ContainerRuntime) *Handler {
	return &Handler{
		cfg:     cfg,
		db:      db,
		queries: queries,
		asynq:   asynqClient,
		runtime: rt,
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

package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/hibiken/asynq"

	"github.com/ungweiliang/selfhost-paas/internal/status"
)

type metricsResponse struct {
	Projects     int64          `json:"projects"`
	Applications int64          `json:"applications"`
	Databases    int64          `json:"databases"`
	Deployments  int64          `json:"deployments"`
	Containers   containerStats `json:"containers"`
}

type containerStats struct {
	Running int `json:"running"`
	Stopped int `json:"stopped"`
	Total   int `json:"total"`
}

func (h *Handler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	projects, err := h.queries.CountProjects(ctx)
	if err != nil {
		slog.Error("failed to count projects", "error", err)
	}
	applications, err := h.queries.CountApplications(ctx)
	if err != nil {
		slog.Error("failed to count applications", "error", err)
	}
	databases, err := h.queries.CountDatabases(ctx)
	if err != nil {
		slog.Error("failed to count databases", "error", err)
	}
	deployments, err := h.queries.CountDeployments(ctx)
	if err != nil {
		slog.Error("failed to count deployments", "error", err)
	}

	var stats containerStats
	containers, err := h.runtime.ListContainers(ctx)
	if err == nil {
		for _, c := range containers {
			stats.Total++
			if c.Status == status.ApplicationRunning {
				stats.Running++
			} else {
				stats.Stopped++
			}
		}
	}

	writeJSON(w, http.StatusOK, metricsResponse{
		Projects:     projects,
		Applications: applications,
		Databases:    databases,
		Deployments:  deployments,
		Containers:   stats,
	})
}

func (h *Handler) TriggerCleanup(w http.ResponseWriter, r *http.Request) {
	type cleanupRequest struct {
		RetainCount int `json:"retain_count,omitempty"`
	}

	var req cleanupRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	if req.RetainCount <= 0 {
		req.RetainCount = 3
	}

	payload, _ := json.Marshal(req)
	task := asynq.NewTask("cleanup", payload)
	if _, err := h.asynq.Enqueue(task, asynq.Queue("low")); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue cleanup task")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "cleanup queued"})
}

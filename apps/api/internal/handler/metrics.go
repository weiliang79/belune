package handler

import (
	"encoding/json"
	"net/http"

	"github.com/hibiken/asynq"
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

	projects, _ := h.queries.CountProjects(ctx)
	applications, _ := h.queries.CountApplications(ctx)
	databases, _ := h.queries.CountDatabases(ctx)
	deployments, _ := h.queries.CountDeployments(ctx)

	var stats containerStats
	containers, err := h.runtime.ListContainers(ctx)
	if err == nil {
		for _, c := range containers {
			stats.Total++
			if c.Status == "running" {
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

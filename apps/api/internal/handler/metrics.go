package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/hibiken/asynq"

	"github.com/weiling79/belune/internal/status"
)

type metricsResponse struct {
	Projects     int64          `json:"projects"`
	Applications int64          `json:"applications"`
	Databases    int64          `json:"databases"`
	Deployments  int64          `json:"deployments"`
	Containers   containerStats `json:"containers"`
}

type containerTypeCount struct {
	Running int `json:"running"`
	Total   int `json:"total"`
}

type containerStats struct {
	Running int `json:"running"`
	Stopped int `json:"stopped"`
	Error   int `json:"error"`
	Total   int `json:"total"`
	// ByType groups managed containers into the categories the platform
	// actually models: "application" (carries an application-id label) and
	// "database" (everything else managed). No synthetic worker/cron buckets.
	ByType map[string]containerTypeCount `json:"by_type"`
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

	stats := containerStats{ByType: map[string]containerTypeCount{}}
	containers, err := h.runtime.ListContainers(ctx)
	if err == nil {
		for _, c := range containers {
			stats.Total++
			switch c.Status {
			case status.ApplicationRunning:
				stats.Running++
			case "dead", "restarting":
				stats.Error++
			default:
				stats.Stopped++
			}

			kind := "database"
			if _, ok := c.Labels["application-id"]; ok {
				kind = "application"
			}
			tc := stats.ByType[kind]
			tc.Total++
			if c.Status == status.ApplicationRunning {
				tc.Running++
			}
			stats.ByType[kind] = tc
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
		RetainCount int      `json:"retain_count,omitempty"`
		Actions     []string `json:"actions,omitempty"` // empty = full cleanup
	}

	var req cleanupRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	if req.RetainCount <= 0 {
		req.RetainCount = 3
	}

	validActions := map[string]bool{
		"deployments": true, "images": true, "volumes": true,
		"containers": true, "build_cache": true,
	}
	for _, a := range req.Actions {
		if !validActions[a] {
			writeError(w, http.StatusBadRequest, "invalid cleanup action: "+a)
			return
		}
	}

	payload, _ := json.Marshal(req)
	task := asynq.NewTask("cleanup", payload)
	if _, err := h.asynq.Enqueue(task, asynq.Queue("low")); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue cleanup task")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "cleanup queued"})
}

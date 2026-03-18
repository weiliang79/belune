package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/git"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

func (h *Handler) HandleWebhookPush(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Warn("webhook: failed to read body", "error", err)
		w.WriteHeader(http.StatusOK)
		return
	}
	defer r.Body.Close()

	// We need to try each matching service's secret, so first find services
	// by trying to detect provider and extract repo URL from the raw payload.
	repoURL := extractRepoURL(body, r)
	if repoURL == "" {
		slog.Warn("webhook: could not extract repo URL")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Normalize repo URL for matching
	normalized := normalizeRepoURL(repoURL)

	// Find all services with this source_repo that have webhooks enabled
	services, err := h.queries.ListServicesBySourceRepo(r.Context(), pgtype.Text{
		String: normalized, Valid: true,
	})
	if err != nil {
		slog.Warn("webhook: failed to query services", "error", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	if len(services) == 0 {
		// Also try with .git suffix
		services, _ = h.queries.ListServicesBySourceRepo(r.Context(), pgtype.Text{
			String: normalized + ".git", Valid: true,
		})
	}

	if len(services) == 0 {
		slog.Debug("webhook: no matching services", "repo", normalized)
		w.WriteHeader(http.StatusOK)
		return
	}

	triggered := 0
	for _, svc := range services {
		secret := svc.WebhookSecret.String

		payload, err := git.ParseWebhook(r, body, secret)
		if err != nil {
			slog.Warn("webhook: parse/verify failed", "service", svc.Name, "error", err)
			continue
		}

		// Check if push branch matches auto_deploy_branch
		autoBranch := "main"
		if svc.AutoDeployBranch.Valid && svc.AutoDeployBranch.String != "" {
			autoBranch = svc.AutoDeployBranch.String
		}

		if payload.Branch != autoBranch {
			slog.Debug("webhook: branch mismatch",
				"service", svc.Name,
				"push_branch", payload.Branch,
				"auto_deploy_branch", autoBranch,
			)
			continue
		}

		// Create deployment and enqueue task
		serviceID := fmt.Sprintf("%x-%x-%x-%x-%x",
			svc.ID.Bytes[0:4], svc.ID.Bytes[4:6],
			svc.ID.Bytes[6:8], svc.ID.Bytes[8:10], svc.ID.Bytes[10:16])

		deployment, err := h.queries.CreateDeployment(r.Context(), generated.CreateDeploymentParams{
			ServiceID:   svc.ID,
			Status:      "pending",
			TriggeredBy: "push",
			CommitSha:   pgtype.Text{String: payload.CommitSHA, Valid: payload.CommitSHA != ""},
		})
		if err != nil {
			slog.Error("webhook: failed to create deployment", "service", svc.Name, "error", err)
			continue
		}

		deploymentID := fmt.Sprintf("%x-%x-%x-%x-%x",
			deployment.ID.Bytes[0:4], deployment.ID.Bytes[4:6],
			deployment.ID.Bytes[6:8], deployment.ID.Bytes[8:10], deployment.ID.Bytes[10:16])

		taskPayload, _ := json.Marshal(deployPayload{
			ServiceID:    serviceID,
			DeploymentID: deploymentID,
		})

		task := asynq.NewTask("deploy", taskPayload)
		if _, err := h.asynq.Enqueue(task, asynq.Queue("critical")); err != nil {
			slog.Error("webhook: failed to enqueue deploy", "service", svc.Name, "error", err)
			continue
		}

		slog.Info("webhook: triggered deploy",
			"service", svc.Name,
			"branch", payload.Branch,
			"commit", payload.CommitSHA,
		)
		triggered++
	}

	slog.Info("webhook: processing complete", "repo", normalized, "triggered", triggered)
	w.WriteHeader(http.StatusOK)
}

type updateWebhookRequest struct {
	WebhookSecret    *string `json:"webhook_secret"`
	AutoDeployBranch *string `json:"auto_deploy_branch"`
}

func (h *Handler) UpdateServiceWebhook(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "serviceId")
	var serviceUUID pgtype.UUID
	if err := serviceUUID.Scan(serviceID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid service id")
		return
	}

	var req updateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Get current service to preserve defaults
	current, err := h.queries.GetService(r.Context(), serviceUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}

	secret := current.WebhookSecret
	if req.WebhookSecret != nil {
		secret = pgtype.Text{String: *req.WebhookSecret, Valid: *req.WebhookSecret != ""}
	}

	branch := current.AutoDeployBranch
	if req.AutoDeployBranch != nil {
		b := *req.AutoDeployBranch
		if b == "" {
			b = "main"
		}
		branch = pgtype.Text{String: b, Valid: true}
	}

	svc, err := h.queries.UpdateServiceWebhook(r.Context(), generated.UpdateServiceWebhookParams{
		ID:               serviceUUID,
		WebhookSecret:    secret,
		AutoDeployBranch: branch,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update webhook settings")
		return
	}

	writeJSON(w, http.StatusOK, svc)
}

// extractRepoURL extracts the repository URL from the raw webhook payload
// without verifying signatures (that happens per-service).
func extractRepoURL(body []byte, r *http.Request) string {
	if r.Header.Get("X-GitHub-Event") != "" {
		var event struct {
			Repository struct {
				CloneURL string `json:"clone_url"`
			} `json:"repository"`
		}
		if err := json.Unmarshal(body, &event); err == nil {
			return event.Repository.CloneURL
		}
	}

	if r.Header.Get("X-Gitlab-Event") != "" {
		var event struct {
			Project struct {
				GitHTTPURL string `json:"git_http_url"`
			} `json:"project"`
		}
		if err := json.Unmarshal(body, &event); err == nil {
			return event.Project.GitHTTPURL
		}
	}

	return ""
}

// normalizeRepoURL strips .git suffix and lowercases for consistent matching.
func normalizeRepoURL(url string) string {
	url = strings.TrimSuffix(url, ".git")
	return strings.ToLower(url)
}

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/deploy"
	"github.com/ungweiliang/selfhost-paas/internal/git"
	"github.com/ungweiliang/selfhost-paas/internal/pkg/metrics"
	"github.com/ungweiliang/selfhost-paas/internal/pkg/tracing"
	"github.com/ungweiliang/selfhost-paas/internal/preview"
	"github.com/ungweiliang/selfhost-paas/internal/status"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

func (h *Handler) HandleWebhookPush(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	source := webhookSource(r)
	defer func() { metrics.RecordWebhookDelivery(source, time.Since(start)) }()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Warn("webhook: failed to read body", "error", err)
		w.WriteHeader(http.StatusOK)
		return
	}
	defer r.Body.Close()

	// We need to try each matching application's secret, so first find applications
	// by trying to detect provider and extract repo URL from the raw payload.
	repoURL := extractRepoURL(body, r)
	if repoURL == "" {
		slog.Warn("webhook: could not extract repo URL")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Normalize repo URL for matching
	normalized := normalizeRepoURL(repoURL)

	// Find all applications with this source_repo that have webhooks enabled
	applications, err := h.queries.ListApplicationsBySourceRepo(r.Context(), pgtype.Text{
		String: normalized, Valid: true,
	})
	if err != nil {
		slog.Warn("webhook: failed to query applications", "error", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	if len(applications) == 0 {
		// Also try with .git suffix — some webhook providers preserve the
		// trailing .git that we strip during normalisation.
		var fallbackErr error
		applications, fallbackErr = h.queries.ListApplicationsBySourceRepo(r.Context(), pgtype.Text{
			String: normalized + ".git", Valid: true,
		})
		if fallbackErr != nil {
			slog.Warn("webhook: .git suffix fallback query failed", "repo", normalized, "error", fallbackErr)
		}
	}

	if len(applications) == 0 {
		slog.Debug("webhook: no matching applications", "repo", normalized)
		w.WriteHeader(http.StatusOK)
		return
	}

	triggered := 0
	for _, app := range applications {
		secret := app.WebhookSecret.String

		payload, err := git.ParseWebhook(r, body, secret)
		if err != nil {
			slog.Warn("webhook: parse/verify failed", "application", app.Name, "error", err)
			continue
		}

		// Check if push branch matches auto_deploy_branch
		autoBranch := "main"
		if app.AutoDeployBranch.Valid && app.AutoDeployBranch.String != "" {
			autoBranch = app.AutoDeployBranch.String
		}

		if payload.Branch == autoBranch {
			if h.triggerPushDeploy(r, app, payload.Branch, payload.CommitSHA) {
				triggered++
			}
			continue
		}

		// Preview path: branch did not match auto_deploy_branch, but may match
		// a configured preview pattern on this parent app.
		if !app.PreviewBranchPattern.Valid || app.PreviewBranchPattern.String == "" ||
			!app.PreviewDomainTemplate.Valid || app.PreviewDomainTemplate.String == "" {
			slog.Debug("webhook: branch mismatch and no preview config",
				"application", app.Name,
				"push_branch", payload.Branch,
				"auto_deploy_branch", autoBranch,
			)
			continue
		}
		if !preview.MatchesPattern(app.PreviewBranchPattern.String, payload.Branch) {
			slog.Debug("webhook: branch does not match preview pattern",
				"application", app.Name,
				"push_branch", payload.Branch,
				"pattern", app.PreviewBranchPattern.String,
			)
			continue
		}

		previewApp, err := h.ensurePreviewApp(r.Context(), app, payload.Branch)
		if err != nil {
			slog.Error("webhook: failed to materialize preview app",
				"parent", app.Name,
				"branch", payload.Branch,
				"error", err,
			)
			continue
		}
		if h.triggerPushDeploy(r, previewApp, payload.Branch, payload.CommitSHA) {
			triggered++
		}
	}

	slog.Info("webhook: processing complete", "repo", normalized, "triggered", triggered)
	w.WriteHeader(http.StatusOK)
}

type updateWebhookRequest struct {
	WebhookSecret    *string `json:"webhook_secret"`
	AutoDeployBranch *string `json:"auto_deploy_branch"`
}

func (h *Handler) UpdateApplicationWebhook(w http.ResponseWriter, r *http.Request) {
	applicationID := chi.URLParam(r, "applicationId")
	var applicationUUID pgtype.UUID
	if err := applicationUUID.Scan(applicationID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid application id")
		return
	}

	if !h.canAccessApplication(r, applicationUUID) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	var req updateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Get current application to preserve defaults
	current, err := h.queries.GetApplication(r.Context(), applicationUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "application not found")
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

	app, err := h.queries.UpdateApplicationWebhook(r.Context(), generated.UpdateApplicationWebhookParams{
		ID:               applicationUUID,
		WebhookSecret:    secret,
		AutoDeployBranch: branch,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update webhook settings")
		return
	}

	writeJSON(w, http.StatusOK, app)
}

// webhookSource returns "github", "gitlab", or "unknown" based on request headers.
func webhookSource(r *http.Request) string {
	if r.Header.Get("X-GitHub-Event") != "" {
		return "github"
	}
	if r.Header.Get("X-Gitlab-Event") != "" {
		return "gitlab"
	}
	return "unknown"
}

// extractRepoURL extracts the repository URL from the raw webhook payload
// without verifying signatures (that happens per-application).
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

// triggerPushDeploy creates a deployment row and enqueues the deploy task for
// a specific application (parent or preview). Returns true when the task was
// successfully enqueued. Dedups duplicate webhook deliveries by commit SHA.
func (h *Handler) triggerPushDeploy(r *http.Request, app generated.Application, branch, commitSHA string) bool {
	applicationID := uuidToString(app.ID)

	var idempotencyKey pgtype.Text
	if commitSHA != "" {
		key := deploy.Key(applicationID, "push", commitSHA)
		idempotencyKey = pgtype.Text{String: key, Valid: true}

		existing, lookupErr := h.queries.FindRecentDeploymentByIdempotencyKey(r.Context(), generated.FindRecentDeploymentByIdempotencyKeyParams{
			ApplicationID:  app.ID,
			IdempotencyKey: idempotencyKey,
			WindowSeconds:  int32(deploy.WindowPushSeconds),
		})
		if lookupErr == nil {
			slog.Info("webhook: duplicate delivery, reusing existing deployment",
				"application", app.Name,
				"commit", commitSHA,
				"deployment_id", uuidToString(existing.ID),
			)
			return false
		}
		if !errors.Is(lookupErr, pgx.ErrNoRows) {
			slog.Warn("webhook: idempotency lookup failed, proceeding",
				"application", app.Name, "error", lookupErr,
			)
		}
	}

	deployment, err := h.queries.CreateDeployment(r.Context(), generated.CreateDeploymentParams{
		ApplicationID:  app.ID,
		Status:         status.DeploymentPending,
		TriggeredBy:    "push",
		CommitSha:      pgtype.Text{String: commitSHA, Valid: commitSHA != ""},
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		slog.Error("webhook: failed to create deployment", "application", app.Name, "error", err)
		return false
	}

	deploymentID := fmt.Sprintf("%x-%x-%x-%x-%x",
		deployment.ID.Bytes[0:4], deployment.ID.Bytes[4:6],
		deployment.ID.Bytes[6:8], deployment.ID.Bytes[8:10], deployment.ID.Bytes[10:16])

	taskPayload, marshalErr := json.Marshal(deployPayload{
		ApplicationID: applicationID,
		DeploymentID:  deploymentID,
		TraceCarrier:  tracing.InjectContext(r.Context()),
	})
	if marshalErr != nil {
		slog.Error("webhook: failed to marshal deploy payload", "application", app.Name, "error", marshalErr)
		return false
	}

	task := asynq.NewTask("deploy", taskPayload)
	if _, enqErr := h.asynq.Enqueue(task,
		asynq.Queue("critical"),
		asynq.Timeout(time.Duration(h.cfg.TaskTimeoutMinutes)*time.Minute),
		asynq.MaxRetry(3),
		asynq.TaskID("deploy:"+applicationID),
	); enqErr != nil {
		if !errors.Is(enqErr, asynq.ErrTaskIDConflict) {
			slog.Error("webhook: failed to enqueue deploy", "application", app.Name, "error", enqErr)
		} else {
			slog.Debug("webhook: deploy already in progress, skipping", "application", app.Name)
		}
		return false
	}

	slog.Info("webhook: triggered deploy",
		"application", app.Name,
		"branch", branch,
		"commit", commitSHA,
	)
	return true
}

// ensurePreviewApp returns the preview child app for (parent, branch), creating
// it and its derived domain on first push. It looks up the parent's project
// slug + base slug so the created app's container name follows the standard
// "{projectSlug}-{baseSlug}-{appID[:8]}" shape; a matching Domain row is
// upserted from the parent's preview_domain_template so the existing deploy
// worker wires up Caddy automatically.
func (h *Handler) ensurePreviewApp(ctx context.Context, parent generated.Application, branch string) (generated.Application, error) {
	parentRow, err := h.queries.GetApplicationWithProjectSlug(ctx, parent.ID)
	if err != nil {
		return generated.Application{}, fmt.Errorf("fetch parent: %w", err)
	}
	parentBaseSlug := deriveBaseSlug(parentRow.Slug, parentRow.ProjectSlug, uuidToString(parent.ID))
	if parentBaseSlug == "" {
		return generated.Application{}, fmt.Errorf("could not derive parent base slug from %q", parentRow.Slug)
	}

	branchSlug := preview.SlugifyBranch(branch)
	if branchSlug == "" {
		return generated.Application{}, fmt.Errorf("branch %q slugifies to empty", branch)
	}

	// Quota gate: previews count toward the parent project's caps. If this
	// branch already has a preview row, FindOrCreatePreview will return it
	// without creating, so we only validate when materialising a *new* one.
	if _, lookupErr := h.queries.GetPreviewByParentBranch(ctx, generated.GetPreviewByParentBranchParams{
		ParentApplicationID: pgtype.UUID{Bytes: parent.ID.Bytes, Valid: true},
		Branch:              pgtype.Text{String: branch, Valid: true},
	}); errors.Is(lookupErr, pgx.ErrNoRows) {
		if h.quotaSvc != nil {
			project, perr := h.queries.GetProject(ctx, parent.ProjectID)
			if perr != nil {
				return generated.Application{}, fmt.Errorf("fetch project for preview quota: %w", perr)
			}
			if qerr := h.quotaSvc.CheckApplicationCreate(ctx, parent.ProjectID, project.UserID, parent.CpuLimit, parent.MemoryLimit); qerr != nil {
				return generated.Application{}, qerr
			}
		}
	} else if lookupErr != nil {
		// Treat lookup failures as "let FindOrCreatePreview decide" — it
		// will surface the same error if it persists. We don't want a
		// transient DB blip to bypass the quota check.
		slog.Warn("webhook: preview lookup failed before quota check",
			"parent", parent.Name, "branch", branch, "error", lookupErr,
		)
	}

	child, created, err := h.appService.FindOrCreatePreview(
		ctx, parent, parentRow.ProjectSlug, parentBaseSlug, branch, branchSlug,
	)
	if err != nil {
		return generated.Application{}, err
	}

	hostname := preview.RenderDomainTemplate(parent.PreviewDomainTemplate.String, branch, parentBaseSlug)
	if hostname != "" {
		if _, derr := h.queries.GetDomainByHostname(ctx, hostname); errors.Is(derr, pgx.ErrNoRows) {
			if _, cerr := h.queries.CreateDomain(ctx, generated.CreateDomainParams{
				ApplicationID: child.ID,
				Hostname:      hostname,
				SslEnabled:    true,
				ForceHttps:    true,
				SslMode:       "automatic",
			}); cerr != nil {
				slog.Warn("webhook: failed to create preview domain",
					"hostname", hostname, "application", child.Name, "error", cerr,
				)
			}
		} else if derr != nil {
			slog.Warn("webhook: domain lookup failed", "hostname", hostname, "error", derr)
		}
	}

	if created {
		slog.Info("webhook: materialized preview app",
			"parent", parent.Name,
			"child", child.Name,
			"branch", branch,
			"hostname", hostname,
		)
	}
	return child, nil
}

// deriveBaseSlug inverts the "{projectSlug}-{baseSlug}-{appID[:8]}" format
// produced by ApplicationService.Create to recover the base slug.
func deriveBaseSlug(fullSlug, projectSlug, appID string) string {
	prefix := projectSlug + "-"
	if !strings.HasPrefix(fullSlug, prefix) {
		return ""
	}
	remainder := strings.TrimPrefix(fullSlug, prefix)
	if len(appID) < 8 {
		return remainder
	}
	suffix := "-" + appID[:8]
	if !strings.HasSuffix(remainder, suffix) {
		return remainder
	}
	return strings.TrimSuffix(remainder, suffix)
}

package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/ungweiliang/selfhost-paas/internal/build"
	"github.com/ungweiliang/selfhost-paas/internal/git"
	"github.com/ungweiliang/selfhost-paas/internal/naming"
	"github.com/ungweiliang/selfhost-paas/internal/pkg/buildlog"
	"github.com/ungweiliang/selfhost-paas/internal/pkg/metrics"
	"github.com/ungweiliang/selfhost-paas/internal/pkg/redact"
	"github.com/ungweiliang/selfhost-paas/internal/pkg/tracing"
	"github.com/ungweiliang/selfhost-paas/internal/proxy"
	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/status"
	"github.com/ungweiliang/selfhost-paas/internal/store"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

type deployPayload struct {
	ApplicationID    string            `json:"application_id"`
	DeploymentID     string            `json:"deployment_id"`
	RollbackImageTag string            `json:"rollback_image_tag,omitempty"` // non-empty = skip build, redeploy this image
	TraceCarrier     map[string]string `json:"trace_carrier,omitempty"`      // W3C trace context for span linking across the queue
}

// deployContext holds all state accumulated across deploy stages.
type deployContext struct {
	payload       deployPayload
	applicationID pgtype.UUID
	deploymentID  pgtype.UUID
	appRow        generated.GetApplicationWithProjectSlugRow
	app           generated.Application
	containerName string
	containerID   string
	env           map[string]string
	imageName     string
	commitSHA     string
	domains       []generated.Domain
	// compensators are cleanup functions appended as resources are created.
	// On any post-creation failure they run in reverse order.
	compensators []func()
}

// HandleDeployTask is the asynq task handler for application deployments.
// It drives a sequence of named stages; any failure after resource creation
// triggers the compensating cleanup chain.
func (h *TaskHandler) HandleDeployTask(ctx context.Context, t *asynq.Task) error {
	var payload deployPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal deploy payload: %w", err)
	}

	// Restore trace context from the enqueue site so the worker's spans
	// appear as children of the HTTP handler that triggered this deploy.
	ctx = tracing.ExtractContext(ctx, payload.TraceCarrier)
	ctx, rootSpan := tracing.Tracer().Start(ctx, "deploy.run",
		trace.WithAttributes(
			attribute.String("application.id", payload.ApplicationID),
			attribute.String("deployment.id", payload.DeploymentID),
			attribute.Bool("deploy.is_rollback", payload.RollbackImageTag != ""),
		),
	)
	defer rootSpan.End()

	slog.Info("handling deploy task", "application_id", payload.ApplicationID, "deployment_id", payload.DeploymentID)

	applicationID, err := parseUUID(payload.ApplicationID)
	if err != nil {
		recordSpanErr(rootSpan, err)
		return fmt.Errorf("invalid application_id (permanent): %w: %w", err, asynq.SkipRetry)
	}
	deploymentID, err := parseUUID(payload.DeploymentID)
	if err != nil {
		recordSpanErr(rootSpan, err)
		return fmt.Errorf("invalid deployment_id (permanent): %w: %w", err, asynq.SkipRetry)
	}

	// Defense-in-depth: if the deployment row is already in a terminal state,
	// a duplicate task slipped past webhook dedup and the asynq TaskID lock
	// (e.g., a stale retry after the first run finished). Skip without
	// retrying instead of clobbering the existing success/failure.
	existing, err := h.Queries.GetDeployment(ctx, deploymentID)
	if err != nil {
		recordSpanErr(rootSpan, err)
		return fmt.Errorf("fetch deployment (permanent): %w: %w", err, asynq.SkipRetry)
	}
	if existing.Status == status.DeploymentSuccess || existing.Status == status.DeploymentFailed {
		rootSpan.SetAttributes(attribute.String("deploy.skipped_reason", "already_terminal"))
		slog.Info("deploy task skipped: deployment already terminal",
			"deployment_id", payload.DeploymentID,
			"status", existing.Status,
		)
		return nil
	}

	dc := &deployContext{
		payload:       payload,
		applicationID: applicationID,
		deploymentID:  deploymentID,
	}

	// Stage 1: load application and decrypt env vars — permanent failure if missing
	if err := runStage(ctx, "deploy.load_application", func(ctx context.Context) error {
		return h.loadApplication(ctx, dc)
	}); err != nil {
		h.failDeployment(ctx, dc.deploymentID, fmt.Sprintf("load application: %v", err))
		recordSpanErr(rootSpan, err)
		return fmt.Errorf("load application (permanent): %w: %w", err, asynq.SkipRetry)
	}

	// Stage 2 & 3: idempotent cleanup and network setup — log-only on error
	_ = runStage(ctx, "deploy.cleanup_existing", func(ctx context.Context) error {
		h.cleanupExisting(ctx, dc)
		return nil
	})
	_ = runStage(ctx, "deploy.ensure_networks", func(ctx context.Context) error {
		h.ensureNetworks(ctx, dc)
		return nil
	})

	// Stage 4: build or pull image
	if err := runStage(ctx, "deploy.prepare_image", func(ctx context.Context) error {
		return h.prepareImage(ctx, dc)
	}); err != nil {
		h.failDeployment(ctx, dc.deploymentID, fmt.Sprintf("%v", err))
		recordSpanErr(rootSpan, err)
		return err
	}

	// Stage 5: create and start container — appends a compensator on success
	if err := runStage(ctx, "deploy.create_and_start", func(ctx context.Context) error {
		return h.createAndStart(ctx, dc)
	}); err != nil {
		h.runCompensators(dc)
		h.failDeployment(ctx, dc.deploymentID, fmt.Sprintf("create container: %v", err))
		recordSpanErr(rootSpan, err)
		return fmt.Errorf("create container: %w", err)
	}

	// Stage 6: optional health check
	if err := runStage(ctx, "deploy.check_health", func(ctx context.Context) error {
		return h.checkHealth(ctx, dc)
	}); err != nil {
		h.runCompensators(dc)
		h.failDeployment(ctx, dc.deploymentID, fmt.Sprintf("health check failed: %v", err))
		recordSpanErr(rootSpan, err)
		return fmt.Errorf("health check: %w", err)
	}

	// Stage 7: wire proxy routes — appends a compensator per successful AddRoute
	if err := runStage(ctx, "deploy.wire_proxy", func(ctx context.Context) error {
		return h.wireProxy(ctx, dc)
	}); err != nil {
		h.runCompensators(dc)
		h.failDeployment(ctx, dc.deploymentID, fmt.Sprintf("wire proxy: %v", err))
		recordSpanErr(rootSpan, err)
		return fmt.Errorf("wire proxy: %w", err)
	}

	// Stage 8: atomic finalize — mark deployment success + application running
	if err := runStage(ctx, "deploy.finalize", func(ctx context.Context) error {
		return h.finalize(ctx, dc)
	}); err != nil {
		slog.Error("failed to commit deploy success status", "application_id", payload.ApplicationID, "error", err)
		recordSpanErr(rootSpan, err)
	}

	rootSpan.SetAttributes(
		attribute.String("container.id", dc.containerID),
		attribute.String("deploy.commit_sha", dc.commitSHA),
	)
	rootSpan.SetStatus(codes.Ok, "deploy completed")

	slog.Info("deploy completed",
		"application_id", payload.ApplicationID,
		"deployment_id", payload.DeploymentID,
		"container", dc.containerID,
		"commit", dc.commitSHA,
	)
	return nil
}

// runStage wraps a deploy stage in its own span so per-stage latency and
// failures are attributable in traces. Emits a histogram sample per run so
// operators can track per-stage p95 / success ratios.
func runStage(ctx context.Context, name string, fn func(context.Context) error) error {
	ctx, span := tracing.Tracer().Start(ctx, name)
	defer span.End()
	start := time.Now()
	err := fn(ctx)
	resultLabel := "ok"
	if err != nil {
		resultLabel = "error"
		recordSpanErr(span, err)
	}
	metrics.RecordDeployStage(name, resultLabel, time.Since(start))
	return err
}

// recordSpanErr tags the span as failed and attaches the error.
func recordSpanErr(span trace.Span, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// loadApplication fetches the application row, maps it, and decrypts env vars.
func (h *TaskHandler) loadApplication(ctx context.Context, dc *deployContext) error {
	appRow, err := h.Queries.GetApplicationWithProjectSlug(ctx, dc.applicationID)
	if err != nil {
		return fmt.Errorf("fetch application: %w", err)
	}
	dc.appRow = appRow
	dc.app = generated.Application{
		ID: appRow.ID, ProjectID: appRow.ProjectID, Name: appRow.Name,
		Type: appRow.Type, SourceRepo: appRow.SourceRepo, SourceImage: appRow.SourceImage,
		DockerfilePath: appRow.DockerfilePath, BuildType: appRow.BuildType,
		BuilderImage: appRow.BuilderImage, CustomBuildpacks: appRow.CustomBuildpacks,
		Status:   appRow.Status,
		CpuLimit: appRow.CpuLimit, MemoryLimit: appRow.MemoryLimit,
		GitCredentialsEncrypted: appRow.GitCredentialsEncrypted,
		HealthCheckPath:         appRow.HealthCheckPath,
	}
	dc.containerName = naming.ContainerName(appRow.ProjectSlug, appRow.Slug, dc.payload.ApplicationID)

	// Decrypt env vars: project-level first (base), then app-level (override)
	env := make(map[string]string)
	projectEnvVars, err := h.Queries.ListProjectEnvVars(ctx, dc.app.ProjectID)
	if err != nil {
		slog.Warn("failed to fetch project env vars, continuing without them", "error", err)
	}
	for _, ev := range projectEnvVars {
		decrypted, err := h.Keyring.Decrypt(ev.ValueEncrypted)
		if err != nil {
			slog.Warn("failed to decrypt project env var, skipping", "key", ev.Key, "error", err)
			continue
		}
		env[ev.Key] = string(decrypted)
	}

	appEnvVars, err := h.Queries.ListEnvVarsByApplication(ctx, dc.applicationID)
	if err != nil {
		slog.Warn("failed to fetch app env vars, continuing without them", "error", err)
	}
	for _, ev := range appEnvVars {
		decrypted, err := h.Keyring.Decrypt(ev.ValueEncrypted)
		if err != nil {
			slog.Warn("failed to decrypt env var, skipping", "key", ev.Key, "error", err)
			continue
		}
		env[ev.Key] = string(decrypted)
	}
	dc.env = env
	return nil
}

// cleanupExisting stops and removes any pre-existing containers (all naming formats).
func (h *TaskHandler) cleanupExisting(ctx context.Context, dc *deployContext) {
	intermediateContainerName := naming.IntermediateContainerName(dc.appRow.ProjectSlug, dc.payload.ApplicationID)
	oldContainerName := naming.OldContainerName(dc.payload.ApplicationID)
	for _, name := range []string{oldContainerName, intermediateContainerName, dc.containerName} {
		if err := h.Runtime.StopContainer(ctx, name); err != nil {
			slog.Debug("could not stop container before deploy (may not exist)", "container", name, "error", err)
		}
		if err := h.Runtime.RemoveContainer(ctx, name); err != nil {
			slog.Debug("could not remove container before deploy (may not exist)", "container", name, "error", err)
		}
	}
}

// ensureNetworks creates the project-scoped network and attaches Caddy to it
// so the reverse proxy can reach this project's containers. Both operations
// are idempotent.
//
// v0.0.9-alpha Phase 2: stopped attaching app containers to the shared
// `paas-infra` network. Apps in different projects can no longer see each
// other on the Docker network plane. Caddy still bridges them by joining
// each project network on demand.
func (h *TaskHandler) ensureNetworks(ctx context.Context, dc *deployContext) {
	projectNetwork := naming.ProjectNetworkName(dc.appRow.ProjectSlug)
	if err := h.Runtime.CreateNetwork(ctx, projectNetwork); err != nil {
		slog.Debug("could not create project network (may already exist)", "network", projectNetwork, "error", err)
	}
	if name := h.Config.CaddyContainerName; name != "" {
		if err := h.Runtime.ConnectContainerToNetwork(ctx, name, projectNetwork); err != nil {
			// Caddy may legitimately be on a different orchestration plane
			// (e.g. running on the host, not in Docker). Warn rather than
			// fail — the deploy itself can still succeed.
			slog.Warn("could not attach caddy to project network", "caddy", name, "network", projectNetwork, "error", err)
		}
	}
}

// prepareImage resolves the image to deploy: rollback, image-pull, or git-build path.
func (h *TaskHandler) prepareImage(ctx context.Context, dc *deployContext) error {
	if dc.payload.RollbackImageTag != "" {
		dc.imageName = dc.payload.RollbackImageTag
		slog.Info("rolling back to image", "image", dc.imageName, "application_id", dc.payload.ApplicationID)
		h.updateDeploymentStatus(ctx, dc.deploymentID, status.DeploymentPending, status.DeploymentDeploying)
		return nil
	}

	switch dc.app.Type {
	case "image":
		h.updateDeploymentStatus(ctx, dc.deploymentID, status.DeploymentPending, status.DeploymentDeploying)
		dc.imageName = dc.app.SourceImage.String
		slog.Info("pulling image", "image", dc.imageName)
		pullCtx, pullCancel := context.WithTimeout(ctx, time.Duration(h.Config.ImagePullTimeoutMinutes)*time.Minute)
		defer pullCancel()
		if err := h.Runtime.PullImage(pullCtx, dc.imageName); err != nil {
			return fmt.Errorf("pull image: %w", err)
		}

	case "git":
		if err := h.buildFromGit(ctx, dc); err != nil {
			return err
		}

	default:
		return fmt.Errorf("unknown application type %s: %w", dc.app.Type, asynq.SkipRetry)
	}

	// Store the image tag for future rollback support.
	h.Queries.UpdateDeploymentImageTag(ctx, generated.UpdateDeploymentImageTagParams{
		ID:       dc.deploymentID,
		ImageTag: pgtype.Text{String: dc.imageName, Valid: true},
	})
	return nil
}

// buildFromGit clones the repository, runs the build chain, and sets dc.imageName.
func (h *TaskHandler) buildFromGit(ctx context.Context, dc *deployContext) error {
	dc.imageName = naming.ImageTag(dc.appRow.ProjectSlug, dc.appRow.Slug, dc.payload.ApplicationID, dc.payload.DeploymentID)

	tmpDir, err := os.MkdirTemp("", "paas-build-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	buildCtx, buildCancel := context.WithTimeout(ctx, time.Duration(h.Config.BuildTimeoutMinutes)*time.Minute)
	defer buildCancel()

	// Resolve git credentials: centralized credential takes priority, fallback to per-app token
	var cloneURL string
	if dc.app.GitCredentialID.Valid {
		cred, credErr := h.Queries.GetGitCredential(ctx, dc.app.GitCredentialID)
		if credErr != nil {
			slog.Warn("failed to fetch git credential, trying per-app token", "error", credErr)
		} else {
			tokenBytes, decErr := h.Keyring.Decrypt(cred.TokenEncrypted)
			if decErr != nil {
				slog.Warn("failed to decrypt centralized git credential", "error", decErr)
			} else {
				cloneURL = git.BuildCloneURL(cred.Provider, string(tokenBytes), cred.Username, dc.app.SourceRepo.String)
			}
		}
	}
	if cloneURL == "" && len(dc.app.GitCredentialsEncrypted) > 0 {
		tokenBytes, decErr := h.Keyring.Decrypt(dc.app.GitCredentialsEncrypted)
		if decErr != nil {
			slog.Warn("failed to decrypt git credentials, cloning without token", "error", decErr)
		} else {
			cloneURL = git.BuildCloneURL("generic", string(tokenBytes), "", dc.app.SourceRepo.String)
		}
	}
	if cloneURL == "" {
		cloneURL = dc.app.SourceRepo.String
	}

	// Previews pin the clone to their branch; parents (branch unset) clone the
	// default ref. Token is empty here because cloneURL already embeds it when
	// credentials are present.
	branch := dc.appRow.Branch.String
	slog.Info("cloning repository", "repo", dc.app.SourceRepo.String, "dest", tmpDir, "branch", branch)
	cloneResult, err := git.Clone(buildCtx, cloneURL, tmpDir, branch, "")
	if err != nil {
		return fmt.Errorf("git clone: %w", err)
	}
	dc.commitSHA = cloneResult.CommitSHA
	slog.Info("cloned repository", "commit", dc.commitSHA)

	var customBuildpacks []string
	if len(dc.app.CustomBuildpacks) > 0 {
		if err := json.Unmarshal(dc.app.CustomBuildpacks, &customBuildpacks); err != nil {
			slog.Debug("could not parse custom buildpacks, using defaults", "app_id", dc.payload.ApplicationID, "error", err)
		}
	}

	pub := buildlog.NewPublisher(h.RedisClient, dc.payload.DeploymentID)
	logWriter := buildlog.NewLineWriter(pub, buildCtx)

	buildOpts := build.BuildOptions{
		SourceDir:      tmpDir,
		ImageTag:       dc.imageName,
		DockerfilePath: dc.app.DockerfilePath.String,
		BuilderImage:   dc.app.BuilderImage.String,
		Buildpacks:     customBuildpacks,
		Env:            dc.env,
		BuildType:      dc.app.BuildType,
		LogWriter:      logWriter,
		ApplicationID:  dc.payload.ApplicationID,
	}

	h.updateDeploymentStatus(ctx, dc.deploymentID, status.DeploymentPending, status.DeploymentBuilding)

	slog.Info("building image", "tag", dc.imageName)
	result, err := h.Chain.Build(buildCtx, buildOpts)
	logWriter.Flush()
	pub.Close(ctx)
	if err != nil {
		return fmt.Errorf("build: %w", err)
	}

	if result.Logs != "" {
		h.Queries.UpdateDeploymentBuildLogs(ctx, generated.UpdateDeploymentBuildLogsParams{
			ID:        dc.deploymentID,
			BuildLogs: pgtype.Text{String: result.Logs, Valid: true},
		})
	}

	dc.imageName = result.ImageTag
	slog.Info("build completed", "image", dc.imageName)
	h.updateDeploymentStatus(ctx, dc.deploymentID, status.DeploymentBuilding, status.DeploymentDeploying)
	return nil
}

// createAndStart creates the container, starts it, and connects it to paas-infra.
// On success it appends a compensator that stops and removes the container.
func (h *TaskHandler) createAndStart(ctx context.Context, dc *deployContext) error {
	containerID, err := h.Runtime.CreateContainer(ctx, runtime.ContainerConfig{
		Name:            dc.containerName,
		Image:           dc.imageName,
		Env:             dc.env,
		Ports:           map[string]string{},
		Network:         naming.ProjectNetworkName(dc.appRow.ProjectSlug),
		Labels:          map[string]string{"application-id": dc.payload.ApplicationID},
		CPULimit:        dc.app.CpuLimit,
		MemoryLimit:     dc.app.MemoryLimit,
		HealthCheckPath: dc.app.HealthCheckPath.String,
		// Security hardening (v0.0.9-alpha Phase 2): drop all capabilities,
		// disallow privilege escalation, and run with a read-only rootfs +
		// tmpfs for the conventional writable paths. Apps that need more
		// can request specific capabilities back via CapAdd in a future
		// per-app config; default-deny is safer for the average user.
		CapDrop:        []string{"ALL"},
		SecurityOpt:    []string{"no-new-privileges"},
		ReadonlyRootfs: true,
		Tmpfs: map[string]string{
			"/tmp": "",
			"/run": "",
		},
	})
	if err != nil {
		return fmt.Errorf("create container: %w", err)
	}

	if err := h.Runtime.StartContainer(ctx, containerID); err != nil {
		// Container created but not started — remove it before returning
		h.Runtime.RemoveContainer(context.Background(), containerID)
		return fmt.Errorf("start container: %w", err)
	}

	// No paas-infra cross-attach: project network isolation (v0.0.9 Phase 2).
	// Caddy was attached to the project network in ensureNetworks above so
	// it can already reach this container by hostname.

	dc.containerID = containerID

	// Compensator: stop and remove this container if a later stage fails
	dc.compensators = append(dc.compensators, func() {
		cleanCtx := context.Background()
		if err := h.Runtime.StopContainer(cleanCtx, dc.containerID); err != nil {
			slog.Warn("compensator: failed to stop container", "container", dc.containerID, "error", err)
		}
		if err := h.Runtime.RemoveContainer(cleanCtx, dc.containerID); err != nil {
			slog.Warn("compensator: failed to remove container", "container", dc.containerID, "error", err)
		}
	})
	return nil
}

// checkHealth polls the container's health endpoint if configured.
func (h *TaskHandler) checkHealth(ctx context.Context, dc *deployContext) error {
	if !dc.app.HealthCheckPath.Valid || dc.app.HealthCheckPath.String == "" {
		return nil
	}

	// Load domains to resolve the container port
	domains, err := h.Queries.ListDomainsByApplication(ctx, dc.applicationID)
	if err != nil {
		slog.Warn("failed to list domains for health check port resolution", "application_id", dc.payload.ApplicationID, "error", err)
	}
	dc.domains = domains

	healthURL := fmt.Sprintf("http://%s:%d%s", dc.containerName, resolveContainerPort(domains), dc.app.HealthCheckPath.String)
	slog.Info("waiting for container health check", "url", healthURL)
	if err := pollHealthCheck(ctx, healthURL, 60*time.Second); err != nil {
		return err
	}
	slog.Info("container health check passed", "container", dc.containerName)
	return nil
}

// wireProxy adds a Caddy route for each domain. On success it appends a compensator
// per route so they can be removed if a later stage fails.
func (h *TaskHandler) wireProxy(ctx context.Context, dc *deployContext) error {
	// Load domains if not already fetched by checkHealth
	if dc.domains == nil {
		domains, err := h.Queries.ListDomainsByApplication(ctx, dc.applicationID)
		if err != nil {
			slog.Warn("failed to list domains for application", "application_id", dc.payload.ApplicationID, "error", err)
		}
		dc.domains = domains
	}

	for _, domain := range dc.domains {
		cfg, err := proxy.BuildRouteConfigFromDB(ctx, h.Queries, domain, dc.containerName)
		if err != nil {
			return fmt.Errorf("build route config for %s: %w", domain.Hostname, err)
		}
		if err := h.Proxy.AddRoute(ctx, cfg); err != nil {
			return fmt.Errorf("add route for %s: %w", domain.Hostname, err)
		}

		// Compensator: remove this route if a later stage fails
		hostname := domain.Hostname
		dc.compensators = append(dc.compensators, func() {
			if err := h.Proxy.RemoveRoute(context.Background(), hostname); err != nil {
				slog.Warn("compensator: failed to remove proxy route", "hostname", hostname, "error", err)
			}
		})
	}
	return nil
}

// finalize atomically marks the deployment as succeeded and the application as running.
func (h *TaskHandler) finalize(ctx context.Context, dc *deployContext) error {
	return store.WithTx(ctx, h.DB, func(q *generated.Queries) error {
		if !status.ValidTransition(status.DeploymentDeploying, status.DeploymentSuccess) {
			slog.Warn("invalid deployment transition skipped", "from", status.DeploymentDeploying, "to", status.DeploymentSuccess)
			return nil
		}
		q.UpdateDeploymentStatus(ctx, generated.UpdateDeploymentStatusParams{
			ID:     dc.deploymentID,
			Status: status.DeploymentSuccess,
		})
		q.UpdateApplicationStatus(ctx, generated.UpdateApplicationStatusParams{
			ID:     dc.applicationID,
			Status: status.ApplicationRunning,
		})
		// Stamp last_activity_at for preview GC. Doing this on every deploy
		// (not just previews) keeps the column meaningful if we later extend
		// idle cleanup to parents too.
		q.TouchApplicationActivity(ctx, dc.applicationID)
		return nil
	})
}

// runCompensators runs all compensating cleanup functions in reverse order.
func (h *TaskHandler) runCompensators(dc *deployContext) {
	for i := len(dc.compensators) - 1; i >= 0; i-- {
		dc.compensators[i]()
	}
}

func (h *TaskHandler) failDeployment(ctx context.Context, deploymentID pgtype.UUID, errMsg string) {
	retried, _ := asynq.GetRetryCount(ctx)
	maxRetry, _ := asynq.GetMaxRetry(ctx)

	safeMsg := redact.Error(errMsg)

	slog.Error("deployment failed",
		"deployment_id", fmt.Sprintf("%v", deploymentID),
		"error", safeMsg,
		"retry", retried,
		"max_retry", maxRetry,
	)

	if retried >= maxRetry {
		h.Queries.UpdateDeploymentStatus(ctx, generated.UpdateDeploymentStatusParams{
			ID:           deploymentID,
			Status:       status.DeploymentFailed,
			ErrorMessage: pgtype.Text{String: safeMsg, Valid: true},
		})
	}
}

// updateDeploymentStatus validates the transition, updates the deployment status,
// and stamps the relevant timing column for the new state.
func (h *TaskHandler) updateDeploymentStatus(ctx context.Context, id pgtype.UUID, from, to string) {
	if !status.ValidTransition(from, to) {
		slog.Warn("invalid deployment transition skipped", "from", from, "to", to,
			"deployment_id", fmt.Sprintf("%v", id))
		return
	}
	h.Queries.UpdateDeploymentStatus(ctx, generated.UpdateDeploymentStatusParams{
		ID:     id,
		Status: to,
	})
	switch to {
	case status.DeploymentBuilding:
		h.Queries.SetDeploymentBuildStarted(ctx, id)
	case status.DeploymentDeploying:
		if from == status.DeploymentBuilding {
			h.Queries.SetDeploymentBuildEnded(ctx, id)
		}
		h.Queries.SetDeploymentDeployStarted(ctx, id)
	}
}

// parseUUID parses a UUID string into a pgtype.UUID, returning an error on invalid input.
func parseUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		return pgtype.UUID{}, fmt.Errorf("invalid UUID %q: %w", s, err)
	}
	return u, nil
}

// resolveContainerPort returns the container port from the first domain that has one, or 8080.
func resolveContainerPort(domains []generated.Domain) int32 {
	for _, d := range domains {
		if d.ContainerPort.Valid {
			return d.ContainerPort.Int32
		}
	}
	return 8080
}

// pollHealthCheck repeatedly GETs url until a 2xx response is received or deadline expires.
func pollHealthCheck(ctx context.Context, url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 5 * time.Second}
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("build health check request: %w", err)
		}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("health check at %s did not return 2xx within %s", url, timeout)
}

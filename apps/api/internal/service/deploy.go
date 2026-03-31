package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/naming"
	"github.com/ungweiliang/selfhost-paas/internal/proxy"
	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

type DeployService struct {
	runtime      runtime.ContainerRuntime
	proxy        proxy.ProxyManager
	queries      *generated.Queries
	asynq        *asynq.Client
	taskTimeout  time.Duration
}

func NewDeployService(
	rt runtime.ContainerRuntime,
	pm proxy.ProxyManager,
	queries *generated.Queries,
	asynqClient *asynq.Client,
	taskTimeoutMinutes int,
) *DeployService {
	return &DeployService{
		runtime:     rt,
		proxy:       pm,
		queries:     queries,
		asynq:       asynqClient,
		taskTimeout: time.Duration(taskTimeoutMinutes) * time.Minute,
	}
}

// DeployTaskPayload is the JSON payload sent to the deploy worker.
type DeployTaskPayload struct {
	ApplicationID string `json:"application_id"`
	DeploymentID  string `json:"deployment_id"`
}

// Deploy creates a deployment record and enqueues the deploy task.
func (s *DeployService) Deploy(ctx context.Context, applicationID pgtype.UUID) (generated.Deployment, error) {
	deployment, err := s.queries.CreateDeployment(ctx, generated.CreateDeploymentParams{
		ApplicationID: applicationID,
		Status:        "pending",
		TriggeredBy:   "manual",
	})
	if err != nil {
		return generated.Deployment{}, fmt.Errorf("create deployment: %w", err)
	}

	payload, err := json.Marshal(DeployTaskPayload{
		ApplicationID: uuidToString(applicationID),
		DeploymentID:  uuidToString(deployment.ID),
	})
	if err != nil {
		return generated.Deployment{}, fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask("deploy", payload)
	_, err = s.asynq.Enqueue(task,
		asynq.Queue("critical"),
		asynq.Timeout(s.taskTimeout),
		asynq.MaxRetry(3),
		asynq.TaskID("deploy:"+uuidToString(applicationID)),
	)
	if err != nil {
		return generated.Deployment{}, fmt.Errorf("enqueue deploy task: %w", err)
	}

	slog.Info("deploy task enqueued", "application_id", uuidToString(applicationID), "deployment_id", uuidToString(deployment.ID))
	return deployment, nil
}

// Stop stops a running application container.
func (s *DeployService) Stop(ctx context.Context, applicationID pgtype.UUID) error {
	row, err := s.queries.GetApplicationWithProjectSlug(ctx, applicationID)
	if err != nil {
		return fmt.Errorf("get application: %w", err)
	}
	applicationIDStr := uuidToString(applicationID)
	containerName := naming.ContainerName(row.ProjectSlug, row.Slug, applicationIDStr)
	if err := s.runtime.StopContainer(ctx, containerName); err != nil {
		return fmt.Errorf("stop container: %w", err)
	}
	_, err = s.queries.UpdateApplicationStatus(ctx, generated.UpdateApplicationStatusParams{
		ID:     applicationID,
		Status: "stopped",
	})
	return err
}

// Restart stops and re-deploys an application.
func (s *DeployService) Restart(ctx context.Context, applicationID pgtype.UUID) (generated.Deployment, error) {
	row, err := s.queries.GetApplicationWithProjectSlug(ctx, applicationID)
	if err != nil {
		return generated.Deployment{}, fmt.Errorf("get application: %w", err)
	}
	applicationIDStr := uuidToString(applicationID)
	containerName := naming.ContainerName(row.ProjectSlug, row.Slug, applicationIDStr)
	if err := s.runtime.StopContainer(ctx, containerName); err != nil {
		slog.Warn("could not stop container before restart", "container", containerName, "error", err)
	}
	if err := s.runtime.RemoveContainer(ctx, containerName); err != nil {
		slog.Warn("could not remove container before restart", "container", containerName, "error", err)
	}

	return s.Deploy(ctx, applicationID)
}

func uuidToString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	b := u.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

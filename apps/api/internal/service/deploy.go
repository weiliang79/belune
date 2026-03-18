package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/naming"
	"github.com/ungweiliang/selfhost-paas/internal/proxy"
	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

type DeployService struct {
	runtime runtime.ContainerRuntime
	proxy   proxy.ProxyManager
	queries *generated.Queries
	asynq   *asynq.Client
}

func NewDeployService(
	rt runtime.ContainerRuntime,
	pm proxy.ProxyManager,
	queries *generated.Queries,
	asynqClient *asynq.Client,
) *DeployService {
	return &DeployService{
		runtime: rt,
		proxy:   pm,
		queries: queries,
		asynq:   asynqClient,
	}
}

// DeployTaskPayload is the JSON payload sent to the deploy worker.
type DeployTaskPayload struct {
	ServiceID    string `json:"service_id"`
	DeploymentID string `json:"deployment_id"`
}

// Deploy creates a deployment record and enqueues the deploy task.
func (s *DeployService) Deploy(ctx context.Context, serviceID pgtype.UUID) (generated.Deployment, error) {
	deployment, err := s.queries.CreateDeployment(ctx, generated.CreateDeploymentParams{
		ServiceID:   serviceID,
		Status:      "pending",
		TriggeredBy: "manual",
	})
	if err != nil {
		return generated.Deployment{}, fmt.Errorf("create deployment: %w", err)
	}

	payload, err := json.Marshal(DeployTaskPayload{
		ServiceID:    uuidToString(serviceID),
		DeploymentID: uuidToString(deployment.ID),
	})
	if err != nil {
		return generated.Deployment{}, fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask("deploy", payload)
	_, err = s.asynq.Enqueue(task, asynq.Queue("critical"))
	if err != nil {
		return generated.Deployment{}, fmt.Errorf("enqueue deploy task: %w", err)
	}

	slog.Info("deploy task enqueued", "service_id", uuidToString(serviceID), "deployment_id", uuidToString(deployment.ID))
	return deployment, nil
}

// Stop stops a running service container.
func (s *DeployService) Stop(ctx context.Context, serviceID pgtype.UUID) error {
	row, err := s.queries.GetServiceWithProjectSlug(ctx, serviceID)
	if err != nil {
		return fmt.Errorf("get service: %w", err)
	}
	serviceIDStr := uuidToString(serviceID)
	containerName := naming.ContainerName(row.ProjectSlug, row.Slug, serviceIDStr)
	if err := s.runtime.StopContainer(ctx, containerName); err != nil {
		return fmt.Errorf("stop container: %w", err)
	}
	_, err = s.queries.UpdateServiceStatus(ctx, generated.UpdateServiceStatusParams{
		ID:     serviceID,
		Status: "stopped",
	})
	return err
}

// Restart stops and re-deploys a service.
func (s *DeployService) Restart(ctx context.Context, serviceID pgtype.UUID) (generated.Deployment, error) {
	row, err := s.queries.GetServiceWithProjectSlug(ctx, serviceID)
	if err != nil {
		return generated.Deployment{}, fmt.Errorf("get service: %w", err)
	}
	serviceIDStr := uuidToString(serviceID)
	containerName := naming.ContainerName(row.ProjectSlug, row.Slug, serviceIDStr)
	_ = s.runtime.StopContainer(ctx, containerName)
	_ = s.runtime.RemoveContainer(ctx, containerName)

	return s.Deploy(ctx, serviceID)
}

func uuidToString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	b := u.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

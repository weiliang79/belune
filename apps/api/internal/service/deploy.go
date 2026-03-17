package service

import "context"

type DeployService struct{}

func NewDeployService() *DeployService {
	return &DeployService{}
}

func (s *DeployService) Deploy(ctx context.Context, serviceID string) (string, error)  { return "", nil }
func (s *DeployService) Stop(ctx context.Context, serviceID string) error              { return nil }
func (s *DeployService) Restart(ctx context.Context, serviceID string) error           { return nil }
func (s *DeployService) GetStatus(ctx context.Context, deploymentID string) (any, error) { return nil, nil }

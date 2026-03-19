package service

import "context"

type EnvVarService struct{}

func NewEnvVarService() *EnvVarService {
	return &EnvVarService{}
}

func (s *EnvVarService) List(ctx context.Context, applicationID string) (any, error)     { return nil, nil }
func (s *EnvVarService) Update(ctx context.Context, applicationID string, vars map[string]string) error { return nil }

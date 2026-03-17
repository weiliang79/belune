package service

import "context"

type MetricsService struct{}

func NewMetricsService() *MetricsService {
	return &MetricsService{}
}

func (s *MetricsService) GetOverview(ctx context.Context) (any, error) { return nil, nil }

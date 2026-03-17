package service

import "context"

type DatabaseService struct{}

func NewDatabaseService() *DatabaseService {
	return &DatabaseService{}
}

func (s *DatabaseService) List(ctx context.Context, projectID string) (any, error)                  { return nil, nil }
func (s *DatabaseService) Create(ctx context.Context, projectID, name, dbType, version string) (any, error) { return nil, nil }
func (s *DatabaseService) Get(ctx context.Context, id string) (any, error)                          { return nil, nil }
func (s *DatabaseService) Delete(ctx context.Context, id string) error                              { return nil }

package service

import "context"

type ProjectService struct{}

func NewProjectService() *ProjectService {
	return &ProjectService{}
}

func (s *ProjectService) List(ctx context.Context, userID string) (any, error)           { return nil, nil }
func (s *ProjectService) Create(ctx context.Context, userID, name string) (any, error)   { return nil, nil }
func (s *ProjectService) Get(ctx context.Context, id string) (any, error)                { return nil, nil }
func (s *ProjectService) Update(ctx context.Context, id, name string) (any, error)       { return nil, nil }
func (s *ProjectService) Delete(ctx context.Context, id string) error                    { return nil }

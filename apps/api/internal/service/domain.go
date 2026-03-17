package service

import "context"

type DomainService struct{}

func NewDomainService() *DomainService {
	return &DomainService{}
}

func (s *DomainService) List(ctx context.Context, serviceID string) (any, error)              { return nil, nil }
func (s *DomainService) Add(ctx context.Context, serviceID, hostname string) (any, error)     { return nil, nil }
func (s *DomainService) Remove(ctx context.Context, id string) error                          { return nil }

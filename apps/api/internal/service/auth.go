package service

import "context"

type AuthService struct{}

func NewAuthService() *AuthService {
	return &AuthService{}
}

func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	// TODO: Validate credentials, return session token
	return "", nil
}

func (s *AuthService) ValidateToken(ctx context.Context, token string) (string, error) {
	// TODO: Validate JWT/session, return user ID
	return "", nil
}

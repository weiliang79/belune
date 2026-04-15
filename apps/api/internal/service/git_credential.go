package service

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/pkg/crypto"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

type GitCredentialService struct {
	queries *generated.Queries
	keyring *crypto.Keyring
}

func NewGitCredentialService(queries *generated.Queries, keyring *crypto.Keyring) *GitCredentialService {
	return &GitCredentialService{queries: queries, keyring: keyring}
}

type CreateGitCredentialParams struct {
	Name      string
	Provider  string
	Token     string // plaintext, encrypted before storage
	Username  string
	CreatedBy pgtype.UUID
}

func (s *GitCredentialService) Create(ctx context.Context, p CreateGitCredentialParams) (generated.GitCredential, error) {
	encrypted, err := s.keyring.Encrypt([]byte(p.Token))
	if err != nil {
		return generated.GitCredential{}, fmt.Errorf("encrypt token: %w", err)
	}

	return s.queries.CreateGitCredential(ctx, generated.CreateGitCredentialParams{
		Name:           p.Name,
		Provider:       p.Provider,
		TokenEncrypted: encrypted,
		Username:       p.Username,
		CreatedBy:      p.CreatedBy,
	})
}

func (s *GitCredentialService) List(ctx context.Context) ([]generated.ListGitCredentialsRow, error) {
	return s.queries.ListGitCredentials(ctx)
}

func (s *GitCredentialService) Get(ctx context.Context, id pgtype.UUID) (generated.GitCredential, error) {
	return s.queries.GetGitCredential(ctx, id)
}

type UpdateGitCredentialParams struct {
	Name     string
	Provider string
	Token    string // plaintext; if empty, existing token is preserved
	Username string
}

func (s *GitCredentialService) Update(ctx context.Context, id pgtype.UUID, p UpdateGitCredentialParams) (generated.GitCredential, error) {
	current, err := s.queries.GetGitCredential(ctx, id)
	if err != nil {
		return generated.GitCredential{}, fmt.Errorf("get credential: %w", err)
	}

	tokenEncrypted := current.TokenEncrypted
	if p.Token != "" {
		tokenEncrypted, err = s.keyring.Encrypt([]byte(p.Token))
		if err != nil {
			return generated.GitCredential{}, fmt.Errorf("encrypt token: %w", err)
		}
	}

	return s.queries.UpdateGitCredential(ctx, generated.UpdateGitCredentialParams{
		ID:             id,
		Name:           p.Name,
		Provider:       p.Provider,
		TokenEncrypted: tokenEncrypted,
		Username:       p.Username,
	})
}

func (s *GitCredentialService) Delete(ctx context.Context, id pgtype.UUID) error {
	return s.queries.DeleteGitCredential(ctx, id)
}

// DecryptToken decrypts the token for a credential (used by deploy worker).
func (s *GitCredentialService) DecryptToken(cred generated.GitCredential) (string, error) {
	plaintext, err := s.keyring.Decrypt(cred.TokenEncrypted)
	if err != nil {
		return "", fmt.Errorf("decrypt token: %w", err)
	}
	return string(plaintext), nil
}

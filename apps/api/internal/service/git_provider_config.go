package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/pkg/crypto"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

// GitProviderConfigService manages the per-instance registered provider apps
// (a GitHub App or an OAuth client) used to connect user accounts. Secrets are
// stored as a keyring-encrypted JSON blob.
type GitProviderConfigService struct {
	queries *generated.Queries
	keyring *crypto.Keyring
}

func NewGitProviderConfigService(queries *generated.Queries, keyring *crypto.Keyring) *GitProviderConfigService {
	return &GitProviderConfigService{queries: queries, keyring: keyring}
}

// ProviderSecret is the provider-specific secret material. OAuth providers use
// ClientSecret only; a GitHub App also uses PrivateKey and WebhookSecret.
type ProviderSecret struct {
	ClientSecret  string `json:"client_secret,omitempty"`
	PrivateKey    string `json:"private_key,omitempty"`
	WebhookSecret string `json:"webhook_secret,omitempty"`
}

// SaveGitProviderConfigParams holds the fields for creating/updating a config.
// Secret is optional: nil preserves the existing encrypted secret.
type SaveGitProviderConfigParams struct {
	Provider string
	BaseURL  string
	ClientID string
	AppID    string
	AppSlug  string
	Secret   *ProviderSecret
}

// Save upserts a provider config keyed by (provider, base_url). When Secret is
// nil the existing encrypted secret is preserved.
func (s *GitProviderConfigService) Save(ctx context.Context, p SaveGitProviderConfigParams) (generated.GitProviderConfig, error) {
	var secretEncrypted []byte
	if p.Secret != nil {
		raw, err := json.Marshal(p.Secret)
		if err != nil {
			return generated.GitProviderConfig{}, fmt.Errorf("marshal provider secret: %w", err)
		}
		encrypted, err := s.keyring.Encrypt(raw)
		if err != nil {
			return generated.GitProviderConfig{}, fmt.Errorf("encrypt provider secret: %w", err)
		}
		secretEncrypted = encrypted
	}

	return s.queries.UpsertGitProviderConfig(ctx, generated.UpsertGitProviderConfigParams{
		Provider:        p.Provider,
		BaseUrl:         p.BaseURL,
		ClientID:        p.ClientID,
		AppID:           p.AppID,
		AppSlug:         p.AppSlug,
		SecretEncrypted: secretEncrypted,
	})
}

// List returns all provider configs without their secrets.
func (s *GitProviderConfigService) List(ctx context.Context) ([]generated.ListGitProviderConfigsRow, error) {
	return s.queries.ListGitProviderConfigs(ctx)
}

// Get returns a config and its decrypted secret for use by connect/token flows.
func (s *GitProviderConfigService) Get(ctx context.Context, provider, baseURL string) (generated.GitProviderConfig, ProviderSecret, error) {
	cfg, err := s.queries.GetGitProviderConfigByProvider(ctx, generated.GetGitProviderConfigByProviderParams{
		Provider: provider,
		BaseUrl:  baseURL,
	})
	if err != nil {
		return generated.GitProviderConfig{}, ProviderSecret{}, err
	}
	secret, err := s.decryptSecret(cfg.SecretEncrypted)
	if err != nil {
		return generated.GitProviderConfig{}, ProviderSecret{}, err
	}
	return cfg, secret, nil
}

func (s *GitProviderConfigService) decryptSecret(encrypted []byte) (ProviderSecret, error) {
	if len(encrypted) == 0 {
		return ProviderSecret{}, nil
	}
	raw, err := s.keyring.Decrypt(encrypted)
	if err != nil {
		return ProviderSecret{}, fmt.Errorf("decrypt provider secret: %w", err)
	}
	var secret ProviderSecret
	if err := json.Unmarshal(raw, &secret); err != nil {
		return ProviderSecret{}, fmt.Errorf("unmarshal provider secret: %w", err)
	}
	return secret, nil
}

// Delete removes a provider config by id.
func (s *GitProviderConfigService) Delete(ctx context.Context, id pgtype.UUID) error {
	return s.queries.DeleteGitProviderConfig(ctx, id)
}

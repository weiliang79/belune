package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ungweiliang/selfhost-paas/internal/pkg/crypto"
)

type Config struct {
	Port           int
	DatabaseURL    string
	RedisURL       string
	JWTSecret      string
	JWTExpiryHours int
	// EncryptionKey is the legacy single-KEK env var (ENCRYPTION_KEY). Retained
	// for backward compatibility with call sites that have not yet migrated to
	// the keyring. New code should use Keyring instead.
	EncryptionKey        string
	EncryptionKeys       string // ENCRYPTION_KEYS — "v1:hex64,v2:hex64,..."
	EncryptionKeyCurrent string // ENCRYPTION_KEY_CURRENT — "v2" (optional override)
	Keyring              *crypto.Keyring
	CaddyAdminURL        string
	AccessLogPath        string
	CORSOrigins          []string
	SecureCookies        bool
	TLS                  bool   // when true, send HSTS headers
	LogLevel             string // debug, info, warn, error (default: info)
	DisableRateLimiting  bool   // set true in tests to avoid per-IP counter accumulation

	// Timeouts
	BuildTimeoutMinutes     int // max duration for build operations (default 30)
	TaskTimeoutMinutes      int // max duration for asynq tasks (default 45)
	ImagePullTimeoutMinutes int // max duration for image pull operations (default 10)

	// Resource limits
	MaxTerminalSessionsPerUser int // per-user cap on concurrent terminal sessions (default 5)
	MaxWebSocketConnsPerUser   int // per-user cap on concurrent WebSocket connections (default 20)
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:                 getEnvInt("PORT", 8080),
		DatabaseURL:          getEnv("DATABASE_URL", "postgres://paas:paas@localhost:5432/paas?sslmode=disable"),
		RedisURL:             getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:            getEnv("JWT_SECRET", ""),
		JWTExpiryHours:       getEnvInt("JWT_EXPIRY_HOURS", 24),
		EncryptionKey:        getEnv("ENCRYPTION_KEY", ""),
		EncryptionKeys:       getEnv("ENCRYPTION_KEYS", ""),
		EncryptionKeyCurrent: getEnv("ENCRYPTION_KEY_CURRENT", ""),
		CaddyAdminURL:        getEnv("CADDY_ADMIN_URL", "http://localhost:2019"),
		AccessLogPath:        getEnv("ACCESS_LOG_PATH", "../../infra/caddy/logs/access.log"),
		CORSOrigins:          getEnvList("CORS_ORIGINS", []string{"http://localhost:5173"}),
		SecureCookies:        getEnvBool("SECURE_COOKIES", false),
		TLS:                  getEnvBool("TLS_ENABLED", false),
		LogLevel:             getEnv("LOG_LEVEL", "info"),

		BuildTimeoutMinutes:     getEnvInt("BUILD_TIMEOUT_MINUTES", 30),
		TaskTimeoutMinutes:      getEnvInt("TASK_TIMEOUT_MINUTES", 45),
		ImagePullTimeoutMinutes: getEnvInt("IMAGE_PULL_TIMEOUT_MINUTES", 10),

		MaxTerminalSessionsPerUser: getEnvInt("MAX_TERMINAL_SESSIONS_PER_USER", 5),
		MaxWebSocketConnsPerUser:   getEnvInt("MAX_WEBSOCKET_CONNS_PER_USER", 20),
	}

	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	// Keyring accepts either ENCRYPTION_KEYS (multi-key) or the legacy
	// ENCRYPTION_KEY (single key, promoted to v1). At least one must be set.
	keyring, err := crypto.ParseKeyringEnv(cfg.EncryptionKeys, cfg.EncryptionKey, cfg.EncryptionKeyCurrent)
	if err != nil {
		return nil, err
	}
	cfg.Keyring = keyring

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvList(key string, fallback []string) []string {
	if val, ok := os.LookupEnv(key); ok {
		parts := strings.Split(val, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return fallback
}

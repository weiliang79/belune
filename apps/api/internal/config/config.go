package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port                int
	DatabaseURL         string
	RedisURL            string
	JWTSecret           string
	JWTExpiryHours      int
	EncryptionKey       string
	CaddyAdminURL       string
	AccessLogPath       string
	CORSOrigins         []string
	SecureCookies       bool
	TLS                 bool   // when true, send HSTS headers
	LogLevel            string // debug, info, warn, error (default: info)
	DisableRateLimiting bool   // set true in tests to avoid per-IP counter accumulation

	// Timeouts
	BuildTimeoutMinutes     int // max duration for build operations (default 30)
	TaskTimeoutMinutes      int // max duration for asynq tasks (default 45)
	ImagePullTimeoutMinutes int // max duration for image pull operations (default 10)
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:           getEnvInt("PORT", 8080),
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://paas:paas@localhost:5432/paas?sslmode=disable"),
		RedisURL:       getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:      getEnv("JWT_SECRET", ""),
		JWTExpiryHours: getEnvInt("JWT_EXPIRY_HOURS", 24),
		EncryptionKey:  getEnv("ENCRYPTION_KEY", ""),
		CaddyAdminURL:  getEnv("CADDY_ADMIN_URL", "http://localhost:2019"),
		AccessLogPath:  getEnv("ACCESS_LOG_PATH", "/var/log/caddy/access.log"),
		CORSOrigins:    getEnvList("CORS_ORIGINS", []string{"http://localhost:5173"}),
		SecureCookies:  getEnvBool("SECURE_COOKIES", false),
		TLS:      getEnvBool("TLS_ENABLED", false),
		LogLevel: getEnv("LOG_LEVEL", "info"),

		BuildTimeoutMinutes:     getEnvInt("BUILD_TIMEOUT_MINUTES", 30),
		TaskTimeoutMinutes:      getEnvInt("TASK_TIMEOUT_MINUTES", 45),
		ImagePullTimeoutMinutes: getEnvInt("IMAGE_PULL_TIMEOUT_MINUTES", 10),
	}

	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	if cfg.EncryptionKey == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY is required")
	}
	if len(cfg.EncryptionKey) != 64 || !isHex(cfg.EncryptionKey) {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be exactly 64 hex characters")
	}

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

func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func getEnvBool(key string, fallback bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return fallback
}

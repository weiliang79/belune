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
	EncryptionKey       string
	CaddyAdminURL       string
	CORSOrigins         []string
	SecureCookies       bool
	DisableRateLimiting bool // set true in tests to avoid per-IP counter accumulation
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:           getEnvInt("PORT", 8080),
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://paas:paas@localhost:5432/paas?sslmode=disable"),
		RedisURL:       getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:      getEnv("JWT_SECRET", ""),
		EncryptionKey:  getEnv("ENCRYPTION_KEY", ""),
		CaddyAdminURL:  getEnv("CADDY_ADMIN_URL", "http://localhost:2019"),
		CORSOrigins:    getEnvList("CORS_ORIGINS", []string{"http://localhost:5173"}),
		SecureCookies:  getEnvBool("SECURE_COOKIES", false),
	}

	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if cfg.EncryptionKey == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY is required")
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

func getEnvBool(key string, fallback bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return fallback
}

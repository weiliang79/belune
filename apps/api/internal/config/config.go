package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ungweiliang/selfhost-paas/internal/pkg/crypto"
)

type Config struct {
	Port                int
	DatabaseURL         string
	RedisURL            string
	JWTSecret           string
	JWTExpiryHours      int // access-token TTL in hours (legacy name; default 1)
	JWTRefreshHours     int // refresh-token TTL in hours (default 168 = 7 days)
	Keyring             *crypto.Keyring
	CaddyAdminURL       string
	CaddyContainerName  string // Docker container name/ID for Caddy; used to attach it to per-project networks
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

	// Resource limits
	MaxTerminalSessionsPerUser int // per-user cap on concurrent terminal sessions (default 5)
	MaxWebSocketConnsPerUser   int // per-user cap on concurrent WebSocket connections (default 20)

	// Preview environments
	PreviewIdleDays int // days after which idle preview apps are garbage-collected (default 7; 0 disables)

	// SkipMigrations is the runtime kill-switch for auto-migration. Set
	// PAAS_SKIP_MIGRATIONS=true to bring the API up without running pending
	// migrations — useful when an in-progress migration left the schema in a
	// state the operator wants to repair manually before letting the next
	// version of the binary touch it.
	SkipMigrations bool

	// Tracing
	OTLPEndpoint string // OTEL_EXPORTER_OTLP_ENDPOINT; empty = no-op tracer
	OTLPInsecure bool   // OTEL_EXPORTER_OTLP_INSECURE; default true (loopback collectors)

	// Metrics
	// MetricsBind, when non-empty, starts a separate HTTP listener that serves
	// /metrics without auth (Prometheus-friendly). Typically "127.0.0.1:9090".
	// When empty, /metrics is mounted on the main router behind admin auth.
	MetricsBind string

	// Public base URL — required when SMTP is configured; used to construct
	// links in outbound emails (password reset, invitations).
	PublicBaseURL string

	// ServerSSHHost / ServerSSHUser are presentation-only hints shown in the
	// database External Access card to build the SSH-tunnel command (ssh -L).
	// When empty the UI falls back to a generic placeholder.
	ServerSSHHost string
	ServerSSHUser string

	// SMTP configuration for outbound email. All fields are optional; when
	// SMTPHost is empty the email service writes to slog instead of dialing.
	SMTPHost      string
	SMTPPort      int // default 587
	SMTPUser      string
	SMTPPassword  string
	SMTPFromEmail string
	SMTPFromName  string // default "Self-Hosted PaaS"
	SMTPTLSMode   string // none | starttls | tls (default: starttls)

	// Backup remote storage. When BackupRemoteEnabled is false all BACKUP_S3_*
	// fields are ignored and backup.sh writes local archives only.
	BackupRemoteEnabled bool
	BackupS3Endpoint    string // empty = AWS; set for MinIO/B2/R2/Wasabi
	BackupS3Region      string // default "us-east-1"
	BackupS3Bucket      string
	BackupS3AccessKey   string
	BackupS3SecretKey   string
	BackupS3Prefix      string // key prefix inside the bucket (default "paas/")
	BackupS3UseSSL      bool   // default true
	BackupRetainDays    int    // delete objects older than N days (default 30)
	BackupRetainCount   int    // always keep the N most-recent objects (default 14)
	// Path to backup.sh reachable from the API process. Defaults to
	// $PAAS_DIR/scripts/backup.sh (falls back to /opt/paas/scripts/backup.sh).
	BackupScriptPath string
	// Local directory where managed-database logical dumps are written before
	// (optional) upload to S3. Defaults to $PAAS_DIR/backups/databases.
	DatabaseBackupDir string
	// Image for the short-lived helper that tars/untars a database volume during
	// "other"-type volume-snapshot backup/restore. Must contain `tar` and `sh`.
	DatabaseBackupHelperImage string
	// Number of most-recent backups to keep per managed database; older ones are
	// pruned (rows + local files + S3 objects) after a new backup succeeds.
	DatabaseBackupRetainCount int
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:               getEnvInt("PORT", 8080),
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://paas:paas@localhost:5432/paas?sslmode=disable"),
		RedisURL:           getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:          getEnv("JWT_SECRET", ""),
		JWTExpiryHours:     getEnvInt("JWT_EXPIRY_HOURS", 1),
		JWTRefreshHours:    getEnvInt("JWT_REFRESH_HOURS", 24*7),
		CaddyAdminURL:      getEnv("CADDY_ADMIN_URL", "http://localhost:2019"),
		CaddyContainerName: getEnv("CADDY_CONTAINER_NAME", "infra-caddy-1"),
		AccessLogPath:      getEnv("ACCESS_LOG_PATH", "../../infra/caddy/logs/access.log"),
		CORSOrigins:        getEnvList("CORS_ORIGINS", []string{"http://localhost:5173"}),
		SecureCookies:      getEnvBool("SECURE_COOKIES", false),
		TLS:                getEnvBool("TLS_ENABLED", false),
		LogLevel:           getEnv("LOG_LEVEL", "info"),

		BuildTimeoutMinutes:     getEnvInt("BUILD_TIMEOUT_MINUTES", 30),
		TaskTimeoutMinutes:      getEnvInt("TASK_TIMEOUT_MINUTES", 45),
		ImagePullTimeoutMinutes: getEnvInt("IMAGE_PULL_TIMEOUT_MINUTES", 10),

		MaxTerminalSessionsPerUser: getEnvInt("MAX_TERMINAL_SESSIONS_PER_USER", 5),
		MaxWebSocketConnsPerUser:   getEnvInt("MAX_WEBSOCKET_CONNS_PER_USER", 20),

		PreviewIdleDays: getEnvInt("PREVIEW_IDLE_DAYS", 7),

		SkipMigrations: getEnvBool("PAAS_SKIP_MIGRATIONS", false),

		OTLPEndpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		OTLPInsecure: getEnvBool("OTEL_EXPORTER_OTLP_INSECURE", true),

		MetricsBind: getEnv("METRICS_BIND", ""),

		PublicBaseURL: getEnv("PUBLIC_BASE_URL", ""),

		ServerSSHHost: getEnv("SERVER_SSH_HOST", ""),
		ServerSSHUser: getEnv("SERVER_SSH_USER", ""),

		SMTPHost:      getEnv("SMTP_HOST", ""),
		SMTPPort:      getEnvInt("SMTP_PORT", 587),
		SMTPUser:      getEnv("SMTP_USER", ""),
		SMTPPassword:  getEnv("SMTP_PASSWORD", ""),
		SMTPFromEmail: getEnv("SMTP_FROM_EMAIL", ""),
		SMTPFromName:  getEnv("SMTP_FROM_NAME", "Self-Hosted PaaS"),
		SMTPTLSMode:   getEnv("SMTP_TLS_MODE", "starttls"),

		BackupRemoteEnabled:       getEnvBool("BACKUP_REMOTE_ENABLED", false),
		BackupS3Endpoint:          getEnv("BACKUP_S3_ENDPOINT", ""),
		BackupS3Region:            getEnv("BACKUP_S3_REGION", "us-east-1"),
		BackupS3Bucket:            getEnv("BACKUP_S3_BUCKET", ""),
		BackupS3AccessKey:         getEnv("BACKUP_S3_ACCESS_KEY", ""),
		BackupS3SecretKey:         getEnv("BACKUP_S3_SECRET_KEY", ""),
		BackupS3Prefix:            getEnv("BACKUP_S3_PREFIX", "paas/"),
		BackupS3UseSSL:            getEnvBool("BACKUP_S3_USE_SSL", true),
		BackupRetainDays:          getEnvInt("BACKUP_RETAIN_DAYS", 30),
		BackupRetainCount:         getEnvInt("BACKUP_RETAIN_COUNT", 14),
		BackupScriptPath:          getEnv("BACKUP_SCRIPT_PATH", paasDir()+"/scripts/backup.sh"),
		DatabaseBackupDir:         getEnv("DATABASE_BACKUP_DIR", paasDir()+"/backups/databases"),
		DatabaseBackupHelperImage: getEnv("DATABASE_BACKUP_HELPER_IMAGE", "alpine:3.20"),
		DatabaseBackupRetainCount: getEnvInt("DATABASE_BACKUP_RETAIN_COUNT", 7),
	}

	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	// Keyring accepts either ENCRYPTION_KEYS (multi-key) or the legacy
	// ENCRYPTION_KEY (single key, promoted to v1). At least one must be set.
	keyring, err := crypto.ParseKeyringEnv(
		getEnv("ENCRYPTION_KEYS", ""),
		getEnv("ENCRYPTION_KEY", ""),
		getEnv("ENCRYPTION_KEY_CURRENT", ""),
	)
	if err != nil {
		return nil, err
	}
	cfg.Keyring = keyring

	return cfg, nil
}

// paasDir returns the PaaS install directory, used to derive default paths.
func paasDir() string {
	if dir := os.Getenv("PAAS_DIR"); dir != "" {
		return dir
	}
	return "/opt/paas"
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

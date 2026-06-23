package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"

	"github.com/ungweiliang/selfhost-paas/internal/handler"
	"github.com/ungweiliang/selfhost-paas/internal/pkg/metrics"
	"github.com/ungweiliang/selfhost-paas/internal/server/middleware"
	"github.com/ungweiliang/selfhost-paas/internal/service"
)

const (
	defaultBodyLimit = 1 << 20 // 1 MB — applied to all non-streaming routes
	envBodyLimit     = 5 << 20 // 5 MB — raised for bulk env-var imports
	handlerTimeout   = 15 * time.Second
)

// keyByUserIDOrIP keys rate limiting by authenticated user ID, falling back to
// IP address for unauthenticated requests. This ensures per-user limits rather
// than per-IP limits on protected routes.
func keyByUserIDOrIP(r *http.Request) (string, error) {
	if userID := middleware.UserIDFromContext(r.Context()); userID != "" {
		return "user:" + userID, nil
	}
	return httprate.KeyByIP(r)
}

// withTimeout wraps a handler in an http.TimeoutHandler so slow-loris attacks
// cannot hold non-streaming connections open indefinitely.
func withTimeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, d, `{"error":"request timeout"}`)
	}
}

func registerRoutes(r chi.Router, h *handler.Handler, auth *service.AuthService, disableRateLimit bool) {
	// Health check (unauthenticated; no body limit needed, no timeout applied so
	// health-check pollers with long intervals are not artificially rejected).
	r.Get("/healthz", h.HealthCheck)

	// Public routes — small body limit; no timeout on login (rate-limited instead)
	r.Group(func(r chi.Router) {
		r.Use(middleware.BodyLimit(defaultBodyLimit))

		// Login rate limit: 5 req/min per IP
		r.Group(func(r chi.Router) {
			if !disableRateLimit {
				r.Use(httprate.LimitByIP(5, time.Minute))
			}
			r.With(withTimeout(handlerTimeout)).Post("/api/auth/login", h.Login)
		})

		r.With(withTimeout(handlerTimeout)).Get("/api/auth/setup", h.Setup)
		r.With(withTimeout(handlerTimeout)).Post("/api/auth/setup", h.Setup)
		r.With(withTimeout(handlerTimeout)).Get("/api/features", h.GetFeatures)

		// Refresh: cookie-driven, no Auth middleware — but CSRF and rate
		// limit still apply. 30 req/min per IP is generous enough for normal
		// SPA usage (one refresh per access expiry) and tight enough that a
		// stolen refresh cookie cannot be brute-rotated against.
		r.Group(func(r chi.Router) {
			r.Use(middleware.CSRF())
			if !disableRateLimit {
				r.Use(httprate.LimitByIP(30, time.Minute))
			}
			r.With(withTimeout(handlerTimeout)).Post("/api/auth/refresh", h.Refresh)
		})

		// Password reset: forgot-password 3/hour by IP; reset-password 10/min by IP.
		r.Group(func(r chi.Router) {
			if !disableRateLimit {
				r.Use(httprate.LimitByIP(3, time.Hour))
			}
			r.With(withTimeout(handlerTimeout)).Post("/api/auth/forgot-password", h.ForgotPassword)
		})
		r.Group(func(r chi.Router) {
			if !disableRateLimit {
				r.Use(httprate.LimitByIP(10, time.Minute))
			}
			r.With(withTimeout(handlerTimeout)).Post("/api/auth/reset-password", h.ResetPassword)
		})

		// Invitation acceptance: peek 30/min by IP; accept 10/min by IP.
		r.Group(func(r chi.Router) {
			if !disableRateLimit {
				r.Use(httprate.LimitByIP(30, time.Minute))
			}
			r.With(withTimeout(handlerTimeout)).Get("/api/auth/invitation", h.GetInvitation)
		})
		r.Group(func(r chi.Router) {
			if !disableRateLimit {
				r.Use(httprate.LimitByIP(10, time.Minute))
			}
			r.With(withTimeout(handlerTimeout)).Post("/api/auth/accept-invitation", h.AcceptInvitation)
		})

		// Webhooks: 30 req/min per IP
		r.Group(func(r chi.Router) {
			if !disableRateLimit {
				r.Use(httprate.LimitByIP(30, time.Minute))
			}
			r.With(withTimeout(handlerTimeout)).Post("/api/webhooks/push", h.HandleWebhookPush)
		})
	})

	// WebSocket routes: auth-protected, no body limit, no timeout (long-lived connections).
	// httprate must NOT be applied here — it wraps ResponseWriter in a way that
	// breaks Hijacker and prevents the protocol upgrade.
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(auth))
		r.Get("/api/ws", h.HandleWebSocket)
		r.Get("/api/ws/terminal/{sessionId}", h.HandleTerminalWebSocket)
	})

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(auth))
		r.Use(middleware.CSRF())
		if !disableRateLimit {
			r.Use(httprate.Limit(100, time.Minute, httprate.WithKeyFuncs(keyByUserIDOrIP)))
		}

		// Standard JSON routes: 1 MB body limit + 15 s timeout.
		r.Group(func(r chi.Router) {
			r.Use(middleware.BodyLimit(defaultBodyLimit))
			r.Use(withTimeout(handlerTimeout))

			r.Post("/api/auth/logout", h.Logout)
			r.Get("/api/auth/me", h.Me)
			r.Put("/api/auth/password", h.ChangeOwnPassword)
			r.Put("/api/auth/profile", h.UpdateProfile)

			r.Get("/api/account/alert-preferences", h.GetAlertPreferences)
			r.Put("/api/account/alert-preferences", h.UpdateAlertPreferences)

			// Admin-only routes
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRole("admin"))

				r.Get("/api/users", h.ListUsers)
				r.Put("/api/users/{userId}/role", h.UpdateUserRole)
				r.Delete("/api/users/{userId}", h.DeleteUser)

				r.Group(func(r chi.Router) {
					if !disableRateLimit {
						r.Use(httprate.LimitByIP(10, time.Minute))
					}
					r.Post("/api/users", h.CreateUser)
					r.Post("/api/users/invite", h.InviteUser)
					r.Put("/api/users/{userId}/password", h.ResetUserPassword)
				})
				r.Get("/api/users/invitations", h.ListPendingInvitations)
				r.Delete("/api/users/invitations/{invitationId}", h.RevokeInvitation)

				// Backup management
				r.Get("/api/backups", h.ListBackupRuns)
				r.Get("/api/backups/status", h.GetBackupStatus)
				r.Post("/api/backups/run", h.TriggerBackupRun)
			})

			// Git credentials
			r.Get("/api/git-credentials", h.ListGitCredentials)
			r.Post("/api/git-credentials", h.CreateGitCredential)
			r.Put("/api/git-credentials/{credentialId}", h.UpdateGitCredential)
			r.Delete("/api/git-credentials/{credentialId}", h.DeleteGitCredential)

			// Projects
			r.Get("/api/projects", h.ListProjects)
			r.Post("/api/projects", h.CreateProject)
			r.Get("/api/projects/{projectId}", h.GetProject)
			r.Put("/api/projects/{projectId}", h.UpdateProject)
			r.Delete("/api/projects/{projectId}", h.DeleteProject)
			r.Put("/api/projects/{projectId}/transfer", h.TransferProject)

			// Project runtime metrics snapshot (per-service CPU/mem/uptime/domain)
			r.Get("/api/projects/{projectId}/metrics", h.GetProjectMetrics)

			// Applications
			r.Get("/api/projects/{projectId}/applications", h.ListApplications)
			r.Post("/api/projects/{projectId}/applications", h.CreateApplication)
			r.Get("/api/projects/{projectId}/applications/{applicationId}", h.GetApplication)
			r.Put("/api/projects/{projectId}/applications/{applicationId}", h.UpdateApplication)
			r.Delete("/api/projects/{projectId}/applications/{applicationId}", h.DeleteApplication)
			r.Put("/api/projects/{projectId}/applications/{applicationId}/webhook", h.UpdateApplicationWebhook)
			r.Post("/api/projects/{projectId}/applications/{applicationId}/deploy", h.DeployApplication)
			r.Post("/api/projects/{projectId}/applications/{applicationId}/stop", h.StopApplication)
			r.Post("/api/projects/{projectId}/applications/{applicationId}/start", h.StartApplication)
			r.Post("/api/projects/{projectId}/applications/{applicationId}/restart", h.RestartApplication)
			r.Post("/api/projects/{projectId}/applications/{applicationId}/build", h.BuildApplication)
			r.Post("/api/projects/{projectId}/applications/{applicationId}/rollback", h.RollbackDeployment)
			r.Get("/api/projects/{projectId}/applications/{applicationId}/cache", h.GetBuildCache)
			r.Delete("/api/projects/{projectId}/applications/{applicationId}/cache", h.ClearBuildCache)

			// Preview environments: parent config + child list + child delete
			r.Put("/api/projects/{projectId}/applications/{applicationId}/previews/config", h.UpdatePreviewConfig)
			r.Get("/api/projects/{projectId}/applications/{applicationId}/previews", h.ListPreviews)
			r.Delete("/api/projects/{projectId}/applications/{applicationId}/previews/{previewId}", h.DeletePreview)

			// Deployments
			r.Get("/api/projects/{projectId}/applications/{applicationId}/deployments", h.ListDeployments)
			r.Get("/api/projects/{projectId}/applications/{applicationId}/deployments/{deploymentId}", h.GetDeployment)

			// Latest post-deploy health-probe result
			r.Get("/api/projects/{projectId}/applications/{applicationId}/health", h.GetApplicationHealth)

			// Terminal session creation (exec is short; websocket tunnel is in the WS group)
			r.Post("/api/projects/{projectId}/applications/{applicationId}/terminal", h.CreateTerminalSession)

			// Application logs history (paginated query, not a stream)
			r.Get("/api/projects/{projectId}/applications/{applicationId}/logs/history", h.ListApplicationLogs)

			// Request logs (paginated)
			r.Get("/api/projects/{projectId}/applications/{applicationId}/requests", h.ListRequestLogs)

			// Domains
			r.Get("/api/projects/{projectId}/applications/{applicationId}/domains", h.ListDomains)
			r.Post("/api/projects/{projectId}/applications/{applicationId}/domains", h.AddDomain)
			r.Put("/api/projects/{projectId}/applications/{applicationId}/domains/{domainId}", h.UpdateDomain)
			r.Delete("/api/projects/{projectId}/applications/{applicationId}/domains/{domainId}", h.RemoveDomain)

			// Domain route features
			r.Get("/api/projects/{projectId}/applications/{applicationId}/domains/{domainId}/features", h.ListRouteFeatures)
			r.Put("/api/projects/{projectId}/applications/{applicationId}/domains/{domainId}/features", h.UpsertRouteFeature)
			r.Delete("/api/projects/{projectId}/applications/{applicationId}/domains/{domainId}/features/{featureId}", h.DeleteRouteFeature)

			// Global deployments
			r.Get("/api/deployments", h.GetGlobalDeployments)

			// Operator-health stat strip (member-scoped; admins see host + backups)
			r.Get("/api/stats", h.GetStats)

			// Notifications — per-user feed (recipient is the current user).
			r.Get("/api/notifications", h.ListNotifications)
			r.Get("/api/notifications/unread-count", h.UnreadNotificationCount)
			r.Post("/api/notifications/{notificationId}/read", h.MarkNotificationRead)
			r.Post("/api/notifications/read-all", h.MarkAllNotificationsRead)

			// Admin-only: metrics snapshots, settings, cleanup, audit
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRole("admin"))
				r.Get("/api/metrics", h.GetMetrics)
				r.Get("/api/metrics/host", h.GetHostHistoricalMetrics)
				r.Post("/api/cleanup", h.TriggerCleanup)
				r.Get("/api/settings", h.ListSettings)
				r.Put("/api/settings", h.UpdateSettings)
				r.Get("/api/requests", h.ListAllRequestLogs)
				r.Get("/api/requests/summary", h.GetAllRequestsSummary)
				r.Get("/api/server/services", h.GetServerServices)
				r.Get("/api/audit-logs", h.ListAuditLogs)
				r.Get("/api/audit-logs/actions", h.ListAuditActions)
				r.Get("/api/audit-logs/export", h.ExportAuditLogs)
				r.Get("/api/proxy/reconciler", h.GetProxyReconcilerStatus)
				r.Get("/api/quotas", h.ListQuotas)
				r.Get("/api/quotas/{scope}/{scopeId}", h.GetQuota)
				r.Put("/api/quotas/{scope}/{scopeId}", h.UpsertQuota)
				r.Delete("/api/quotas/{scope}/{scopeId}", h.DeleteQuota)
				// Prometheus scrape endpoint. When METRICS_BIND is configured
				// the metrics are also exposed anonymously on that listener;
				// this admin-gated copy is for operators browsing via the UI.
				r.Method("GET", "/metrics", metrics.Handler())
			})
		})

		// Env routes: 5 MB body limit + 15 s timeout (bulk import may exceed 1 MB).
		r.Group(func(r chi.Router) {
			r.Use(middleware.BodyLimit(envBodyLimit))
			r.Use(withTimeout(handlerTimeout))
			r.Get("/api/projects/{projectId}/env", h.ListProjectEnvVars)
			r.Put("/api/projects/{projectId}/env", h.UpdateProjectEnvVars)
			r.Get("/api/projects/{projectId}/applications/{applicationId}/env", h.ListEnvVars)
			r.Put("/api/projects/{projectId}/applications/{applicationId}/env", h.UpdateEnvVars)
		})

		// Databases: standard limit + timeout
		r.Group(func(r chi.Router) {
			r.Use(middleware.BodyLimit(defaultBodyLimit))
			r.Use(withTimeout(handlerTimeout))
			r.Get("/api/projects/{projectId}/databases", h.ListDatabases)
			r.Post("/api/projects/{projectId}/databases", h.CreateDatabase)
			r.Get("/api/projects/{projectId}/databases/{databaseId}", h.GetDatabase)
			r.Put("/api/projects/{projectId}/databases/{databaseId}", h.UpdateDatabase)
			r.Post("/api/projects/{projectId}/databases/{databaseId}/external-access", h.SetDatabaseExternalAccess)
			r.Get("/api/projects/{projectId}/databases/{databaseId}/backups", h.ListDatabaseBackups)
			r.Post("/api/projects/{projectId}/databases/{databaseId}/backups", h.BackupDatabase)
			r.Delete("/api/projects/{projectId}/databases/{databaseId}/backups/{backupId}", h.DeleteDatabaseBackup)
			r.Post("/api/projects/{projectId}/databases/{databaseId}/restore", h.RestoreDatabase)
			r.Post("/api/projects/{projectId}/databases/{databaseId}/upgrade", h.UpgradeDatabase)
			r.Delete("/api/projects/{projectId}/databases/{databaseId}", h.DeleteDatabase)
		})

		// Streaming routes: SSE / long-poll — no timeout, no body limit.
		r.Get("/api/projects/{projectId}/applications/{applicationId}/deployments/{deploymentId}/build-logs", h.StreamBuildLogs)
		r.Get("/api/projects/{projectId}/applications/{applicationId}/logs", h.StreamLogs)
		r.Get("/api/projects/{projectId}/applications/{applicationId}/requests/stream", h.StreamRequestLogs)
		r.Get("/api/projects/{projectId}/applications/{applicationId}/metrics/stream", h.StreamApplicationMetrics)
		r.Get("/api/notifications/stream", h.StreamNotifications)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole("admin"))
			r.Get("/api/metrics/host/stream", h.StreamHostMetrics)
			r.Get("/api/requests/stream", h.StreamAllRequestLogs)
		})
	})
}

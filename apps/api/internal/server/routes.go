package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"

	"github.com/weiliang79/belune/internal/handler"
	"github.com/weiliang79/belune/internal/pkg/metrics"
	"github.com/weiliang79/belune/internal/server/middleware"
	"github.com/weiliang79/belune/internal/service"
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
		r.With(withTimeout(handlerTimeout)).Get("/api/version", h.GetVersion)

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
			// Provider App/OAuth webhooks (verified against the provider app's
			// shared webhook secret).
			r.With(withTimeout(handlerTimeout)).Post("/api/git/webhooks/{provider}", h.HandleProviderWebhook)
			// Per-application deploy hook. Unauthenticated by design: the token
			// in the path is the credential, so CI can fire it with a bare curl.
			r.With(withTimeout(handlerTimeout)).Post("/api/webhooks/deploy/{token}", h.HandleDeployHook)
		})

		// Git provider OAuth/manifest callbacks are public: they are top-level
		// browser redirects from the provider that carry no Authorization header,
		// so they are guarded by a one-time state nonce instead of the JWT.
		r.Group(func(r chi.Router) {
			r.Use(withTimeout(handlerTimeout))
			r.Get("/api/git/providers/github/manifest/callback", h.HandleGitHubAppManifestCallback)
			r.Get("/api/git/integrations/callback", h.HandleGitIntegrationCallback)
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
				r.Post("/api/backups/test", h.TestBackupRemote)
				r.Put("/api/backups/remote", h.UpdateBackupRemote)
			})

			// Git connections (per-user connected provider accounts)
			r.Get("/api/git/integrations", h.ListGitIntegrations)
			r.Get("/api/git/integrations/available", h.ListAvailableProviders)
			r.Get("/api/git/integrations/connect", h.StartGitIntegrationConnect)
			r.Get("/api/git/integrations/{integrationId}/repos", h.ListIntegrationRepos)
			r.Get("/api/git/integrations/{integrationId}/branches", h.ListIntegrationBranches)
			r.Delete("/api/git/integrations/{integrationId}", h.DeleteGitIntegration)

			// App templates (catalog + one-click instantiation)
			r.Get("/api/templates", h.ListTemplates)
			r.Get("/api/templates/{templateId}", h.GetTemplate)
			r.Post("/api/templates/{templateId}/instantiate", h.InstantiateTemplate)

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
			r.Put("/api/projects/{projectId}/applications/{applicationId}/runtime", h.UpdateApplicationRuntime)
			r.Delete("/api/projects/{projectId}/applications/{applicationId}", h.DeleteApplication)
			r.Put("/api/projects/{projectId}/applications/{applicationId}/webhook", h.UpdateApplicationWebhook)
			r.Get("/api/projects/{projectId}/applications/{applicationId}/webhook/reveal", h.RevealWebhookSecret)
			r.Get("/api/projects/{projectId}/applications/{applicationId}/deploy-hook", h.GetDeployHook)
			r.Get("/api/projects/{projectId}/applications/{applicationId}/deploy-hook/reveal", h.RevealDeployHook)
			r.Post("/api/projects/{projectId}/applications/{applicationId}/deploy-hook", h.GenerateDeployHook)
			r.Delete("/api/projects/{projectId}/applications/{applicationId}/deploy-hook", h.DeleteDeployHook)
			r.Post("/api/projects/{projectId}/applications/{applicationId}/deploy", h.DeployApplication)
			r.Post("/api/projects/{projectId}/applications/{applicationId}/stop", h.StopApplication)
			r.Post("/api/projects/{projectId}/applications/{applicationId}/start", h.StartApplication)
			r.Post("/api/projects/{projectId}/applications/{applicationId}/restart", h.RestartApplication)
			r.Post("/api/projects/{projectId}/applications/{applicationId}/reload", h.ReloadApplication)
			r.Post("/api/projects/{projectId}/applications/{applicationId}/rebuild", h.RebuildApplication)
			r.Post("/api/projects/{projectId}/applications/{applicationId}/build", h.BuildApplication)
			r.Post("/api/projects/{projectId}/applications/{applicationId}/change-source", h.ChangeApplicationSource)
			r.Put("/api/projects/{projectId}/applications/{applicationId}/health-check", h.SetHealthCheck)
			r.Put("/api/projects/{projectId}/applications/{applicationId}/resources", h.SetResources)
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
			r.Get("/api/projects/{projectId}/applications/{applicationId}/logs/sessions", h.ListApplicationLogSessions)

			// Request logs (paginated)
			r.Get("/api/projects/{projectId}/applications/{applicationId}/requests", h.ListRequestLogs)

			// Domains
			r.Get("/api/projects/{projectId}/applications/{applicationId}/domains", h.ListDomains)
			r.Post("/api/projects/{projectId}/applications/{applicationId}/domains", h.AddDomain)
			r.Put("/api/projects/{projectId}/applications/{applicationId}/domains/{domainId}", h.UpdateDomain)
			r.Delete("/api/projects/{projectId}/applications/{applicationId}/domains/{domainId}", h.RemoveDomain)

			// Domain route features
			r.Post("/api/projects/{projectId}/applications/{applicationId}/domains/{domainId}/tls/recheck", h.RecheckDomainTLS)
			r.Get("/api/projects/{projectId}/applications/{applicationId}/domains/{domainId}/features", h.ListRouteFeatures)
			r.Put("/api/projects/{projectId}/applications/{applicationId}/domains/{domainId}/features", h.UpsertRouteFeature)
			r.Delete("/api/projects/{projectId}/applications/{applicationId}/domains/{domainId}/features/{featureId}", h.DeleteRouteFeature)

			// Application persistent volumes
			r.Get("/api/projects/{projectId}/applications/{applicationId}/volumes", h.ListApplicationVolumes)
			r.Post("/api/projects/{projectId}/applications/{applicationId}/volumes", h.CreateApplicationVolume)
			r.Delete("/api/projects/{projectId}/applications/{applicationId}/volumes/{volumeId}", h.DeleteApplicationVolume)

			// Application volume backups
			r.Get("/api/projects/{projectId}/applications/{applicationId}/volumes/{volumeId}/backup-configs", h.ListVolumeBackupConfigs)
			r.Post("/api/projects/{projectId}/applications/{applicationId}/volumes/{volumeId}/backup-configs", h.CreateVolumeBackupConfig)
			r.Put("/api/projects/{projectId}/applications/{applicationId}/volumes/{volumeId}/backup-configs/{configId}", h.UpdateVolumeBackupConfig)
			r.Delete("/api/projects/{projectId}/applications/{applicationId}/volumes/{volumeId}/backup-configs/{configId}", h.DeleteVolumeBackupConfig)
			r.Post("/api/projects/{projectId}/applications/{applicationId}/volumes/{volumeId}/backup-configs/{configId}/run", h.RunVolumeBackupConfig)
			r.Get("/api/projects/{projectId}/applications/{applicationId}/volumes/{volumeId}/backups", h.ListVolumeBackups)
			r.Post("/api/projects/{projectId}/applications/{applicationId}/volumes/{volumeId}/backups/{backupId}/restore", h.RestoreVolumeBackup)
			r.Get("/api/projects/{projectId}/applications/{applicationId}/volumes/{volumeId}/restores", h.ListVolumeRestores)
			r.Get("/api/projects/{projectId}/applications/{applicationId}/volume-backup-configs", h.ListAppVolumeBackupConfigs)

			// Application file/config mounts
			r.Get("/api/projects/{projectId}/applications/{applicationId}/file-mounts", h.ListFileMounts)
			r.Get("/api/projects/{projectId}/applications/{applicationId}/file-mounts/{fileMountId}/reveal", h.RevealFileMount)
			r.Post("/api/projects/{projectId}/applications/{applicationId}/file-mounts", h.CreateFileMount)
			r.Put("/api/projects/{projectId}/applications/{applicationId}/file-mounts/{fileMountId}", h.UpdateFileMount)
			r.Delete("/api/projects/{projectId}/applications/{applicationId}/file-mounts/{fileMountId}", h.DeleteFileMount)

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
				// SMTP config: dedicated endpoints so the password stays
				// keyring-encrypted and masked (never in the generic settings list).
				r.Get("/api/settings/smtp", h.GetSMTPSettings)
				r.Put("/api/settings/smtp", h.UpdateSMTPSettings)
				r.Post("/api/settings/smtp/test", h.TestSMTPSettings)
				r.Get("/api/requests", h.ListAllRequestLogs)
				r.Get("/api/requests/summary", h.GetAllRequestsSummary)
				r.Get("/api/server/services", h.GetServerServices)
				// Live TLS state of the dashboard's own domain.
				r.Get("/api/server/dashboard-tls", h.GetDashboardTLS)
				// Read-only Docker inspect pages (containers/images/volumes/networks).
				r.Get("/api/docker/overview", h.GetDockerOverview)
				r.Get("/api/docker/containers", h.ListDockerContainers)
				r.Get("/api/docker/images", h.ListDockerImages)
				r.Get("/api/docker/volumes", h.ListDockerVolumes)
				r.Get("/api/docker/networks", h.ListDockerNetworks)
				r.Get("/api/audit-logs", h.ListAuditLogs)
				r.Get("/api/audit-logs/actions", h.ListAuditActions)
				r.Get("/api/audit-logs/export", h.ExportAuditLogs)
				r.Get("/api/proxy/reconciler", h.GetProxyReconcilerStatus)
				r.Post("/api/proxy/reconcile", h.ReconcileProxy)
				r.Get("/api/maintenance/queue", h.GetQueueStatus)
				r.Post("/api/maintenance/queue/clear", h.ClearQueue)
				r.Post("/api/maintenance/queue/clear-pending", h.ClearPendingQueue)
				r.Get("/api/maintenance/logs", h.GetPlatformLogs)
				r.Get("/api/maintenance/server-ip", h.GetServerIP)
				r.Post("/api/maintenance/restart", h.RestartService)
				r.Post("/api/maintenance/host-shell", h.CreateHostShellSession)
				r.Get("/api/quotas", h.ListQuotas)
				r.Get("/api/quotas/{scope}/{scopeId}", h.GetQuota)
				r.Put("/api/quotas/{scope}/{scopeId}", h.UpsertQuota)
				r.Delete("/api/quotas/{scope}/{scopeId}", h.DeleteQuota)
				// Centralised TLS certificate store (upload once, use per-domain)
				r.Get("/api/certificates", h.ListCertificates)
				// Every domain's observed TLS state in one view.
				r.Get("/api/domains/tls", h.ListDomainTLSStatus)
				r.Post("/api/certificates", h.UploadCertificate)
				r.Delete("/api/certificates/{certificateId}", h.DeleteCertificate)
				// Notification channels: route existing events out to providers.
				r.Get("/api/notification-events", h.ListNotificationEvents)
				r.Get("/api/notification-channels", h.ListNotificationChannels)
				r.Post("/api/notification-channels", h.CreateNotificationChannel)
				r.Post("/api/notification-channels/test", h.TestNotificationChannelParams)
				r.Put("/api/notification-channels/{channelId}", h.UpdateNotificationChannel)
				r.Patch("/api/notification-channels/{channelId}", h.SetNotificationChannelEnabled)
				r.Delete("/api/notification-channels/{channelId}", h.DeleteNotificationChannel)
				r.Post("/api/notification-channels/{channelId}/test", h.TestNotificationChannel)
				// Git provider app configs (per-instance GitHub App / OAuth clients)
				r.Get("/api/git/providers", h.ListGitProviderConfigs)
				r.Put("/api/git/providers", h.SaveGitProviderConfig)
				r.Delete("/api/git/providers/{configId}", h.DeleteGitProviderConfig)
				r.Get("/api/git/providers/github/manifest", h.GetGitHubAppManifest)
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
			r.Get("/api/projects/{projectId}/env/{envVarId}/reveal", h.RevealProjectEnvVar)
			r.Get("/api/projects/{projectId}/applications/{applicationId}/env", h.ListEnvVars)
			r.Put("/api/projects/{projectId}/applications/{applicationId}/env", h.UpdateEnvVars)
			r.Get("/api/projects/{projectId}/applications/{applicationId}/env/{envVarId}/reveal", h.RevealEnvVar)
		})

		// Databases: standard limit + timeout
		r.Group(func(r chi.Router) {
			r.Use(middleware.BodyLimit(defaultBodyLimit))
			r.Use(withTimeout(handlerTimeout))
			r.Get("/api/projects/{projectId}/databases", h.ListDatabases)
			r.Post("/api/projects/{projectId}/databases", h.CreateDatabase)
			r.Get("/api/projects/{projectId}/databases/{databaseId}", h.GetDatabase)
			r.Get("/api/projects/{projectId}/databases/{databaseId}/volume", h.GetDatabaseVolume)
			r.Put("/api/projects/{projectId}/databases/{databaseId}", h.UpdateDatabase)
			r.Post("/api/projects/{projectId}/databases/{databaseId}/external-access", h.SetDatabaseExternalAccess)
			r.Post("/api/projects/{projectId}/databases/{databaseId}/stop", h.StopDatabase)
			r.Post("/api/projects/{projectId}/databases/{databaseId}/start", h.StartDatabase)
			r.Post("/api/projects/{projectId}/databases/{databaseId}/restart", h.RestartDatabase)
			r.Post("/api/projects/{projectId}/databases/{databaseId}/reload", h.ReloadDatabase)
			r.Get("/api/projects/{projectId}/databases/{databaseId}/backups", h.ListDatabaseBackups)
			r.Post("/api/projects/{projectId}/databases/{databaseId}/backups", h.BackupDatabase)
			r.Delete("/api/projects/{projectId}/databases/{databaseId}/backups/{backupId}", h.DeleteDatabaseBackup)
			r.Post("/api/projects/{projectId}/databases/{databaseId}/restore", h.RestoreDatabase)
			r.Get("/api/projects/{projectId}/databases/{databaseId}/restores", h.ListDatabaseRestores)
			r.Get("/api/projects/{projectId}/databases/{databaseId}/logs/history", h.ListDatabaseLogs)
			r.Get("/api/projects/{projectId}/databases/{databaseId}/logs/sessions", h.ListDatabaseLogSessions)
			r.Post("/api/projects/{projectId}/databases/{databaseId}/upgrade", h.UpgradeDatabase)
			r.Get("/api/projects/{projectId}/databases/{databaseId}/deletion-impact", h.GetDatabaseDeletionImpact)
			r.Delete("/api/projects/{projectId}/databases/{databaseId}", h.DeleteDatabase)

			// Scheduled backup configurations per database
			r.Get("/api/projects/{projectId}/databases/{databaseId}/backup-configs", h.ListDatabaseBackupConfigs)
			r.Post("/api/projects/{projectId}/databases/{databaseId}/backup-configs", h.CreateDatabaseBackupConfig)
			r.Put("/api/projects/{projectId}/databases/{databaseId}/backup-configs/{configId}", h.UpdateDatabaseBackupConfig)
			r.Delete("/api/projects/{projectId}/databases/{databaseId}/backup-configs/{configId}", h.DeleteDatabaseBackupConfig)
			r.Post("/api/projects/{projectId}/databases/{databaseId}/backup-configs/{configId}/run", h.RunDatabaseBackupConfig)

			// Project-scoped backup destinations (managed by project members)
			r.Get("/api/projects/{projectId}/backup-destinations", h.ListBackupDestinations)
			r.Post("/api/projects/{projectId}/backup-destinations", h.CreateBackupDestination)
			r.Post("/api/projects/{projectId}/backup-destinations/test", h.TestBackupDestinationParams)
			r.Put("/api/projects/{projectId}/backup-destinations/{destId}", h.UpdateBackupDestination)
			r.Delete("/api/projects/{projectId}/backup-destinations/{destId}", h.DeleteBackupDestination)
			r.Post("/api/projects/{projectId}/backup-destinations/{destId}/test", h.TestBackupDestination)

			// Project backup activity (recent runs across the project's databases)
			r.Get("/api/projects/{projectId}/backups", h.ListProjectBackups)
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

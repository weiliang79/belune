package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"

	"github.com/ungweiliang/selfhost-paas/internal/handler"
	"github.com/ungweiliang/selfhost-paas/internal/server/middleware"
	"github.com/ungweiliang/selfhost-paas/internal/service"
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

func registerRoutes(r chi.Router, h *handler.Handler, auth *service.AuthService, disableRateLimit bool) {
	// Health check (deep — checks DB, Redis, Docker)
	r.Get("/healthz", h.HealthCheck)

	// Public routes
	r.Group(func(r chi.Router) {
		// Login rate limit: 5 req/min per IP
		r.Group(func(r chi.Router) {
			if !disableRateLimit {
				r.Use(httprate.LimitByIP(5, time.Minute))
			}
			r.Post("/api/auth/login", h.Login)
		})

		r.Get("/api/auth/setup", h.Setup)
		r.Post("/api/auth/setup", h.Setup)

		// Feature flags / system status
		r.Get("/api/features", h.GetFeatures)

		// Webhooks: 30 req/min per IP
		r.Group(func(r chi.Router) {
			if !disableRateLimit {
				r.Use(httprate.LimitByIP(30, time.Minute))
			}
			r.Post("/api/webhooks/push", h.HandleWebhookPush)
		})
	})

	// WebSocket: auth-protected but not rate-limited. The httprate middleware
	// wraps the ResponseWriter in a way that breaks Hijacker, preventing the
	// protocol upgrade.
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(auth))
		r.Get("/api/ws", h.HandleWebSocket)
	})

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(auth))
		if !disableRateLimit {
			// Key by user ID so each authenticated user gets their own 100 req/min
			// bucket, independent of IP. Falls back to IP for unauthenticated paths
			// that share this group (none currently, but safe by default).
			r.Use(httprate.Limit(100, time.Minute, httprate.WithKeyFuncs(keyByUserIDOrIP)))
		}

		r.Post("/api/auth/logout", h.Logout)
		r.Get("/api/auth/me", h.Me)
		r.Put("/api/auth/password", h.ChangeOwnPassword)
		r.Put("/api/auth/profile", h.UpdateProfile)

		// Admin-only routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole("admin"))

			r.Get("/api/users", h.ListUsers)
			r.Post("/api/users", h.CreateUser)
			r.Put("/api/users/{userId}/role", h.UpdateUserRole)
			r.Delete("/api/users/{userId}", h.DeleteUser)
			r.Put("/api/users/{userId}/password", h.ResetUserPassword)
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
		r.Put("/api/projects/{projectId}/transfer", h.TransferProject) // admin-only enforced in handler

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

		// Deployments
		r.Get("/api/projects/{projectId}/applications/{applicationId}/deployments", h.ListDeployments)
		r.Get("/api/projects/{projectId}/applications/{applicationId}/deployments/{deploymentId}", h.GetDeployment)
		r.Get("/api/projects/{projectId}/applications/{applicationId}/deployments/{deploymentId}/build-logs", h.StreamBuildLogs)
		r.Post("/api/projects/{projectId}/applications/{applicationId}/rollback", h.RollbackDeployment)

		// Logs
		r.Get("/api/projects/{projectId}/applications/{applicationId}/logs", h.StreamLogs)
		r.Get("/api/projects/{projectId}/applications/{applicationId}/logs/history", h.ListApplicationLogs)

		// Request logs (HTTP access logs from Caddy)
		r.Get("/api/projects/{projectId}/applications/{applicationId}/requests", h.ListRequestLogs)
		r.Get("/api/projects/{projectId}/applications/{applicationId}/requests/stream", h.StreamRequestLogs)

		// Environment variables
		r.Get("/api/projects/{projectId}/applications/{applicationId}/env", h.ListEnvVars)
		r.Put("/api/projects/{projectId}/applications/{applicationId}/env", h.UpdateEnvVars)

		// Domains
		r.Get("/api/projects/{projectId}/applications/{applicationId}/domains", h.ListDomains)
		r.Post("/api/projects/{projectId}/applications/{applicationId}/domains", h.AddDomain)
		r.Put("/api/projects/{projectId}/applications/{applicationId}/domains/{domainId}", h.UpdateDomain)
		r.Delete("/api/projects/{projectId}/applications/{applicationId}/domains/{domainId}", h.RemoveDomain)

		// Domain route features
		r.Get("/api/projects/{projectId}/applications/{applicationId}/domains/{domainId}/features", h.ListRouteFeatures)
		r.Put("/api/projects/{projectId}/applications/{applicationId}/domains/{domainId}/features", h.UpsertRouteFeature)
		r.Delete("/api/projects/{projectId}/applications/{applicationId}/domains/{domainId}/features/{featureId}", h.DeleteRouteFeature)

		// Project environment variables
		r.Get("/api/projects/{projectId}/env", h.ListProjectEnvVars)
		r.Put("/api/projects/{projectId}/env", h.UpdateProjectEnvVars)

		// Databases
		r.Get("/api/projects/{projectId}/databases", h.ListDatabases)
		r.Post("/api/projects/{projectId}/databases", h.CreateDatabase)
		r.Get("/api/projects/{projectId}/databases/{databaseId}", h.GetDatabase)
		r.Delete("/api/projects/{projectId}/databases/{databaseId}", h.DeleteDatabase)

		// Application live metrics stream (no historical — container metrics are on-demand only)
		r.Get("/api/projects/{projectId}/applications/{applicationId}/metrics/stream", h.StreamApplicationMetrics)

		// Global deployments (all authenticated users; scoped by role in handler)
		r.Get("/api/deployments", h.GetGlobalDeployments)

		// Metrics, Settings & Cleanup (admin-only)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole("admin"))
			r.Get("/api/metrics", h.GetMetrics)
			r.Get("/api/metrics/host", h.GetHostHistoricalMetrics)
			r.Get("/api/metrics/host/stream", h.StreamHostMetrics)
			r.Post("/api/cleanup", h.TriggerCleanup)
			r.Get("/api/settings", h.ListSettings)
			r.Put("/api/settings", h.UpdateSettings)
			// Global request logs (admin only)
			r.Get("/api/requests", h.ListAllRequestLogs)
			r.Get("/api/requests/stream", h.StreamAllRequestLogs)

			// Audit logs (admin only)
			r.Get("/api/audit-logs", h.ListAuditLogs)
		})

		// Build (standalone build without deploy)
		r.Post("/api/projects/{projectId}/applications/{applicationId}/build", h.BuildApplication)
	})
}

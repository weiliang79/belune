package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ungweiliang/selfhost-paas/internal/handler"
	"github.com/ungweiliang/selfhost-paas/internal/server/middleware"
	"github.com/ungweiliang/selfhost-paas/internal/service"
)

func registerRoutes(r chi.Router, h *handler.Handler, auth *service.AuthService) {
	// Health check
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Public routes
	r.Group(func(r chi.Router) {
		r.Post("/api/auth/login", h.Login)
		r.Post("/api/auth/logout", h.Logout)
		r.Get("/api/auth/setup", h.Setup)
		r.Post("/api/auth/setup", h.Setup)

		// Webhooks (unauthenticated — verified via HMAC/token per-application)
		r.Post("/api/webhooks/push", h.HandleWebhookPush)
	})

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(auth))

		r.Get("/api/auth/me", h.Me)
		r.Put("/api/auth/password", h.ChangeOwnPassword)

		// Admin-only routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole("admin"))

			r.Get("/api/users", h.ListUsers)
			r.Post("/api/users", h.CreateUser)
			r.Put("/api/users/{userId}/role", h.UpdateUserRole)
			r.Delete("/api/users/{userId}", h.DeleteUser)
			r.Put("/api/users/{userId}/password", h.ResetUserPassword)
		})

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

		// Logs
		r.Get("/api/projects/{projectId}/applications/{applicationId}/logs", h.StreamLogs)

		// Environment variables
		r.Get("/api/projects/{projectId}/applications/{applicationId}/env", h.ListEnvVars)
		r.Put("/api/projects/{projectId}/applications/{applicationId}/env", h.UpdateEnvVars)

		// Domains
		r.Get("/api/projects/{projectId}/applications/{applicationId}/domains", h.ListDomains)
		r.Post("/api/projects/{projectId}/applications/{applicationId}/domains", h.AddDomain)
		r.Delete("/api/projects/{projectId}/applications/{applicationId}/domains/{domainId}", h.RemoveDomain)

		// Databases
		r.Get("/api/projects/{projectId}/databases", h.ListDatabases)
		r.Post("/api/projects/{projectId}/databases", h.CreateDatabase)
		r.Get("/api/projects/{projectId}/databases/{databaseId}", h.GetDatabase)
		r.Delete("/api/projects/{projectId}/databases/{databaseId}", h.DeleteDatabase)

		// Metrics & Cleanup (admin-only)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole("admin"))
			r.Get("/api/metrics", h.GetMetrics)
			r.Post("/api/cleanup", h.TriggerCleanup)
		})

		// Build (standalone build without deploy)
		r.Post("/api/projects/{projectId}/applications/{applicationId}/build", h.BuildApplication)
	})
}

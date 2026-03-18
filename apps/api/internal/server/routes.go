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

		// Webhooks (unauthenticated — verified via HMAC/token per-service)
		r.Post("/api/webhooks/push", h.HandleWebhookPush)
	})

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(auth))

		r.Get("/api/auth/me", h.Me)

		// Projects
		r.Get("/api/projects", h.ListProjects)
		r.Post("/api/projects", h.CreateProject)
		r.Get("/api/projects/{projectId}", h.GetProject)
		r.Put("/api/projects/{projectId}", h.UpdateProject)
		r.Delete("/api/projects/{projectId}", h.DeleteProject)

		// Services
		r.Get("/api/projects/{projectId}/services", h.ListServices)
		r.Post("/api/projects/{projectId}/services", h.CreateService)
		r.Get("/api/projects/{projectId}/services/{serviceId}", h.GetService)
		r.Put("/api/projects/{projectId}/services/{serviceId}", h.UpdateService)
		r.Delete("/api/projects/{projectId}/services/{serviceId}", h.DeleteService)
		r.Put("/api/projects/{projectId}/services/{serviceId}/webhook", h.UpdateServiceWebhook)
		r.Post("/api/projects/{projectId}/services/{serviceId}/deploy", h.DeployService)
		r.Post("/api/projects/{projectId}/services/{serviceId}/stop", h.StopService)
		r.Post("/api/projects/{projectId}/services/{serviceId}/restart", h.RestartService)

		// Deployments
		r.Get("/api/projects/{projectId}/services/{serviceId}/deployments", h.ListDeployments)
		r.Get("/api/projects/{projectId}/services/{serviceId}/deployments/{deploymentId}", h.GetDeployment)

		// Logs
		r.Get("/api/projects/{projectId}/services/{serviceId}/logs", h.StreamLogs)

		// Environment variables
		r.Get("/api/projects/{projectId}/services/{serviceId}/env", h.ListEnvVars)
		r.Put("/api/projects/{projectId}/services/{serviceId}/env", h.UpdateEnvVars)

		// Domains
		r.Get("/api/projects/{projectId}/services/{serviceId}/domains", h.ListDomains)
		r.Post("/api/projects/{projectId}/services/{serviceId}/domains", h.AddDomain)
		r.Delete("/api/projects/{projectId}/services/{serviceId}/domains/{domainId}", h.RemoveDomain)

		// Databases
		r.Get("/api/projects/{projectId}/databases", h.ListDatabases)
		r.Post("/api/projects/{projectId}/databases", h.CreateDatabase)
		r.Get("/api/projects/{projectId}/databases/{databaseId}", h.GetDatabase)
		r.Delete("/api/projects/{projectId}/databases/{databaseId}", h.DeleteDatabase)

		// Metrics
		r.Get("/api/metrics", h.GetMetrics)
	})
}

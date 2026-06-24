package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/ungweiliang/selfhost-paas/internal/config"
	"github.com/ungweiliang/selfhost-paas/internal/handler"
	"github.com/ungweiliang/selfhost-paas/internal/pkg/metrics"
	"github.com/ungweiliang/selfhost-paas/internal/proxy"
	"github.com/ungweiliang/selfhost-paas/internal/quota"
	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/server/middleware"
	"github.com/ungweiliang/selfhost-paas/internal/service"
	"github.com/ungweiliang/selfhost-paas/internal/service/backup"
	"github.com/ungweiliang/selfhost-paas/internal/service/email"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
	"github.com/ungweiliang/selfhost-paas/internal/terminal"
	"github.com/ungweiliang/selfhost-paas/internal/ws"
	"github.com/ungweiliang/selfhost-paas/web"
)

type Server struct {
	cfg     *config.Config
	db      *pgxpool.Pool
	router  chi.Router
	handler *handler.Handler
	auth    *service.AuthService
}

func New(cfg *config.Config, db *pgxpool.Pool, queries *generated.Queries, asynqClient handler.TaskEnqueuer, rt runtime.ContainerRuntime, pm proxy.ProxyManager, reconciler handler.ReconcilerStatusProvider, rdb *redis.Client, hub *ws.Hub, auditSvc *service.AuditService, notifySvc *service.NotificationService, termMgr *terminal.Manager, emailSvc *email.Service) *Server {
	auth := service.NewAuthService(queries, cfg.JWTSecret, cfg.JWTExpiryHours, cfg.JWTRefreshHours, rdb)
	appSvc := service.NewApplicationService(db, queries, rt, cfg.Keyring)
	projSvc := service.NewProjectService(queries, rt)
	dbSvc := service.NewDatabaseService(queries, rt, backup.New(cfg))
	quotaSvc := quota.NewService(queries)

	s := &Server{
		cfg:     cfg,
		db:      db,
		auth:    auth,
		handler: handler.New(cfg, db, queries, asynqClient, rt, pm, reconciler, auth, rdb, appSvc, projSvc, dbSvc, hub, auditSvc, notifySvc, termMgr, quotaSvc, emailSvc),
	}

	s.router = s.setupRouter()
	return s
}

func (s *Server) Router() chi.Router {
	return s.router
}

func (s *Server) setupRouter() chi.Router {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	// OTel HTTP server middleware: one span per request plus W3C traceparent
	// extraction. No-ops when the tracer provider is the default no-op.
	r.Use(func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, "http.request")
	})
	// Prometheus HTTP counters/histograms. Mounted before the request logger so
	// latency observations include logger overhead (negligible, simpler to
	// reason about).
	r.Use(metrics.HTTPMiddleware)
	r.Use(middleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	if s.cfg.TLS {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
				next.ServeHTTP(w, r)
			})
		})
	}
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Security-Policy",
				"default-src 'self'; "+
					"script-src 'self'; "+
					"style-src 'self' 'unsafe-inline'; "+
					"img-src 'self' data:; "+
					"connect-src 'self' ws: wss:; "+
					"font-src 'self'; "+
					"object-src 'none'; "+
					"frame-ancestors 'none'")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			next.ServeHTTP(w, r)
		})
	})
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   s.cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Register API + health routes
	registerRoutes(r, s.handler, s.auth, s.cfg.DisableRateLimiting)

	// Catch-all: serve the embedded SPA for any unmatched path
	if spaHandler := web.Handler(); spaHandler != nil {
		r.Handle("/*", spaHandler)
	}

	return r
}

package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/ungweiliang/selfhost-paas/internal/config"
	"github.com/ungweiliang/selfhost-paas/internal/handler"
	"github.com/ungweiliang/selfhost-paas/internal/proxy"
	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/server/middleware"
	"github.com/ungweiliang/selfhost-paas/internal/service"
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

func New(cfg *config.Config, db *pgxpool.Pool, queries *generated.Queries, asynqClient handler.TaskEnqueuer, rt runtime.ContainerRuntime, pm proxy.ProxyManager, rdb *redis.Client, hub *ws.Hub, auditSvc *service.AuditService, termMgr *terminal.Manager) *Server {
	auth := service.NewAuthService(queries, cfg.JWTSecret, cfg.JWTExpiryHours, rdb)
	appSvc := service.NewApplicationService(db, queries, rt, cfg.EncryptionKey)
	projSvc := service.NewProjectService(queries, rt)
	dbSvc := service.NewDatabaseService(queries, rt)
	gitCredSvc := service.NewGitCredentialService(queries, cfg.EncryptionKey)

	s := &Server{
		cfg:     cfg,
		db:      db,
		auth:    auth,
		handler: handler.New(cfg, db, queries, asynqClient, rt, pm, auth, rdb, appSvc, projSvc, dbSvc, gitCredSvc, hub, auditSvc, termMgr),
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
	r.Use(middleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(middleware.BodyLimit(1 << 20)) // 1MB max body size
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

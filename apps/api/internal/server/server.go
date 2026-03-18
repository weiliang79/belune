package server

import (
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/ungweiliang/selfhost-paas/internal/config"
	"github.com/ungweiliang/selfhost-paas/internal/handler"
	"github.com/ungweiliang/selfhost-paas/internal/proxy"
	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/server/middleware"
	"github.com/ungweiliang/selfhost-paas/internal/service"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

type Server struct {
	cfg     *config.Config
	db      *pgxpool.Pool
	router  chi.Router
	handler *handler.Handler
	auth    *service.AuthService
}

func New(cfg *config.Config, db *pgxpool.Pool, queries *generated.Queries, asynqClient *asynq.Client, rt runtime.ContainerRuntime, pm proxy.ProxyManager, rdb *redis.Client) *Server {
	auth := service.NewAuthService(queries, cfg.JWTSecret)

	s := &Server{
		cfg:     cfg,
		db:      db,
		auth:    auth,
		handler: handler.New(cfg, db, queries, asynqClient, rt, pm, auth, rdb),
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
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://localhost:8080"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Register routes
	registerRoutes(r, s.handler, s.auth)

	return r
}

package server

import (
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ungweiliang/selfhost-paas/internal/config"
	"github.com/ungweiliang/selfhost-paas/internal/handler"
	"github.com/ungweiliang/selfhost-paas/internal/server/middleware"
)

type Server struct {
	cfg     *config.Config
	db      *pgxpool.Pool
	router  chi.Router
	handler *handler.Handler
}

func New(cfg *config.Config, db *pgxpool.Pool) *Server {
	s := &Server{
		cfg:     cfg,
		db:      db,
		handler: handler.New(cfg, db),
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
	registerRoutes(r, s.handler, s.cfg)

	return r
}

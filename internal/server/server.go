package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/config"
	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor"
	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor/docsearch"
	exechttp "github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor/http"
	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor/mcp"
	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor/script"
	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor/sql"
	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/middleware"
)

// Server is the HTTP server for agenthub-skill-runtime.
type Server struct {
	router   http.Handler
	pool     *pgxpool.Pool
	registry *executor.Registry
	resolver *executor.SkillResolver
}

// New creates a new Server with all routes mounted.
func New(cfg *config.Config, pool *pgxpool.Pool) *Server {
	reg := executor.NewRegistry()
	reg.Register(exechttp.NewHTTPToolExecutor(cfg.BackendBaseURL))
	reg.Register(sql.NewSQLToolExecutor(pool))
	reg.Register(docsearch.NewDocumentSearchToolExecutor(pool))
	reg.Register(mcp.NewMCPToolExecutor(pool))
	reg.Register(&script.ScriptToolExecutor{})

	s := &Server{
		pool:     pool,
		registry: reg,
		resolver: executor.NewSkillResolver(pool),
	}

	chain := middleware.New(cfg.KeycloakBaseURL, cfg.CORSOrigins())

	r := chi.NewRouter()
	r.Use(chiMiddleware.RealIP)

	// CORS at root level so OPTIONS preflight is handled before chi returns 405.
	r.Use(chain.CORSHandler())
	r.Options("/*", func(w http.ResponseWriter, r *http.Request) {})

	// Health endpoints — no auth required.
	r.Get("/health", s.handleHealth)
	r.Get("/ready", s.handleReady)

	// Protected routes — require valid JWT.
	r.Group(func(r chi.Router) {
		for _, m := range chain.Protected() {
			r.Use(m)
		}
		mountRoutes(r, s)
	})

	s.router = r
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "UP"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if err := s.pool.Ping(r.Context()); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "DOWN", "reason": "database unreachable"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "UP"})
}

package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// mountRoutes registers all skill-runtime API routes on the given router.
func mountRoutes(r chi.Router, s *Server) {
	// POST /api/skills/{skillSlug}/execute — resolve skill binding and execute via tool executor.
	r.Post("/api/skills/{skillSlug}/execute", s.handleExecuteSkill)

	// POST /api/tools/{toolId}/execute — execute a tool directly by ID.
	r.Post("/api/tools/{toolId}/execute", s.handleExecuteTool)
}

func (s *Server) handleExecuteSkill(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
}

func (s *Server) handleExecuteTool(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
}

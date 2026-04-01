package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor"
	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/invoker"
	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/middleware"
)

// mountRoutes registers all skill-runtime API routes on the given router.
func mountRoutes(r chi.Router, s *Server) {
	// POST /api/skills/{skillSlug}/execute — resolve skill binding and execute via tool executor.
	r.Post("/api/skills/{skillSlug}/execute", s.handleExecuteSkill)

	// POST /api/tools/{toolId}/execute — execute a tool directly by ID.
	r.Post("/api/tools/{toolId}/execute", s.handleExecuteTool)
}

func (s *Server) handleExecuteSkill(w http.ResponseWriter, r *http.Request) {
	skillSlug := chi.URLParam(r, "skillSlug")
	tenantID := middleware.TenantIDFromContext(r.Context())

	var input map[string]any
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tool, err := s.resolver.ResolveSkill(r.Context(), tenantID, skillSlug)
	if err != nil {
		writeError(w, http.StatusNotFound, "skill not found")
		return
	}

	s.executeTool(w, r, tool, skillSlug, input)
}

func (s *Server) handleExecuteTool(w http.ResponseWriter, r *http.Request) {
	toolID := chi.URLParam(r, "toolId")
	tenantID := middleware.TenantIDFromContext(r.Context())

	var input map[string]any
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tool, err := s.resolver.ResolveTool(r.Context(), tenantID, toolID)
	if err != nil {
		writeError(w, http.StatusNotFound, "tool not found")
		return
	}

	s.executeTool(w, r, tool, "", input)
}

// executeTool resolves the executor, wraps it with an invoker, and writes the result.
func (s *Server) executeTool(w http.ResponseWriter, r *http.Request, tool *executor.Tool, skillSlug string, input map[string]any) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	exec, err := s.registry.Get(tool.Type)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "unsupported tool type: "+tool.Type)
		return
	}

	var config map[string]any
	if len(tool.Config) > 0 {
		if err := json.Unmarshal(tool.Config, &config); err != nil {
			writeError(w, http.StatusInternalServerError, "invalid tool configuration")
			return
		}
	}

	ec := executor.ExecutionContext{
		ToolID:    tool.ID,
		SkillSlug: skillSlug,
		TenantID:  tenantID,
		Input:     input,
		Config:    config,
	}

	inv := invoker.New(exec, invoker.DefaultConfig())
	result, err := inv.Invoke(r.Context(), ec)
	if err != nil {
		if errors.Is(err, invoker.ErrCircuitOpen) {
			writeError(w, http.StatusServiceUnavailable, "tool temporarily unavailable")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"output":    result.Output,
		"latencyMs": result.LatencyMs,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

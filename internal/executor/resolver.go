package executor

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Skill represents a skill entity from the ah_{tenantID} schema.
type Skill struct {
	ID   string
	Slug string
	Name string
}

// Tool represents a tool entity from the ah_{tenantID} schema.
type Tool struct {
	ID     string
	Type   string // HTTP, SQL, DOCUMENT_SEARCH, CUSTOM
	Config []byte // JSONB config blob
}

// SkillResolver resolves a skill slug or tool ID to a Tool via the tenant schema.
type SkillResolver struct {
	pool *pgxpool.Pool
}

// NewSkillResolver creates a SkillResolver backed by the given connection pool.
func NewSkillResolver(pool *pgxpool.Pool) *SkillResolver {
	return &SkillResolver{pool: pool}
}

// ResolveSkill returns the bound Tool for a skill slug in the given tenant schema.
// It follows the skill → skill_tool → tool chain.
//
// Resolution order (bug 199 fix):
//  1. tenant.skill.slug (primary — single-tool skill aggregator pattern)
//  2. tenant.tool.slug (bug 199 — when skill has 2+ tools, the api exposes each
//     tool by its own slug; ResolveSkill must find it directly in tool table)
//  3. ah_core.tool.slug (platform tools like agenthub_list_skills)
func (r *SkillResolver) ResolveSkill(ctx context.Context, tenantID, skillSlug string) (*Tool, error) {
	schema := TenantSchema(tenantID)
	query := fmt.Sprintf(`
		SELECT t.id, t.type, t.config
		FROM %s.skill s
		JOIN %s.skill_tool st ON st.skill_id = s.id AND st.is_active = true
		JOIN %s.tool t ON t.id = st.tool_id
		WHERE s.slug = $1
		ORDER BY st.priority, st.created_at
		LIMIT 1
	`, schema, schema, schema)

	var tool Tool
	row := r.pool.QueryRow(ctx, query, skillSlug)
	if err := row.Scan(&tool.ID, &tool.Type, &tool.Config); err != nil {
		// Bug 199: tool slug fallback within tenant schema. When skill has 2+
		// tools, api exposes each tool by its slug; LLM picks one and the
		// skill-runtime needs to resolve directly to that tool.
		toolQuery := fmt.Sprintf(`SELECT id, type, config FROM %s.tool WHERE slug = $1`, schema)
		toolRow := r.pool.QueryRow(ctx, toolQuery, skillSlug)
		if toolErr := toolRow.Scan(&tool.ID, &tool.Type, &tool.Config); toolErr == nil {
			return &tool, nil
		}

		// Fallback: try resolving as a platform tool from ah_core by slug.
		// Core tools (e.g. agenthub_list_skills) are stored in ah_core.tool and
		// exposed to the LLM by tool slug directly — there is no corresponding
		// skill entry in the tenant schema for them.
		coreQuery := `SELECT id, type, config FROM ah_core.tool WHERE slug = $1 AND is_active = true`
		coreRow := r.pool.QueryRow(ctx, coreQuery, skillSlug)
		if coreErr := coreRow.Scan(&tool.ID, &tool.Type, &tool.Config); coreErr != nil {
			return nil, fmt.Errorf("resolver: skill %q not found in tenant %q: %w", skillSlug, tenantID, err)
		}
		return &tool, nil
	}
	return &tool, nil
}

// ResolveTool returns a Tool by its ID in the given tenant schema.
func (r *SkillResolver) ResolveTool(ctx context.Context, tenantID, toolID string) (*Tool, error) {
	schema := TenantSchema(tenantID)
	query := fmt.Sprintf(`SELECT id, type, config FROM %s.tool WHERE id = $1`, schema)

	var tool Tool
	row := r.pool.QueryRow(ctx, query, toolID)
	if err := row.Scan(&tool.ID, &tool.Type, &tool.Config); err != nil {
		return nil, fmt.Errorf("resolver: tool %q not found in tenant %q: %w", toolID, tenantID, err)
	}
	return &tool, nil
}

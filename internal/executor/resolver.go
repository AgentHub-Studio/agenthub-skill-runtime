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
func (r *SkillResolver) ResolveSkill(ctx context.Context, tenantID, skillSlug string) (*Tool, error) {
	schema := "ah_" + tenantID
	query := fmt.Sprintf(`
		SELECT t.id, t.type, t.config
		FROM %s.skill s
		JOIN %s.skill_tool st ON st.skill_id = s.id
		JOIN %s.tool t ON t.id = st.tool_id
		WHERE s.slug = $1
		LIMIT 1
	`, schema, schema, schema)

	var tool Tool
	row := r.pool.QueryRow(ctx, query, skillSlug)
	if err := row.Scan(&tool.ID, &tool.Type, &tool.Config); err != nil {
		return nil, fmt.Errorf("resolver: skill %q not found in tenant %q: %w", skillSlug, tenantID, err)
	}
	return &tool, nil
}

// ResolveTool returns a Tool by its ID in the given tenant schema.
func (r *SkillResolver) ResolveTool(ctx context.Context, tenantID, toolID string) (*Tool, error) {
	schema := "ah_" + tenantID
	query := fmt.Sprintf(`SELECT id, type, config FROM %s.tool WHERE id = $1`, schema)

	var tool Tool
	row := r.pool.QueryRow(ctx, query, toolID)
	if err := row.Scan(&tool.ID, &tool.Type, &tool.Config); err != nil {
		return nil, fmt.Errorf("resolver: tool %q not found in tenant %q: %w", toolID, tenantID, err)
	}
	return &tool, nil
}

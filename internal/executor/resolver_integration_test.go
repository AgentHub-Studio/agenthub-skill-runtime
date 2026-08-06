//go:build integration

package executor_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor"
)

func TestIntegration_SkillResolver_ResolvesOnlyActivelyBoundToolSlugs(t *testing.T) {
	dsn := os.Getenv("AGENTHUB_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("AGENTHUB_TEST_DATABASE_URL is required for resolver integration tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	tenantID := "resolver" + uuid.NewString()
	schema := executor.TenantSchema(tenantID)
	t.Cleanup(func() {
		_, cleanupErr := pool.Exec(context.Background(), fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE`, schema))
		require.NoError(t, cleanupErr)
	})

	_, err = pool.Exec(ctx, fmt.Sprintf(`
		CREATE SCHEMA %s;
		CREATE TABLE %s.skill (
			id UUID PRIMARY KEY,
			slug VARCHAR(255) NOT NULL UNIQUE
		);
		CREATE TABLE %s.tool (
			id UUID PRIMARY KEY,
			slug VARCHAR(255) NOT NULL UNIQUE,
			type VARCHAR(50) NOT NULL,
			config JSONB NOT NULL DEFAULT '{}'::jsonb
		);
		CREATE TABLE %s.skill_tool (
			skill_id UUID NOT NULL REFERENCES %s.skill (id),
			tool_id UUID NOT NULL REFERENCES %s.tool (id),
			priority INTEGER NOT NULL DEFAULT 0,
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`, schema, schema, schema, schema, schema, schema))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.skill (id, slug) VALUES
			('00000000-0000-0000-0000-000000000001', 'multi-tool-skill');
		INSERT INTO %s.tool (id, slug, type, config) VALUES
			('00000000-0000-0000-0000-000000000011', 'bound-tool', 'HTTP', '{}'::jsonb),
			('00000000-0000-0000-0000-000000000012', 'inactive-tool', 'HTTP', '{}'::jsonb),
			('00000000-0000-0000-0000-000000000013', 'orphan-tool', 'HTTP', '{}'::jsonb);
		INSERT INTO %s.skill_tool (skill_id, tool_id, priority, is_active) VALUES
			('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000011', 10, true),
			('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000012', 20, false);
	`, schema, schema, schema))
	require.NoError(t, err)

	resolver := executor.NewSkillResolver(pool)
	bound, err := resolver.ResolveSkill(ctx, tenantID, "bound-tool")
	require.NoError(t, err)
	require.Equal(t, "00000000-0000-0000-0000-000000000011", bound.ID)

	for _, slug := range []string{"inactive-tool", "orphan-tool"} {
		t.Run(slug, func(t *testing.T) {
			_, resolveErr := resolver.ResolveSkill(ctx, tenantID, slug)
			require.Error(t, resolveErr)
			require.Contains(t, resolveErr.Error(), "not found")
		})
	}
}

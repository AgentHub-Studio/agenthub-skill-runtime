package sql_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor"
	sqlexec "github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor/sql"
)

func TestSQLExecutor_GetToolType(t *testing.T) {
	e := sqlexec.NewSQLToolExecutor(nil)
	assert.Equal(t, "SQL", e.GetToolType())
}

func TestSQLExecutor_MissingDatasourceID(t *testing.T) {
	e := sqlexec.NewSQLToolExecutor(nil)
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{
			"query": "SELECT 1",
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "datasource not configured")
}

func TestSQLExecutor_MissingQuery(t *testing.T) {
	e := sqlexec.NewSQLToolExecutor(nil)
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{
			"datasource_id": "550e8400-e29b-41d4-a716-446655440000",
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query is required")
}

func TestSQLExecutor_EmptyConfig_MissingBothFields(t *testing.T) {
	e := sqlexec.NewSQLToolExecutor(nil)
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "datasource not configured")
}

func TestSQLExecutor_NilConfig_MissingBothFields(t *testing.T) {
	e := sqlexec.NewSQLToolExecutor(nil)
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: nil,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "datasource not configured")
}

func TestSQLExecutor_OnlyDatasourceID_MissingQuery(t *testing.T) {
	e := sqlexec.NewSQLToolExecutor(nil)
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{
			"datasource_id": "550e8400-e29b-41d4-a716-446655440000",
			"max_rows":      50,
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query is required")
}

func TestSQLExecutor_RejectsConflictingDatasourceAliasesBeforeFetch(t *testing.T) {
	e := sqlexec.NewSQLToolExecutor(nil)
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{
			"datasource_id": "00000000-0000-0000-0000-000000000001",
			"dataSourceId":  "00000000-0000-0000-0000-000000000002",
			"query":         "SELECT 1",
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "datasource_id")
	assert.Contains(t, err.Error(), "dataSourceId")
}

func TestSQLExecutor_AcceptsEquivalentDatasourceAliases(t *testing.T) {
	e := sqlexec.NewSQLToolExecutor(nil)
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		Config: map[string]any{
			"datasource_id": "AAAAAAAA-0000-0000-0000-000000000001",
			"dataSourceId":  "aaaaaaaa-0000-0000-0000-000000000001",
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query is required")
}

// TestSQLExecutor_UnsupportedDatasourceType documents the behavior when
// a non-POSTGRESQL datasource is returned. Full integration requires Testcontainers.
//
// Wire contract: Execute returns "unsupported datasource type" for MySQL/SQL Server.
func TestSQLExecutor_UnsupportedDatasourceType_IsDocumented(t *testing.T) {
	t.Log("SQLToolExecutor supports POSTGRESQL only. MySQL/SQL Server return 'unsupported datasource type'.")
}

// TestSQLExecutor_RenderInputTemplate_IsDocumented describes {{input.key}} substitution.
//
// Wire contract: "SELECT * FROM users WHERE name = '{{input.name}}'" with
// Input{"name": "Alice"} is executed as "SELECT * FROM users WHERE name = $1"
// with "Alice" passed as a pgx argument.
func TestSQLExecutor_RenderInputTemplate_IsDocumented(t *testing.T) {
	t.Log("renderInputTemplate: {{input.key}} placeholders are converted to pgx parameters before execution.")
}

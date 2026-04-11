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
// Input{"name": "Alice"} produces "SELECT * FROM users WHERE name = 'Alice'".
func TestSQLExecutor_RenderInputTemplate_IsDocumented(t *testing.T) {
	t.Log("renderInputTemplate: {{input.key}} placeholders are replaced with Input[key] before execution.")
}

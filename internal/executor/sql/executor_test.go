package sql_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor"
	sqlexec "github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor/sql"
)

func TestSQLToolExecutor_GetToolType(t *testing.T) {
	e := sqlexec.NewSQLToolExecutor(nil)
	assert.Equal(t, "SQL", e.GetToolType())
}

func TestSQLToolExecutor_Execute_MissingQuery(t *testing.T) {
	e := sqlexec.NewSQLToolExecutor(nil)
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		TenantID: "test-tenant",
		Config: map[string]any{
			"datasource_id": "some-uuid",
			// query intentionally omitted
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query is required")
}

func TestSQLToolExecutor_Execute_MissingDatasourceID(t *testing.T) {
	e := sqlexec.NewSQLToolExecutor(nil)
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		TenantID: "test-tenant",
		Config: map[string]any{
			"query": "SELECT 1",
			// datasource_id intentionally omitted
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "datasource_id is required")
}

func TestSQLToolExecutor_Execute_EmptyConfig(t *testing.T) {
	e := sqlexec.NewSQLToolExecutor(nil)
	_, err := e.Execute(context.Background(), executor.ExecutionContext{
		TenantID: "test-tenant",
		Config:   map[string]any{},
	})
	require.Error(t, err)
}

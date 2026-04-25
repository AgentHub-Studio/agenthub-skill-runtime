package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderInputTemplateUsesQueryParameters(t *testing.T) {
	query, args := renderInputTemplate(
		"SELECT * FROM users WHERE name = '{{input.name}}' AND age >= {{input.age}}",
		map[string]any{"name": "Alice' OR 1=1 --", "age": 42},
	)

	assert.Equal(t, "SELECT * FROM users WHERE name = $1 AND age >= $2", query)
	require.Len(t, args, 2)
	assert.Equal(t, "Alice' OR 1=1 --", args[0])
	assert.Equal(t, 42, args[1])
}

func TestRenderInputTemplateContinuesExistingParameters(t *testing.T) {
	query, args := renderInputTemplate(
		"SELECT * FROM users WHERE account_id = $1 AND email = {{input.email}}",
		map[string]any{"email": "a@example.com"},
	)

	assert.Equal(t, "SELECT * FROM users WHERE account_id = $1 AND email = $2", query)
	require.Len(t, args, 1)
	assert.Equal(t, "a@example.com", args[0])
}

func TestMaxSQLParamIndex(t *testing.T) {
	assert.Equal(t, 12, maxSQLParamIndex("SELECT $2, $12, $1"))
	assert.Equal(t, 0, maxSQLParamIndex("SELECT 1"))
}

func TestDatasourceDSNEscapesCredentials(t *testing.T) {
	dsn := datasourceDSN(&datasourceConfig{
		Host:       "db.internal",
		Port:       5432,
		Database:   "customer",
		DBUser:     "user@example.com",
		DBPassword: "pa:ss@word",
	})

	assert.Equal(t, "postgres://user%40example.com:pa%3Ass%40word@db.internal:5432/customer", dsn)
}

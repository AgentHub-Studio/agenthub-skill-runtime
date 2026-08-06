package sql

import (
	"strings"
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
		map[string]any{"$1": "acct-1", "email": "a@example.com"},
	)

	assert.Equal(t, "SELECT * FROM users WHERE account_id = $1 AND email = $2", query)
	require.Len(t, args, 2)
	assert.Equal(t, "acct-1", args[0])
	assert.Equal(t, "a@example.com", args[1])
}

func TestRenderInputTemplateBindsExistingDollarParameters(t *testing.T) {
	query, args := renderInputTemplate(
		"SELECT * FROM orders WHERE id = $1",
		map[string]any{"$1": 123},
	)

	assert.Equal(t, "SELECT * FROM orders WHERE id = $1", query)
	require.Len(t, args, 1)
	assert.Equal(t, 123, args[0])
}

func TestRenderSQLQueryBindsParametersArray(t *testing.T) {
	query, args, err := renderSQLQuery(
		"SELECT * FROM orders WHERE id = $1 AND status = $2",
		map[string]any{"parameters": []any{123, "paid"}},
	)

	require.NoError(t, err)
	assert.Equal(t, "SELECT * FROM orders WHERE id = $1 AND status = $2", query)
	require.Len(t, args, 2)
	assert.Equal(t, 123, args[0])
	assert.Equal(t, "paid", args[1])
}

func TestRenderSQLQueryBindsParametersMap(t *testing.T) {
	query, args, err := renderSQLQuery(
		"SELECT * FROM orders WHERE id = $1 AND status = $2",
		map[string]any{"parameters": map[string]any{"$1": 123, "param2": "paid"}},
	)

	require.NoError(t, err)
	assert.Equal(t, "SELECT * FROM orders WHERE id = $1 AND status = $2", query)
	require.Len(t, args, 2)
	assert.Equal(t, 123, args[0])
	assert.Equal(t, "paid", args[1])
}

func TestRenderSQLQueryRequiresExistingDollarParameters(t *testing.T) {
	_, _, err := renderSQLQuery(
		"SELECT * FROM orders WHERE id = $1",
		map[string]any{},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing value for SQL parameter $1")
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

func FuzzRenderSQLQueryDoesNotInlineInputValues(f *testing.F) {
	f.Add("123", "Alice' OR 1=1 --")
	f.Add("A-42", "paid")
	f.Add("x", "Robert'); DROP TABLE orders; --")

	f.Fuzz(func(t *testing.T, orderID, email string) {
		orderID = safeSQLFuzzValue(orderID)
		email = safeSQLFuzzValue(email)

		query, args, err := renderSQLQuery(
			"SELECT * FROM orders WHERE id = $1 AND email = '{{input.email}}'",
			map[string]any{"$1": orderID, "email": email},
		)
		require.NoError(t, err)

		assert.Equal(t, "SELECT * FROM orders WHERE id = $1 AND email = $2", query)
		require.Len(t, args, 2)
		assert.Equal(t, orderID, args[0])
		assert.Equal(t, email, args[1])
		assert.NotContains(t, query, orderID)
		assert.NotContains(t, query, email)
	})
}

func safeSQLFuzzValue(value string) string {
	value = strings.ReplaceAll(value, "\x00", "")
	if len(value) > 96 {
		value = value[:96]
	}
	return "agenthub_sql_fuzz_" + value
}

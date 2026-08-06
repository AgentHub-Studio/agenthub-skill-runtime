//go:build integration

package sql

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor"
)

const sqlIntegrationDSNEnv = "AGENTHUB_SQL_INTEGRATION_DSN"
const sqlIntegrationAllowEnv = "AGENTHUB_SQL_INTEGRATION_ALLOW"
const sqlIntegrationTargetHostEnv = "AGENTHUB_SQL_INTEGRATION_TARGET_HOST"
const sqlIntegrationTargetPortEnv = "AGENTHUB_SQL_INTEGRATION_TARGET_PORT"

func TestIntegration_SQLToolExecutorKeepsInjectionPayloadOutOfQuery(t *testing.T) {
	if os.Getenv(sqlIntegrationAllowEnv) != "1" {
		t.Skipf("set %s=1 to allow writes to the isolated PostgreSQL test database", sqlIntegrationAllowEnv)
	}
	dsn := os.Getenv(sqlIntegrationDSNEnv)
	if dsn == "" {
		t.Skipf("set %s to an isolated PostgreSQL test database", sqlIntegrationDSNEnv)
	}

	ctx := context.Background()
	poolConfig, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	targetHost := os.Getenv(sqlIntegrationTargetHostEnv)
	if targetHost == "" {
		targetHost = poolConfig.ConnConfig.Host
	}
	targetPort := int(poolConfig.ConnConfig.Port)
	if rawTargetPort := os.Getenv(sqlIntegrationTargetPortEnv); rawTargetPort != "" {
		targetPort, err = strconv.Atoi(rawTargetPort)
		require.NoError(t, err)
	}

	const tenantID = "sqlinjectionintegration"
	schema := executor.TenantSchema(tenantID)
	const probeTable = "agenthub_sql_injection_probe"
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP TABLE IF EXISTS public."+probeTable)
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
	})

	_, err = pool.Exec(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s.data_source (
		id UUID PRIMARY KEY,
		type TEXT NOT NULL,
		host TEXT NOT NULL,
		port INTEGER NOT NULL,
		database TEXT NOT NULL,
		db_user TEXT NOT NULL,
		db_password TEXT NOT NULL
	)`, schema))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "CREATE TABLE public."+probeTable+" (name TEXT NOT NULL)")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "INSERT INTO public."+probeTable+" (name) VALUES ('alice'), ('bob')")
	require.NoError(t, err)

	datasourceID := uuid.New()
	_, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.data_source
		(id, type, host, port, database, db_user, db_password)
		VALUES ($1, 'POSTGRESQL', $2, $3, $4, $5, $6)`, schema),
		datasourceID,
		targetHost,
		targetPort,
		poolConfig.ConnConfig.Database,
		poolConfig.ConnConfig.User,
		poolConfig.ConnConfig.Password,
	)
	require.NoError(t, err)

	result, err := NewSQLToolExecutor(pool).Execute(ctx, executor.ExecutionContext{
		TenantID: tenantID,
		Config: map[string]any{
			"datasource_id": datasourceID.String(),
			"operation":     "SELECT",
			"query":         "SELECT name FROM public." + probeTable + " WHERE name = '{{input.name}}'",
		},
		Input: map[string]any{
			"name": "alice' OR 1=1 --",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Output["rows"], 0, "the payload must be bound as data, not alter the predicate")

	var rowCount int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM public."+probeTable).Scan(&rowCount))
	assert.Equal(t, 2, rowCount, "the query must not mutate the target table")
}

func TestIntegration_SQLToolExecutorDoesNotDialDNSResolvedMetadata(t *testing.T) {
	if os.Getenv(sqlIntegrationAllowEnv) != "1" {
		t.Skipf("set %s=1 to allow writes to the isolated PostgreSQL test database", sqlIntegrationAllowEnv)
	}
	dsn := os.Getenv(sqlIntegrationDSNEnv)
	if dsn == "" {
		t.Skipf("set %s to an isolated PostgreSQL test database", sqlIntegrationDSNEnv)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	const tenantID = "sqlssrfintegration"
	schema := executor.TenantSchema(tenantID)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
	})
	_, err = pool.Exec(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s.data_source (
		id UUID PRIMARY KEY,
		type TEXT NOT NULL,
		host TEXT NOT NULL,
		port INTEGER NOT NULL,
		database TEXT NOT NULL,
		db_user TEXT NOT NULL,
		db_password TEXT NOT NULL
	)`, schema))
	require.NoError(t, err)

	datasourceID := uuid.New()
	_, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.data_source
		(id, type, host, port, database, db_user, db_password)
		VALUES ($1, 'POSTGRESQL', 'attacker.example.test', 5432, 'ignored', 'ignored', 'ignored')`, schema), datasourceID)
	require.NoError(t, err)

	sqlExecutor := NewSQLToolExecutor(pool)
	sqlExecutor.resolveHost = func(_ context.Context, host string) ([]net.IPAddr, error) {
		require.Equal(t, "attacker.example.test", host)
		return []net.IPAddr{{IP: net.ParseIP("169.254.169.254")}}, nil
	}
	sqlExecutor.dialContext = func(context.Context, string, string) (net.Conn, error) {
		t.Fatal("a DNS result targeting metadata must be rejected before dialing")
		return nil, nil
	}

	_, err = sqlExecutor.Execute(ctx, executor.ExecutionContext{
		TenantID: tenantID,
		Config: map[string]any{
			"datasource_id": datasourceID.String(),
			"operation":     "SELECT",
			"query":         "SELECT 1",
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolves to blocked address")
}

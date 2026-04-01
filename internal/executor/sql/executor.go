package sql

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-skill-runtime/internal/executor"
)

// SQLToolExecutor executes SQL queries against a datasource registered in the tenant schema.
//
// Config fields:
//   - datasource_id: string (UUID of the data_source row)
//   - query: string (parameterized with $1, $2... or {{input.field}})
//   - operation: string (SELECT, INSERT, UPDATE, DELETE)
//   - max_rows: int (default 100)
type SQLToolExecutor struct {
	pool *pgxpool.Pool
}

// NewSQLToolExecutor creates an SQLToolExecutor backed by the given connection pool.
func NewSQLToolExecutor(pool *pgxpool.Pool) *SQLToolExecutor {
	return &SQLToolExecutor{pool: pool}
}

// GetToolType returns the tool type identifier.
func (e *SQLToolExecutor) GetToolType() string { return "SQL" }

// sqlConfig holds parsed configuration for a SQL tool.
type sqlConfig struct {
	DatasourceID string `json:"datasource_id"`
	Query        string `json:"query"`
	Operation    string `json:"operation"`
	MaxRows      int    `json:"max_rows"`
}

// datasourceConfig holds the target datasource connection details.
type datasourceConfig struct {
	Host       string
	Port       int
	Database   string
	DBUser     string
	DBPassword string
	Type       string
}

// Execute fetches datasource credentials and runs the SQL query.
func (e *SQLToolExecutor) Execute(ctx context.Context, ec executor.ExecutionContext) (*executor.Result, error) {
	cfg, err := parseSQLConfig(ec.Config)
	if err != nil {
		return nil, fmt.Errorf("sql executor: parse config: %w", err)
	}

	if cfg.DatasourceID == "" {
		return nil, fmt.Errorf("sql executor: datasource_id is required")
	}
	if cfg.Query == "" {
		return nil, fmt.Errorf("sql executor: query is required")
	}

	maxRows := cfg.MaxRows
	if maxRows <= 0 {
		maxRows = 100
	}

	ds, err := e.fetchDatasource(ctx, ec.TenantID, cfg.DatasourceID)
	if err != nil {
		return nil, fmt.Errorf("sql executor: fetch datasource: %w", err)
	}

	if !strings.EqualFold(ds.Type, "POSTGRESQL") {
		return nil, fmt.Errorf("sql executor: unsupported datasource type %q (only POSTGRESQL is supported)", ds.Type)
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s", ds.DBUser, ds.DBPassword, ds.Host, ds.Port, ds.Database)
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("sql executor: connect to datasource: %w", err)
	}
	defer conn.Close(ctx)

	// Render any {{input.field}} placeholders in the query.
	renderedQuery := renderInputTemplate(cfg.Query, ec.Input)

	operation := strings.ToUpper(cfg.Operation)
	if operation == "" || operation == "SELECT" {
		rows, err := conn.Query(ctx, renderedQuery)
		if err != nil {
			return nil, fmt.Errorf("sql executor: query error: %w", err)
		}
		defer rows.Close()

		results, err := collectRows(rows, maxRows)
		if err != nil {
			return nil, fmt.Errorf("sql executor: collect rows: %w", err)
		}

		return &executor.Result{Output: map[string]any{"rows": results}}, nil
	}

	// INSERT / UPDATE / DELETE
	tag, err := conn.Exec(ctx, renderedQuery)
	if err != nil {
		return nil, fmt.Errorf("sql executor: exec error: %w", err)
	}

	return &executor.Result{Output: map[string]any{"rowsAffected": tag.RowsAffected()}}, nil
}

// fetchDatasource loads the datasource credentials from ah_{tenantID}.data_source.
func (e *SQLToolExecutor) fetchDatasource(ctx context.Context, tenantID, datasourceID string) (*datasourceConfig, error) {
	schema := "ah_" + tenantID
	query := fmt.Sprintf(
		`SELECT host, port, database, db_user, db_password, type FROM %s.data_source WHERE id = $1`,
		schema,
	)

	var ds datasourceConfig
	row := e.pool.QueryRow(ctx, query, datasourceID)
	if err := row.Scan(&ds.Host, &ds.Port, &ds.Database, &ds.DBUser, &ds.DBPassword, &ds.Type); err != nil {
		return nil, fmt.Errorf("datasource %q not found in tenant %q: %w", datasourceID, tenantID, err)
	}
	return &ds, nil
}

// collectRows reads up to maxRows from pgx.Rows and returns them as a slice of maps.
func collectRows(rows pgx.Rows, maxRows int) ([]map[string]any, error) {
	fields := rows.FieldDescriptions()
	var result []map[string]any

	for rows.Next() {
		if len(result) >= maxRows {
			break
		}
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		row := make(map[string]any, len(fields))
		for i, fd := range fields {
			row[string(fd.Name)] = values[i]
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// parseSQLConfig decodes the executor config map into a sqlConfig struct.
func parseSQLConfig(raw map[string]any) (*sqlConfig, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	var cfg sqlConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}

// renderInputTemplate replaces {{input.key}} placeholders with values from input.
func renderInputTemplate(tmpl string, input map[string]any) string {
	pairs := make([]string, 0, len(input)*2)
	for k, v := range input {
		pairs = append(pairs, fmt.Sprintf("{{input.%s}}", k), fmt.Sprintf("%v", v))
	}
	return strings.NewReplacer(pairs...).Replace(tmpl)
}

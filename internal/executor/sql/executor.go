package sql

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
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
	pool        *pgxpool.Pool
	resolveHost func(context.Context, string) ([]net.IPAddr, error)
	dialContext func(context.Context, string, string) (net.Conn, error)
}

// NewSQLToolExecutor creates an SQLToolExecutor backed by the given connection pool.
func NewSQLToolExecutor(pool *pgxpool.Pool) *SQLToolExecutor {
	dialer := &net.Dialer{}
	return &SQLToolExecutor{
		pool:        pool,
		resolveHost: net.DefaultResolver.LookupIPAddr,
		dialContext: dialer.DialContext,
	}
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
func (e *SQLToolExecutor) Execute(ctx context.Context, ec executor.ExecutionContext) (result *executor.Result, retErr error) {
	cfg, err := parseSQLConfig(ec.Config)
	if err != nil {
		return nil, fmt.Errorf("sql executor: parse config: %w", err)
	}

	if cfg.DatasourceID == "" {
		return nil, executor.Permanentf("sql executor: datasource not configured — add a datasource_id to the tool config")
	}
	if cfg.Query == "" {
		return nil, executor.Permanentf("sql executor: query is required")
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

	dsn := datasourceDSN(ds)
	connConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("sql executor: parse datasource connection: %w", err)
	}
	resolvedHost, err := e.resolveDatasourceHost(ctx, connConfig.Host)
	if err != nil {
		return nil, err
	}
	// pgx resolves a hostname before invoking DialFunc. Pinning every connection
	// attempt to the checked IP therefore prevents a later resolver call from
	// changing an approved hostname into a loopback or metadata destination.
	connConfig.Host = resolvedHost
	for _, fallback := range connConfig.Fallbacks {
		fallback.Host = resolvedHost
	}
	connConfig.DialFunc = e.dialContext
	conn, err := pgx.ConnectConfig(ctx, connConfig)
	if err != nil {
		return nil, fmt.Errorf("sql executor: connect to datasource: %w", err)
	}
	defer func() {
		if err := conn.Close(ctx); err != nil && retErr == nil {
			retErr = fmt.Errorf("sql executor: close datasource connection: %w", err)
		}
	}()

	renderedQuery, args, err := renderSQLQuery(cfg.Query, ec.Input)
	if err != nil {
		return nil, executor.Permanentf("sql executor: %w", err)
	}

	operation := strings.ToUpper(cfg.Operation)
	if operation == "" || operation == "SELECT" {
		rows, err := conn.Query(ctx, renderedQuery, args...)
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
	tag, err := conn.Exec(ctx, renderedQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("sql executor: exec error: %w", err)
	}

	return &executor.Result{Output: map[string]any{"rowsAffected": tag.RowsAffected()}}, nil
}

// fetchDatasource loads the datasource credentials from ah_{tenantID}.data_source.
func (e *SQLToolExecutor) fetchDatasource(ctx context.Context, tenantID, datasourceID string) (*datasourceConfig, error) {
	schema := executor.TenantSchema(tenantID)
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

func datasourceDSN(ds *datasourceConfig) string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(ds.DBUser, ds.DBPassword),
		Host:   net.JoinHostPort(ds.Host, strconv.Itoa(ds.Port)),
		Path:   ds.Database,
	}
	return u.String()
}

// resolveDatasourceHost resolves a hostname once and rejects loopback and
// link-local results before pgx receives a connection host. RFC1918 addresses
// remain valid because datasource connections may use a VPN.
func (e *SQLToolExecutor) resolveDatasourceHost(ctx context.Context, host string) (string, error) {
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedDatasourceIP(ip) {
			return "", fmt.Errorf("sql executor: datasource resolves to blocked address %s", ip)
		}
		return ip.String(), nil
	}

	addresses, err := e.resolveHost(ctx, host)
	if err != nil {
		return "", fmt.Errorf("sql executor: resolve datasource host %q: %w", host, err)
	}
	if len(addresses) == 0 {
		return "", fmt.Errorf("sql executor: datasource host %q resolved without addresses", host)
	}

	for _, address := range addresses {
		if isBlockedDatasourceIP(address.IP) {
			return "", fmt.Errorf("sql executor: datasource host %q resolves to blocked address %s", host, address.IP)
		}
	}

	return addresses[0].IP.String(), nil
}

func isBlockedDatasourceIP(ip net.IP) bool {
	return ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
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
	if err := validateDatasourceIDAliasContract(raw); err != nil {
		return nil, err
	}
	config := make(map[string]any, len(raw)+1)
	for key, value := range raw {
		config[key] = value
	}
	if _, hasSnake := config["datasource_id"]; !hasSnake {
		if camel, hasCamel := config["dataSourceId"]; hasCamel {
			config["datasource_id"] = camel
		}
	}

	data, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	var cfg sqlConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}

func validateDatasourceIDAliasContract(raw map[string]any) error {
	snake, hasSnake := raw["datasource_id"]
	camel, hasCamel := raw["dataSourceId"]
	if !hasSnake || !hasCamel {
		return nil
	}
	snakeID, snakeOK := snake.(string)
	camelID, camelOK := camel.(string)
	if !snakeOK || !camelOK {
		return conflictingDatasourceIDAliasesError()
	}
	snakeUUID, snakeErr := uuid.Parse(strings.TrimSpace(snakeID))
	camelUUID, camelErr := uuid.Parse(strings.TrimSpace(camelID))
	if snakeErr == nil && camelErr == nil && snakeUUID == camelUUID {
		return nil
	}
	if strings.TrimSpace(snakeID) == strings.TrimSpace(camelID) {
		return nil
	}
	return conflictingDatasourceIDAliasesError()
}

func conflictingDatasourceIDAliasesError() error {
	return fmt.Errorf("conflicting SQL config aliases datasource_id and dataSourceId")
}

var (
	inputPlaceholderRE = regexp.MustCompile(`'?\{\{input\.([A-Za-z0-9_]+)\}\}'?`)
	sqlParamRE         = regexp.MustCompile(`\$([1-9][0-9]*)`)
)

// renderSQLQuery builds the final SQL text and pgx argument list from static
// tool config plus caller input. The SQL text itself is never taken from input.
func renderSQLQuery(tmpl string, input map[string]any) (string, []any, error) {
	existingParams := sqlParamIndexes(tmpl)
	maxExistingParam := 0
	args := make([]any, 0, len(existingParams))
	if len(existingParams) > 0 {
		maxExistingParam = maxSQLParamIndex(tmpl)
		positioned := make([]any, maxExistingParam)
		for i := 1; i <= maxExistingParam; i++ {
			if !existingParams[i] {
				return "", nil, fmt.Errorf("SQL parameters must be contiguous starting at $1; missing $%d", i)
			}
			value, ok := lookupSQLParamInput(input, i)
			if !ok {
				return "", nil, fmt.Errorf("missing value for SQL parameter $%d", i)
			}
			positioned[i-1] = value
		}
		args = append(args, positioned...)
	}

	rendered, templateArgs := renderInputPlaceholders(tmpl, input, maxExistingParam)
	args = append(args, templateArgs...)
	return rendered, args, nil
}

// renderInputTemplate replaces {{input.key}} placeholders with pgx parameters.
func renderInputTemplate(tmpl string, input map[string]any) (string, []any) {
	rendered, args, err := renderSQLQuery(tmpl, input)
	if err != nil {
		return tmpl, nil
	}
	return rendered, args
}

func renderInputPlaceholders(tmpl string, input map[string]any, initialParam int) (string, []any) {
	args := make([]any, 0)
	nextParam := initialParam

	rendered := inputPlaceholderRE.ReplaceAllStringFunc(tmpl, func(match string) string {
		parts := inputPlaceholderRE.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		args = append(args, input[parts[1]])
		nextParam++
		return "$" + strconv.Itoa(nextParam)
	})

	return rendered, args
}

func sqlParamIndexes(query string) map[int]bool {
	indexes := make(map[int]bool)
	for _, match := range sqlParamRE.FindAllStringSubmatch(query, -1) {
		if len(match) != 2 {
			continue
		}
		n, err := strconv.Atoi(match[1])
		if err == nil {
			indexes[n] = true
		}
	}
	return indexes
}

func maxSQLParamIndex(query string) int {
	max := 0
	for _, match := range sqlParamRE.FindAllStringSubmatch(query, -1) {
		if len(match) != 2 {
			continue
		}
		n, err := strconv.Atoi(match[1])
		if err == nil && n > max {
			max = n
		}
	}
	return max
}

func lookupSQLParamInput(input map[string]any, n int) (any, bool) {
	if input == nil {
		return nil, false
	}
	if value, ok := lookupSQLParamInMap(input, n); ok {
		return value, true
	}
	for _, key := range []string{"parameters", "args"} {
		if value, ok := input[key]; ok {
			if paramValue, found := lookupSQLParamInValue(value, n); found {
				return paramValue, true
			}
		}
	}
	return nil, false
}

func lookupSQLParamInValue(value any, n int) (any, bool) {
	switch typed := value.(type) {
	case []any:
		if n > 0 && n <= len(typed) {
			return typed[n-1], true
		}
	case map[string]any:
		return lookupSQLParamInMap(typed, n)
	}
	return nil, false
}

func lookupSQLParamInMap(values map[string]any, n int) (any, bool) {
	for _, key := range sqlParamInputKeys(n) {
		if value, ok := values[key]; ok {
			return value, true
		}
	}
	return nil, false
}

func sqlParamInputKeys(n int) []string {
	number := strconv.Itoa(n)
	return []string{
		"$" + number,
		number,
		"arg" + number,
		"param" + number,
	}
}

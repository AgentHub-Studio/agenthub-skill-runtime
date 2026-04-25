package executor

import "github.com/jackc/pgx/v5"

// TenantSchema returns the quoted PostgreSQL schema name for a tenant.
func TenantSchema(tenantID string) string {
	return pgx.Identifier{"ah_" + tenantID}.Sanitize()
}

package dev.cezar.agenthub.skillruntime.shared.multitenant;

import lombok.Getter;

import java.util.Objects;

/**
 * Holds tenant-specific context information for a request.
 * <p>
 * Provides the tenant ID, optional user ID, and computes the tenant-specific
 * PostgreSQL schema name using the {@code ah_} prefix convention.
 * </p>
 */
@Getter
public class TenantContext {

    private final String tenantId;
    private final String userId;
    private String schemaName;

    /**
     * Creates a TenantContext with tenant ID only.
     *
     * @param tenantId the tenant identifier (UUID string)
     */
    public TenantContext(String tenantId) {
        this(tenantId, null);
    }

    /**
     * Creates a TenantContext with tenant ID and user ID.
     *
     * @param tenantId the tenant identifier (UUID string)
     * @param userId   the user identifier (UUID string), may be null
     */
    public TenantContext(String tenantId, String userId) {
        this.tenantId = tenantId;
        this.userId = userId;
    }

    /**
     * Returns the database schema name for this tenant.
     * <p>
     * Returns the {@link MultiTenant#DEFAULT_SCHEMA} when the tenant ID equals "public",
     * otherwise returns {@code ah_{tenantId}} preserving UUID hyphens.
     * </p>
     *
     * @return the tenant-specific schema name
     */
    public String getSchemaName() {
        Objects.requireNonNull(this.tenantId, "Tenant ID cannot be null");
        if (this.schemaName == null) {
            this.schemaName = MultiTenant.DEFAULT_SCHEMA.equalsIgnoreCase(this.tenantId)
                    ? this.tenantId
                    : MultiTenant.SCHEMA_PREFIX + this.tenantId;
        }
        return this.schemaName;
    }

    @Override
    public String toString() {
        return "TenantContext{tenantId='" + tenantId + "', userId='" + userId + "', schemaName='" + getSchemaName() + "'}";
    }
}

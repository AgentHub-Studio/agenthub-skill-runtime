package dev.cezar.agenthub.skillruntime.shared.multitenant;

/**
 * Centralized constants for multi-tenant schema management.
 * <p>
 * Defines the default schema and the prefix used to construct tenant-specific
 * database schemas in a multi-tenant environment.
 * </p>
 */
public class MultiTenant {

    /**
     * Default database schema used when no tenant is identified.
     */
    public static final String DEFAULT_SCHEMA = "public";

    /**
     * Prefix for tenant-specific database schemas.
     * Schema names follow the pattern: ah_{tenantId}
     */
    public static final String SCHEMA_PREFIX = "ah_";
}

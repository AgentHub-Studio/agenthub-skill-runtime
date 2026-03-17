package dev.cezar.agenthub.skillruntime.multitenant;

import lombok.AccessLevel;
import lombok.NoArgsConstructor;
import reactor.core.publisher.Mono;
import reactor.util.context.Context;

import java.util.Objects;

/**
 * Manages storage and retrieval of {@link TenantContext} for both thread-local
 * (blocking) and Reactor context (reactive) access patterns.
 * <p>
 * In reactive pipelines, use {@link #withTenantContext(TenantContext)} to propagate
 * the context, and {@link #getContextFromReactor()} to read it.
 * </p>
 */
@NoArgsConstructor(access = AccessLevel.PRIVATE)
public class TenantContextHolder {

    /**
     * Key used to store the {@link TenantContext} in the Reactor Context.
     */
    public static final String TENANT_CONTEXT_KEY = "TENANT_CONTEXT_KEY";

    /**
     * Thread-local storage for blocking access patterns.
     */
    private static final ThreadLocal<TenantContext> CONTEXT = new ThreadLocal<>();

    /**
     * Stores the given {@link TenantContext} in the current thread's local storage.
     *
     * @param tenantContext the context to store
     */
    public static void setContext(TenantContext tenantContext) {
        CONTEXT.set(tenantContext);
    }

    /**
     * Retrieves the current {@link TenantContext}, checking thread-local first,
     * then falling back to the Reactor context via a blocking call.
     *
     * @return the current TenantContext, or {@code null} if not set
     */
    public static TenantContext getContext() {
        TenantContext tenantContext = CONTEXT.get();
        if (tenantContext == null) {
            try {
                return Mono.deferContextual(ctx ->
                        Mono.justOrEmpty(ctx.<TenantContext>getOrEmpty(TENANT_CONTEXT_KEY))
                ).block();
            } catch (Exception e) {
                return null;
            }
        }
        return tenantContext;
    }

    /**
     * Removes the {@link TenantContext} from thread-local storage to prevent memory leaks.
     */
    public static void clear() {
        CONTEXT.remove();
    }

    /**
     * Creates a Reactor {@link Context} containing the given {@link TenantContext}.
     *
     * @param tenantContext the tenant context to include
     * @return a Reactor Context keyed by {@link #TENANT_CONTEXT_KEY}
     */
    public static Context withTenantContext(TenantContext tenantContext) {
        return Context.of(TENANT_CONTEXT_KEY, tenantContext);
    }

    /**
     * Creates a Reactor {@link Context} from the current thread's {@link TenantContext}.
     *
     * @return a Reactor Context containing the current thread's TenantContext
     * @throws NullPointerException if no TenantContext is set on the current thread
     */
    public static Context withTenantContext() {
        return Context.of(TENANT_CONTEXT_KEY, Objects.requireNonNull(TenantContextHolder.getContext()));
    }

    /**
     * Retrieves the {@link TenantContext} reactively from the Reactor Context.
     *
     * @return a {@link Mono} emitting the TenantContext, or empty if not present
     */
    public static Mono<TenantContext> getContextFromReactor() {
        return Mono.deferContextual(ctx ->
                Mono.justOrEmpty(ctx.<TenantContext>getOrEmpty(TENANT_CONTEXT_KEY))
        );
    }
}

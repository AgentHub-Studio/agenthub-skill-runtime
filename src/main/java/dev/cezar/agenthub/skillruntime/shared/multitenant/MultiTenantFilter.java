package dev.cezar.agenthub.skillruntime.shared.multitenant;

import lombok.extern.java.Log;
import org.springframework.stereotype.Component;
import org.springframework.web.server.ServerWebExchange;
import org.springframework.web.server.WebFilter;
import org.springframework.web.server.WebFilterChain;
import reactor.core.publisher.Mono;

/**
 * WebFilter that extracts tenant information from the Authorization header and
 * propagates it through both ThreadLocal and Reactor Context for the duration
 * of each request.
 * <p>
 * When no Authorization header is present, defaults to
 * {@link MultiTenant#DEFAULT_SCHEMA}.
 * </p>
 */
@Log
@Component
public class MultiTenantFilter implements WebFilter {

    private static final String BEARER_PREFIX = "Bearer ";
    private static final String AUTHORIZATION = "Authorization";

    @Override
    public Mono<Void> filter(ServerWebExchange exchange, WebFilterChain chain) {
        final var authHeader = exchange.getRequest().getHeaders().getFirst(AUTHORIZATION);
        final var tenantId = resolveTenantId(authHeader);
        final var userId = resolveUserId(authHeader);
        final var tc = new TenantContext(tenantId, userId);

        TenantContextHolder.setContext(tc);
        log.info("Tenant context set: " + tc);

        return chain.filter(exchange)
                .contextWrite(TenantContextHolder.withTenantContext(tc))
                .contextWrite(ctx -> {
                    var c = ctx.put("schema", tc.getSchemaName())
                               .put("tenantId", tc.getTenantId());
                    if (tc.getUserId() != null) {
                        c = c.put("userId", tc.getUserId());
                    }
                    return c;
                })
                .doOnSubscribe(subscription -> TenantContextHolder.setContext(tc))
                .doFinally(signalType -> TenantContextHolder.clear());
    }

    private String resolveTenantId(String authorizationHeader) {
        if (authorizationHeader != null && authorizationHeader.startsWith(BEARER_PREFIX)) {
            final var token = authorizationHeader.substring(BEARER_PREFIX.length());
            String tenantId = TokenExtractorUtils.getTenantIdFromToken(token);
            return tenantId != null ? tenantId : MultiTenant.DEFAULT_SCHEMA;
        }
        return MultiTenant.DEFAULT_SCHEMA;
    }

    private String resolveUserId(String authorizationHeader) {
        if (authorizationHeader != null && authorizationHeader.startsWith(BEARER_PREFIX)) {
            final var token = authorizationHeader.substring(BEARER_PREFIX.length());
            return TokenExtractorUtils.getUserIdFromToken(token);
        }
        return null;
    }
}

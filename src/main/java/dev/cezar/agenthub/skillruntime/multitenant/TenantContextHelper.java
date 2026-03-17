package dev.cezar.agenthub.skillruntime.multitenant;

import lombok.AccessLevel;
import lombok.NoArgsConstructor;
import reactor.core.publisher.Mono;

import java.util.UUID;

/**
 * Helper for reactively accessing tenant context information.
 * <p>
 * Reads tenant and user identifiers from the Reactor context,
 * which must have been populated via {@link TenantContextHolder#withTenantContext(TenantContext)}
 * or the {@code contextWrite} mechanism in {@code SkillController}.
 * </p>
 *
 * @since 1.0.0
 */
@NoArgsConstructor(access = AccessLevel.PRIVATE)
public class TenantContextHelper {

    /**
     * Retrieves the tenant ID from the reactive context.
     *
     * @return {@link Mono} emitting the tenant UUID, or an error if unavailable or invalid
     */
    public static Mono<UUID> getTenantId() {
        return Mono.deferContextual(ctx -> {
            String tenantId = ctx.getOrDefault("tenantId", null);
            if (tenantId == null) {
                return Mono.error(new IllegalStateException("TenantId not available in context"));
            }
            try {
                return Mono.just(UUID.fromString(tenantId));
            } catch (IllegalArgumentException e) {
                return Mono.error(new IllegalStateException("Invalid tenantId: " + tenantId));
            }
        });
    }

    /**
     * Retrieves the user ID from the reactive context.
     *
     * @return {@link Mono} emitting the user UUID, or an error if unavailable or invalid
     */
    public static Mono<UUID> getUserId() {
        return Mono.deferContextual(ctx -> {
            String userId = ctx.getOrDefault("userId", null);
            if (userId == null) {
                return Mono.error(new IllegalStateException("UserId not available in context"));
            }
            try {
                return Mono.just(UUID.fromString(userId));
            } catch (IllegalArgumentException e) {
                return Mono.error(new IllegalStateException("Invalid userId: " + userId));
            }
        });
    }
}

package dev.cezar.agenthub.skillruntime.multitenant;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.http.HttpHeaders;
import org.springframework.http.server.reactive.ServerHttpRequest;
import org.springframework.web.server.ServerWebExchange;
import org.springframework.web.server.WebFilterChain;
import reactor.core.publisher.Mono;
import reactor.test.StepVerifier;

import java.nio.charset.StandardCharsets;
import java.util.Base64;

import static org.mockito.Mockito.when;

@ExtendWith(MockitoExtension.class)
@DisplayName("MultiTenantFilter - Extração de tenant do JWT")
class MultiTenantFilterTest {

    @Mock
    private ServerWebExchange exchange;

    @Mock
    private ServerHttpRequest request;

    @Mock
    private HttpHeaders headers;

    @Mock
    private WebFilterChain chain;

    private static String buildTestJwt(String payloadJson) {
        String header = Base64.getUrlEncoder().withoutPadding()
                .encodeToString("{\"alg\":\"HS256\",\"typ\":\"JWT\"}".getBytes(StandardCharsets.UTF_8));
        String payload = Base64.getUrlEncoder().withoutPadding()
                .encodeToString(payloadJson.getBytes(StandardCharsets.UTF_8));
        return header + "." + payload + ".fakesig";
    }

    @Test
    @DisplayName("Deve propagar tenantId no contexto Reactor quando header Authorization está presente")
    void shouldPropagateTenantIdWhenAuthorizationHeaderPresent() {
        // Given
        String tenantId = "test-uuid";
        String jwt = buildTestJwt("{\"iss\":\"http://keycloak/realms/" + tenantId + "\",\"sub\":\"user-sub\"}");

        when(exchange.getRequest()).thenReturn(request);
        when(request.getHeaders()).thenReturn(headers);
        when(headers.getFirst("Authorization")).thenReturn("Bearer " + jwt);

        // Chain returns a Mono that reads tenantId from Reactor context to verify propagation
        when(chain.filter(exchange)).thenReturn(
                Mono.deferContextual(ctx -> {
                    String extractedTenantId = ctx.getOrDefault("tenantId", null);
                    if (!tenantId.equals(extractedTenantId)) {
                        return Mono.error(new AssertionError(
                                "Expected tenantId=" + tenantId + " but got " + extractedTenantId));
                    }
                    return Mono.empty();
                })
        );

        MultiTenantFilter filter = new MultiTenantFilter();

        // When & Then
        StepVerifier.create(filter.filter(exchange, chain))
                .verifyComplete();
    }

    @Test
    @DisplayName("Deve usar DEFAULT_SCHEMA quando nenhum header Authorization está presente")
    void shouldUseDefaultSchemaWhenNoAuthorizationHeader() {
        // Given
        when(exchange.getRequest()).thenReturn(request);
        when(request.getHeaders()).thenReturn(headers);
        when(headers.getFirst("Authorization")).thenReturn(null);

        // Chain verifies that schema defaults to "public"
        when(chain.filter(exchange)).thenReturn(
                Mono.deferContextual(ctx -> {
                    String schema = ctx.getOrDefault("schema", null);
                    if (!MultiTenant.DEFAULT_SCHEMA.equals(schema)) {
                        return Mono.error(new AssertionError(
                                "Expected schema=" + MultiTenant.DEFAULT_SCHEMA + " but got " + schema));
                    }
                    return Mono.empty();
                })
        );

        MultiTenantFilter filter = new MultiTenantFilter();

        // When & Then
        StepVerifier.create(filter.filter(exchange, chain))
                .verifyComplete();
    }
}

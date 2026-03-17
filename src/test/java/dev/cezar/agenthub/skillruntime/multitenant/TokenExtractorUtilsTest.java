package dev.cezar.agenthub.skillruntime.multitenant;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.nio.charset.StandardCharsets;
import java.util.Base64;

import static org.assertj.core.api.Assertions.assertThat;

@DisplayName("TokenExtractorUtils - JWT claim extraction")
class TokenExtractorUtilsTest {

    /**
     * Builds a test JWT with the given payload JSON.
     * The header and signature are dummy values (no real signing).
     */
    private static String buildTestJwt(String payloadJson) {
        String header = Base64.getUrlEncoder().withoutPadding()
                .encodeToString("{\"alg\":\"HS256\",\"typ\":\"JWT\"}".getBytes(StandardCharsets.UTF_8));
        String payload = Base64.getUrlEncoder().withoutPadding()
                .encodeToString(payloadJson.getBytes(StandardCharsets.UTF_8));
        return header + "." + payload + ".fakesig";
    }

    @Test
    @DisplayName("Deve extrair tenantId do claim iss de um JWT válido")
    void shouldExtractTenantIdFromValidJwt() {
        String jwt = buildTestJwt("{\"iss\":\"http://keycloak/realms/test-tenant\",\"sub\":\"user-uuid\"}");

        String tenantId = TokenExtractorUtils.getTenantIdFromToken(jwt);

        assertThat(tenantId).isEqualTo("test-tenant");
    }

    @Test
    @DisplayName("Deve extrair userId do claim sub de um JWT válido")
    void shouldExtractUserIdFromValidJwt() {
        String jwt = buildTestJwt("{\"iss\":\"http://keycloak/realms/test-tenant\",\"sub\":\"user-uuid\"}");

        String userId = TokenExtractorUtils.getUserIdFromToken(jwt);

        assertThat(userId).isEqualTo("user-uuid");
    }

    @Test
    @DisplayName("Deve retornar null para token inválido ao extrair tenantId")
    void shouldReturnNullForInvalidTokenWhenExtractingTenantId() {
        String tenantId = TokenExtractorUtils.getTenantIdFromToken("not.a.valid.jwt.token");

        // Invalid base64 or missing claims should not throw, just return null
        assertThat(tenantId).isNull();
    }

    @Test
    @DisplayName("Deve retornar null para token inválido ao extrair userId")
    void shouldReturnNullForInvalidTokenWhenExtractingUserId() {
        String userId = TokenExtractorUtils.getUserIdFromToken("invalid");

        assertThat(userId).isNull();
    }

    @Test
    @DisplayName("Deve retornar null quando iss não contém /realms/")
    void shouldReturnNullWhenIssHasNoRealmSegment() {
        String jwt = buildTestJwt("{\"iss\":\"http://keycloak/no-realm-here\",\"sub\":\"user-uuid\"}");

        String tenantId = TokenExtractorUtils.getTenantIdFromToken(jwt);

        assertThat(tenantId).isNull();
    }

    @Test
    @DisplayName("Deve extrair tenantId com UUID como realm name")
    void shouldExtractUuidTenantIdFromJwt() {
        String uuid = "123e4567-e89b-12d3-a456-426614174000";
        String jwt = buildTestJwt("{\"iss\":\"http://keycloak.cezar.dev/realms/" + uuid + "\",\"sub\":\"sub-id\"}");

        String tenantId = TokenExtractorUtils.getTenantIdFromToken(jwt);

        assertThat(tenantId).isEqualTo(uuid);
    }
}

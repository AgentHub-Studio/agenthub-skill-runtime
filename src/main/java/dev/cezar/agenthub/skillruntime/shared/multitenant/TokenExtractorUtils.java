package dev.cezar.agenthub.skillruntime.shared.multitenant;

import com.fasterxml.jackson.databind.ObjectMapper;
import lombok.AccessLevel;
import lombok.NoArgsConstructor;
import lombok.extern.java.Log;

import java.nio.charset.StandardCharsets;
import java.util.Base64;
import java.util.regex.Pattern;

/**
 * Utility class for extracting tenant and user information from JWT tokens.
 * <p>
 * Parses the JWT payload (base64url-encoded) without full signature verification,
 * extracting the {@code iss} claim to derive the tenant ID and the {@code sub}
 * claim for the user ID.
 * </p>
 */
@Log
@NoArgsConstructor(access = AccessLevel.PRIVATE)
public class TokenExtractorUtils {

    private static final ObjectMapper OBJECT_MAPPER = new ObjectMapper();
    private static final Pattern REALM_PATTERN = Pattern.compile("/realms/([^/]+)");

    /**
     * Extracts the tenant ID from a JWT by parsing the {@code iss} claim.
     * <p>
     * Expects an issuer URL in the format: {@code http://keycloak/realms/{tenantId}}
     * </p>
     *
     * @param token the raw JWT string (header.payload.signature)
     * @return the tenant ID extracted from the realm segment, or {@code null} if not found
     */
    public static String getTenantIdFromToken(String token) {
        try {
            var jsonNode = parseJwtPayload(token);
            var iss = jsonNode.path("iss").asText();
            var matcher = REALM_PATTERN.matcher(iss);
            if (matcher.find()) {
                return matcher.group(1);
            }
        } catch (Exception e) {
            log.info("Error extracting tenantId from JWT: " + e.getMessage());
        }
        return null;
    }

    /**
     * Extracts the user ID from a JWT by reading the {@code sub} claim.
     *
     * @param token the raw JWT string (header.payload.signature)
     * @return the user ID (subject), or {@code null} if not found
     */
    public static String getUserIdFromToken(String token) {
        try {
            var jsonNode = parseJwtPayload(token);
            return jsonNode.path("sub").asText(null);
        } catch (Exception e) {
            log.info("Error extracting userId from JWT: " + e.getMessage());
        }
        return null;
    }

    private static com.fasterxml.jackson.databind.JsonNode parseJwtPayload(String token) throws Exception {
        var parts = token.split("\\.");
        if (parts.length < 2) {
            throw new IllegalArgumentException("Invalid JWT token");
        }
        var payloadBase64Url = parts[1];
        var payloadBytes = Base64.getUrlDecoder().decode(payloadBase64Url);
        var payloadJson = new String(payloadBytes, StandardCharsets.UTF_8);
        return OBJECT_MAPPER.readTree(payloadJson);
    }
}

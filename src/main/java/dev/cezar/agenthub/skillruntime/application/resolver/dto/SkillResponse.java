package dev.cezar.agenthub.skillruntime.application.resolver.dto;

import com.fasterxml.jackson.databind.JsonNode;

import java.time.OffsetDateTime;
import java.util.UUID;

/**
 * Response DTO for Skill from backend API.
 * Matches the structure from agenthub-backend SkillResponse.
 */
public record SkillResponse(
        UUID id,
        UUID tenantId,
        String name,
        String slug,
        String description,
        String category,
        String version,
        JsonNode inputSchema,
        JsonNode outputSchema,
        String status,
        OffsetDateTime createdAt,
        OffsetDateTime updatedAt
) {
}

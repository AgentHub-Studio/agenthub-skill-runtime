package dev.cezar.agenthub.skillruntime.resolver.dto;

import java.time.LocalDateTime;
import java.util.List;
import java.util.Map;
import java.util.UUID;

/**
 * Response DTO for Tool from backend API.
 * Matches the structure from agenthub-backend ToolDTO.
 */
public record ToolResponse(
        UUID id,
        String name,
        String description,
        String type,
        String language,
        Map<String, Object> config,
        List<String> labels,
        LocalDateTime createdAt,
        LocalDateTime updatedAt
) {
}

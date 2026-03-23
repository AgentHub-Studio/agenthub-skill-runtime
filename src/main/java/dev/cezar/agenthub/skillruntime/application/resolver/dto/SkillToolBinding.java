package dev.cezar.agenthub.skillruntime.application.resolver.dto;

import java.time.OffsetDateTime;
import java.util.UUID;

/**
 * DTO for Skill-Tool binding from backend API.
 * Matches the structure from agenthub-backend SkillToolEntity.
 */
public record SkillToolBinding(
        UUID id,
        UUID skillId,
        UUID toolId,
        Integer priority,
        OffsetDateTime createdAt
) {
}

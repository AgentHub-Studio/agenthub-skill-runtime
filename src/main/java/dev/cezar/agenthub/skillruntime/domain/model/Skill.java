package dev.cezar.agenthub.skillruntime.domain.model;

import java.util.Map;
import java.util.UUID;

/**
 * Skill representa uma capacidade abstrata que pode ser implementada por múltiplas tools.
 * <p>
 * Exemplo: A skill "document-search" pode ter implementações via:
 * - pgvector (PostgreSQL)
 * - Elasticsearch
 * - Pinecone
 * </p>
 *
 * @param id          ID da skill
 * @param slug        Identificador único (eg. "document-search", "http-request")
 * @param name        Nome legível
 * @param category    Categoria (RAG, DATA, INTEGRATION, etc.)
 * @param description Descrição da skill
 * @param status      Status (ACTIVE, DEPRECATED, INACTIVE)
 * @param schema      Schema JSON dos inputs esperados
 * @since 1.0.0
 */
public record Skill(
        UUID id,
        String slug,
        String name,
        String category,
        String description,
        String status,
        Map<String, Object> schema
) {}

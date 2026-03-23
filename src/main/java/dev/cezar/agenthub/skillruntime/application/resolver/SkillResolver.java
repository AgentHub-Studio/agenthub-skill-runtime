package dev.cezar.agenthub.skillruntime.application.resolver;

import dev.cezar.agenthub.skillruntime.domain.model.Skill;
import dev.cezar.agenthub.skillruntime.domain.model.Tool;
import dev.cezar.agenthub.skillruntime.application.resolver.dto.SkillResponse;
import dev.cezar.agenthub.skillruntime.application.resolver.dto.SkillToolBinding;
import dev.cezar.agenthub.skillruntime.application.resolver.dto.ToolResponse;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.core.ParameterizedTypeReference;
import org.springframework.data.domain.Page;
import org.springframework.stereotype.Component;
import org.springframework.web.reactive.function.client.WebClient;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Mono;

import java.util.Comparator;
import java.util.List;
import java.util.Map;
import java.util.UUID;

/**
 * Resolve skills para tools concretas.
 * <p>
 * Consulta o backend para obter bindings skill → tool e seleciona
 * a tool mais apropriada baseado em prioridade e status.
 * </p>
 *
 * @since 1.0.0
 */
@Slf4j
@Component
public class SkillResolver {

    private final WebClient backendClient;

    public SkillResolver(@Value("${agenthub.backend.url}") String backendUrl) {
        this.backendClient = WebClient.builder()
                .baseUrl(backendUrl)
                .build();
    }

    /**
     * Resolve skill por slug para a tool concreta.
     * <p>
     * Lógica de seleção:
     * 1. Busca skill por slug
     * 2. Busca tools vinculadas à skill
     * 3. Filtra apenas tools ativas
     * 4. Ordena por prioridade (menor = melhor)
     * 5. Retorna a primeira
     * </p>
     *
     * @param tenantId  ID do tenant
     * @param skillSlug slug da skill
     * @return {@link Mono} com a tool selecionada
     */
    public Mono<Tool> resolveBySlug(UUID tenantId, String skillSlug) {
        log.debug("Resolving skill: tenantId={}, skillSlug={}", tenantId, skillSlug);

        return findSkillBySlug(tenantId, skillSlug)
                .flatMap(skill -> findToolsForSkill(tenantId, skill.id()))
                .flatMap(tools -> {
                    if (tools.isEmpty()) {
                        return Mono.error(new SkillResolutionException(
                                "No active tools found for skill: " + skillSlug));
                    }

                    // Seleciona tool com menor prioridade (mais alta precedência)
                    Tool selectedTool = tools.stream()
                            .filter(Tool::isActive)
                            .min(Comparator.comparing(Tool::priority))
                            .orElseThrow(() -> new SkillResolutionException(
                                    "No active tools found for skill: " + skillSlug));

                    log.info("Resolved skill '{}' to tool: {} (type: {}, priority: {})",
                            skillSlug, selectedTool.name(), selectedTool.type(), selectedTool.priority());

                    return Mono.just(selectedTool);
                });
    }

    /**
     * Resolve skill por ID para a tool concreta.
     *
     * @param tenantId ID do tenant
     * @param skillId  ID da skill
     * @return {@link Mono} com a tool selecionada
     */
    public Mono<Tool> resolveById(UUID tenantId, UUID skillId) {
        log.debug("Resolving skill: tenantId={}, skillId={}", tenantId, skillId);

        return findToolsForSkill(tenantId, skillId)
                .flatMap(tools -> {
                    if (tools.isEmpty()) {
                        return Mono.error(new SkillResolutionException(
                                "No active tools found for skill ID: " + skillId));
                    }

                    Tool selectedTool = tools.stream()
                            .filter(Tool::isActive)
                            .min(Comparator.comparing(Tool::priority))
                            .orElseThrow(() -> new SkillResolutionException(
                                    "No active tools found for skill ID: " + skillId));

                    log.info("Resolved skillId={} to tool: {} (type: {}, priority: {})",
                            skillId, selectedTool.name(), selectedTool.type(), selectedTool.priority());

                    return Mono.just(selectedTool);
                });
    }

    /**
     * Busca skill por slug no backend.
     * <p>
     * Nota: Backend não possui endpoint direto para buscar por slug,
     * então fazemos query de todas as skills e filtramos em memória.
     * </p>
     */
    private Mono<Skill> findSkillBySlug(UUID tenantId, String skillSlug) {
        log.debug("Fetching skill by slug from backend: slug={}", skillSlug);
        
        return backendClient.get()
                .uri(uriBuilder -> uriBuilder
                        .path("/api/skills")
                        .queryParam("size", 1000)
                        .build())
                .header("X-Tenant-ID", tenantId.toString())
                .retrieve()
                .bodyToMono(new ParameterizedTypeReference<Page<SkillResponse>>() {})
                .flatMapMany(page -> Flux.fromIterable(page.getContent()))
                .filter(skill -> skillSlug.equals(skill.slug()))
                .next()
                .map(this::toSkillDomain)
                .switchIfEmpty(Mono.error(new SkillResolutionException(
                        "Skill not found: " + skillSlug)));
    }

    /**
     * Busca tools vinculadas a uma skill no backend.
     * <p>
     * Fluxo:
     * 1. GET /api/skills/{skillId}/tools → lista de SkillToolBinding (toolId + priority)
     * 2. Para cada toolId, GET /api/tools/{toolId} → ToolResponse completo
     * 3. Merge binding + tool → Tool domain
     * </p>
     */
    private Mono<List<Tool>> findToolsForSkill(UUID tenantId, UUID skillId) {
        log.debug("Fetching tools for skill from backend: skillId={}", skillId);
        
        return backendClient.get()
                .uri("/api/skills/{skillId}/tools", skillId)
                .header("X-Tenant-ID", tenantId.toString())
                .retrieve()
                .bodyToFlux(SkillToolBinding.class)
                .flatMap(binding -> fetchToolDetails(tenantId, binding))
                .collectList();
    }

    /**
     * Busca detalhes completos de uma tool e combina com binding.
     */
    private Mono<Tool> fetchToolDetails(UUID tenantId, SkillToolBinding binding) {
        return backendClient.get()
                .uri("/api/tools/{toolId}", binding.toolId())
                .header("X-Tenant-ID", tenantId.toString())
                .retrieve()
                .bodyToMono(ToolResponse.class)
                .map(toolResponse -> toToolDomain(toolResponse, binding));
    }

    /**
     * Converte SkillResponse do backend para Skill domain.
     */
    private Skill toSkillDomain(SkillResponse response) {
        return new Skill(
                response.id(),
                response.slug(),
                response.name(),
                response.category(),
                response.description(),
                response.status(),
                Map.of(
                        "version", response.version(),
                        "inputSchema", response.inputSchema(),
                        "outputSchema", response.outputSchema()
                )
        );
    }

    /**
     * Converte ToolResponse + SkillToolBinding para Tool domain.
     */
    private Tool toToolDomain(ToolResponse response, SkillToolBinding binding) {
        // Mapear tipo do backend para tipo do executor
        String executorType = mapToolTypeToExecutorType(response.type());
        
        return new Tool(
                response.id(),
                binding.skillId(),
                response.name(),
                executorType,
                response.config() != null ? response.config() : Map.of(),
                binding.priority(),
                "ACTIVE"  // Tools retornadas pelo binding são consideradas ativas
        );
    }

    /**
     * Mapeia tipo de tool do backend para tipo de executor.
     * <p>
     * Backend: CODE, DATABASE, DOCUMENTS, BLOCKLY
     * Runtime: HTTP, SQL, DOCUMENT_SEARCH, MCP, SCRIPT
     * </p>
     */
    private String mapToolTypeToExecutorType(String backendType) {
        return switch (backendType) {
            case "DATABASE" -> "SQL";
            case "DOCUMENTS" -> "DOCUMENT_SEARCH";
            case "CODE" -> "SCRIPT";
            default -> backendType;  // HTTP, MCP mantêm o nome
        };
    }

    /**
     * Exceção lançada quando não é possível resolver uma skill para tool.
     */
    public static class SkillResolutionException extends RuntimeException {
        public SkillResolutionException(String message) {
            super(message);
        }
    }
}

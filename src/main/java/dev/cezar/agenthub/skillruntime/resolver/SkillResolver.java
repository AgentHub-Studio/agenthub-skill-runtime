package dev.cezar.agenthub.skillruntime.resolver;

import dev.cezar.agenthub.skillruntime.domain.Skill;
import dev.cezar.agenthub.skillruntime.domain.Tool;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;
import org.springframework.web.reactive.function.client.WebClient;
import reactor.core.publisher.Mono;

import java.util.Comparator;
import java.util.List;
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
     */
    private Mono<Skill> findSkillBySlug(UUID tenantId, String skillSlug) {
        // TODO: Implementar chamada real ao backend
        // GET /api/skills?slug={skillSlug}
        
        log.warn("Using placeholder skill resolution (backend integration pending)");
        
        // Placeholder: retorna skill fictícia
        return Mono.just(new Skill(
                UUID.randomUUID(),
                skillSlug,
                "Placeholder Skill",
                "PLACEHOLDER",
                "Placeholder skill for testing",
                "ACTIVE",
                java.util.Map.of()
        ));
    }

    /**
     * Busca tools vinculadas a uma skill no backend.
     */
    private Mono<List<Tool>> findToolsForSkill(UUID tenantId, UUID skillId) {
        // TODO: Implementar chamada real ao backend
        // GET /api/skills/{skillId}/tools
        
        log.warn("Using placeholder tool resolution (backend integration pending)");
        
        // Placeholder: retorna tool fictícia baseada no tipo inferido do slug
        Tool placeholderTool = new Tool(
                UUID.randomUUID(),
                skillId,
                "Placeholder Tool",
                "PLACEHOLDER",
                java.util.Map.of(),
                1,
                "ACTIVE"
        );
        
        return Mono.just(List.of(placeholderTool));
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

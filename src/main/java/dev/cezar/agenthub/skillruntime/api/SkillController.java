package dev.cezar.agenthub.skillruntime.api;

import io.swagger.v3.oas.annotations.Operation;
import io.swagger.v3.oas.annotations.tags.Tag;
import jakarta.validation.Valid;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.HttpStatus;
import org.springframework.web.bind.annotation.*;
import reactor.core.publisher.Mono;

import java.time.OffsetDateTime;
import java.util.Map;
import java.util.UUID;

/**
 * Controller REST para invocar skills.
 *
 * @since 1.0.0
 */
@Slf4j
@RestController
@RequestMapping("/api/v1/skills")
@Tag(name = "Skills", description = "Skill invocation and execution")
public class SkillController {

    /**
     * Invoca uma skill.
     *
     * @param request requisição de invocação
     * @return {@link Mono} contendo a resposta da execução
     */
    @PostMapping("/invoke")
    @ResponseStatus(HttpStatus.OK)
    @Operation(summary = "Invoke skill", description = "Executes a skill by slug or ID")
    public Mono<SkillResponse> invokeSkill(@Valid @RequestBody SkillRequest request) {
        log.info("Invoking skill: tenantId={}, skillSlug={}", 
                request.tenantId(), request.skillSlug());

        long startTime = System.currentTimeMillis();

        // TODO: Implementar lógica real
        // 1. Resolver skill → tool
        // 2. Executar tool
        // 3. Retornar resultado

        return Mono.just(SkillResponse.success(
                UUID.randomUUID(),
                request.skillSlug(),
                UUID.randomUUID(),
                Map.of(
                        "message", "Skill invoked successfully (placeholder)",
                        "input", request.input(),
                        "timestamp", OffsetDateTime.now()
                ),
                System.currentTimeMillis() - startTime
        ));
    }

    /**
     * Health check endpoint.
     */
    @GetMapping("/health")
    public Mono<Map<String, Object>> health() {
        return Mono.just(Map.of(
                "status", "UP",
                "service", "agenthub-skill-runtime",
                "timestamp", OffsetDateTime.now()
        ));
    }
}

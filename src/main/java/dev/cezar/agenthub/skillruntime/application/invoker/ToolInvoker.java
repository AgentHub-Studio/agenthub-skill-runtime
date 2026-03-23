package dev.cezar.agenthub.skillruntime.application.invoker;

import dev.cezar.agenthub.skillruntime.adapter.in.rest.SkillRequest;
import dev.cezar.agenthub.skillruntime.adapter.in.rest.SkillResponse;
import dev.cezar.agenthub.skillruntime.domain.model.Skill;
import dev.cezar.agenthub.skillruntime.domain.model.Tool;
import dev.cezar.agenthub.skillruntime.domain.port.ToolExecutor;
import dev.cezar.agenthub.skillruntime.application.executor.ToolExecutorRegistry;
import dev.cezar.agenthub.skillruntime.application.resolver.ResolvedSkill;
import dev.cezar.agenthub.skillruntime.application.resolver.SkillResolver;
import dev.cezar.agenthub.skillruntime.application.validator.SkillValidator;
import lombok.AllArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Mono;
import reactor.util.retry.Retry;

import java.time.Duration;
import java.util.Map;
import java.util.UUID;

/**
 * Service responsável por invocar tools com retry e timeout.
 * <p>
 * Fluxo de execução:
 * 1. Resolve skill → tool (via SkillResolver)
 * 2. Valida input contra inputSchema da skill (via SkillValidator)
 * 3. Busca executor apropriado (via ToolExecutorRegistry)
 * 4. Valida inputs específicos da tool (via ToolExecutor.validate)
 * 5. Executa tool com timeout e retry
 * 6. Retorna resultado
 * </p>
 *
 * @since 1.0.0
 */
@Slf4j
@Service
@AllArgsConstructor
public class ToolInvoker {

    private final SkillResolver skillResolver;
    private final ToolExecutorRegistry executorRegistry;
    private final SkillValidator skillValidator;

    /**
     * Invoca uma skill de forma síncrona (bloqueia até completar).
     *
     * @param request requisição de invocação
     * @return {@link Mono} com resposta da execução
     */
    public Mono<SkillResponse> invoke(SkillRequest request) {
        long startTime = System.currentTimeMillis();
        UUID executionId = UUID.randomUUID();

        log.info("Invoking skill: executionId={}, tenantId={}, skillSlug={}",
                executionId, request.tenantId(), request.skillSlug());

        return resolveSkillAndTool(request)
                .flatMap(resolved -> 
                    // Valida input contra schema da skill
                    skillValidator.validateInput(resolved.skill(), request.input())
                        .then(Mono.just(resolved))
                )
                .flatMap(resolved -> executeToolWithRetry(
                        resolved.skill(),
                        resolved.tool(),
                        request,
                        executionId,
                        startTime
                ))
                .onErrorResume(error -> {
                    long latency = System.currentTimeMillis() - startTime;
                    log.error("Skill invocation failed: executionId={}, error={}",
                            executionId, error.getMessage(), error);

                    return Mono.just(SkillResponse.error(
                            executionId,
                            request.skillSlug(),
                            error.getMessage(),
                            latency
                    ));
                });
    }

    /**
     * Resolve skill e tool concreta.
     * Retorna tanto a Skill (para validação de schema) quanto a Tool (para execução).
     */
    private Mono<ResolvedSkill> resolveSkillAndTool(SkillRequest request) {
        // Por enquanto, vamos buscar a skill do backend junto com a resolução
        // TODO: Implementar método no SkillResolver que retorna Skill completa
        // Por agora, criar Skill placeholder a partir da Tool
        
        Mono<Tool> toolMono;
        if (request.skillId() != null) {
            toolMono = skillResolver.resolveById(request.tenantId(), request.skillId());
        } else {
            toolMono = skillResolver.resolveBySlug(request.tenantId(), request.skillSlug());
        }
        
        return toolMono.map(tool -> {
            // Cria Skill placeholder (sem schema por enquanto - será ignorado pelo validator)
            // Quando SkillResolver retornar Skill completa, usaremos o schema real
            Skill skill = new Skill(
                    tool.skillId(),
                    request.skillSlug(),
                    request.skillSlug(),
                    "UNKNOWN",
                    "Skill for " + request.skillSlug(),
                    "ACTIVE",
                    Map.of() // Sem schema por enquanto
            );
            
            return new ResolvedSkill(skill, tool);
        });
    }

    /**
     * Resolve skill para tool concreta (método legado).
     * @deprecated Use resolveSkillAndTool() para ter acesso ao schema
     */
    @Deprecated
    private Mono<Tool> resolveTool(SkillRequest request) {
        if (request.skillId() != null) {
            return skillResolver.resolveById(request.tenantId(), request.skillId());
        } else {
            return skillResolver.resolveBySlug(request.tenantId(), request.skillSlug());
        }
    }

    /**
     * Executa tool com retry, timeout e tratamento de erros.
     */
    private Mono<SkillResponse> executeToolWithRetry(
            Skill skill,
            Tool tool,
            SkillRequest request,
            UUID executionId,
            long startTime) {

        // Busca executor para o tipo de tool
        ToolExecutor executor = executorRegistry.getExecutor(tool.type())
                .orElseThrow(() -> new UnsupportedToolTypeException(
                        "No executor found for tool type: " + tool.type()));

        // Cria contexto de execução
        ToolExecutor.ExecutionContext context = buildExecutionContext(request);

        // Timeout padrão ou customizado
        int timeoutMs = request.timeout() != null ? request.timeout() : 30000;

        // Retry policy padrão ou customizada
        SkillRequest.RetryPolicy retryPolicy = request.retryPolicy() != null
                ? request.retryPolicy()
                : SkillRequest.RetryPolicy.defaultPolicy();

        return executor.validate(tool, request.input())
                .then(Mono.defer(() -> executor.execute(tool, request.input(), context)))
                .timeout(Duration.ofMillis(timeoutMs))
                .retryWhen(Retry.backoff(
                        retryPolicy.maxAttempts(),
                        Duration.ofMillis(retryPolicy.backoffMs())
                ).multiplier(retryPolicy.backoffMultiplier()))
                .map(result -> {
                    long latency = System.currentTimeMillis() - startTime;
                    log.info("Tool executed successfully: executionId={}, tool={}, latency={}ms",
                            executionId, tool.name(), latency);

                    return SkillResponse.success(
                            executionId,
                            request.skillSlug(),
                            tool.id(),
                            result,
                            latency
                    );
                })
                .onErrorResume(error -> {
                    long latency = System.currentTimeMillis() - startTime;
                    log.error("Tool execution failed: executionId={}, tool={}, error={}",
                            executionId, tool.name(), error.getMessage());

                    return Mono.just(SkillResponse.error(
                            executionId,
                            request.skillSlug(),
                            "Tool execution failed: " + error.getMessage(),
                            latency
                    ));
                });
    }

    /**
     * Constrói contexto de execução a partir do request.
     */
    private ToolExecutor.ExecutionContext buildExecutionContext(SkillRequest request) {
        if (request.executionContext() == null) {
            return new ToolExecutor.ExecutionContext(
                    request.tenantId().toString(),
                    null,
                    null,
                    null,
                    null
            );
        }

        SkillRequest.ExecutionContext ctx = request.executionContext();
        return new ToolExecutor.ExecutionContext(
                request.tenantId().toString(),
                ctx.userId() != null ? ctx.userId().toString() : null,
                ctx.agentId() != null ? ctx.agentId().toString() : null,
                ctx.executionId() != null ? ctx.executionId().toString() : null,
                ctx.nodeId()
        );
    }

    /**
     * Exceção lançada quando tipo de tool não é suportado.
     */
    public static class UnsupportedToolTypeException extends RuntimeException {
        public UnsupportedToolTypeException(String message) {
            super(message);
        }
    }
}

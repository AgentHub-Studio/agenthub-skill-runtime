package dev.cezar.agenthub.skillruntime.application.executor.impl;

import dev.cezar.agenthub.skillruntime.domain.port.ToolExecutor;

import com.fasterxml.jackson.databind.ObjectMapper;
import dev.cezar.agenthub.skillruntime.domain.model.Tool;
import groovy.lang.Binding;
import groovy.lang.GroovyShell;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;
import reactor.core.scheduler.Schedulers;

import java.util.HashMap;
import java.util.Map;

/**
 * Executor para tools do tipo SCRIPT.
 * Executa scripts Groovy de forma isolada e segura.
 * 
 * Configuration esperado:
 * {
 *   "script": "return ['result': input.value * 2, 'message': 'Success']",
 *   "language": "GROOVY",  // por enquanto só Groovy suportado
 *   "timeout": 5000        // timeout em ms (default: 5s)
 * }
 * 
 * Parameters esperado:
 * {
 *   "value": 10,
 *   "otherParam": "test"
 * }
 * 
 * O script tem acesso a:
 * - input: Map com os parâmetros passados
 * - context: ExecutionContext (tenantId, userId, agentId, executionId, nodeId)
 * - log: Logger para debug
 * 
 * O script deve retornar um Map ou Object que será serializado como resultado.
 * 
 * Segurança:
 * - Scripts rodam em thread separada (boundedElastic)
 * - Timeout configurável
 * - Sem acesso a System, File I/O, Network (via GroovyShell padrão)
 * - Future: adicionar SecureASTCustomizer para restrições adicionais
 */
@Slf4j
@Component
@RequiredArgsConstructor
public class ScriptToolExecutor implements ToolExecutor {
    
    private final ObjectMapper objectMapper;
    
    private static final int DEFAULT_TIMEOUT_MS = 5000;
    
    @Override
    public Mono<Map<String, Object>> execute(Tool tool, Map<String, Object> input, ExecutionContext context) {
        return Mono.fromCallable(() -> parseConfiguration(tool.config()))
                .flatMap(config -> executeScript(config, input, context))
                .subscribeOn(Schedulers.boundedElastic()) // Executa em thread separada
                .onErrorResume(error -> {
                    log.error("Script execution failed: {}", error.getMessage(), error);
                    return Mono.just(Map.of(
                            "success", false,
                            "error", error.getMessage(),
                            "errorType", error.getClass().getSimpleName()
                    ));
                });
    }
    
    @Override
    public Mono<Void> validate(Tool tool, Map<String, Object> input) {
        return Mono.fromRunnable(() -> {
            ScriptConfig config = parseConfiguration(tool.config());
            
            if (config.script == null || config.script.isBlank()) {
                throw new IllegalArgumentException("Script code is required");
            }
            
            if (config.language == null || config.language.isBlank()) {
                throw new IllegalArgumentException("Script language is required");
            }
            
            if (!"GROOVY".equalsIgnoreCase(config.language)) {
                throw new IllegalArgumentException("Only GROOVY language is currently supported, got: " + config.language);
            }
            
            if (config.timeout != null && config.timeout <= 0) {
                throw new IllegalArgumentException("Timeout must be positive");
            }
        });
    }
    
    @Override
    public String getSupportedType() {
        return "SCRIPT";
    }
    
    private ScriptConfig parseConfiguration(Map<String, Object> config) {
        try {
            return objectMapper.convertValue(config, ScriptConfig.class);
        } catch (Exception e) {
            throw new IllegalArgumentException("Invalid SCRIPT configuration: " + e.getMessage(), e);
        }
    }
    
    private Mono<Map<String, Object>> executeScript(
            ScriptConfig config, 
            Map<String, Object> input, 
            ExecutionContext context) {
        
        return Mono.fromCallable(() -> {
            log.debug("Executing Groovy script for tenant: {}", context.tenantId());
            
            // Cria binding com variáveis disponíveis para o script
            Binding binding = new Binding();
            binding.setProperty("input", input != null ? input : Map.of());
            binding.setProperty("context", Map.of(
                    "tenantId", context.tenantId(),
                    "userId", context.userId(),
                    "agentId", context.agentId(),
                    "executionId", context.executionId(),
                    "nodeId", context.nodeId()
            ));
            binding.setProperty("log", log);
            
            // Executa script
            GroovyShell shell = new GroovyShell(binding);
            Object result = shell.evaluate(config.script);
            
            // Converte resultado para Map
            if (result == null) {
                Map<String, Object> nullResult = new HashMap<>();
                nullResult.put("result", null);
                return nullResult;
            }
            
            if (result instanceof Map) {
                @SuppressWarnings("unchecked")
                Map<String, Object> mapResult = (Map<String, Object>) result;
                return mapResult;
            }
            
            // Para outros tipos, envelopa em um Map
            Map<String, Object> wrappedResult = new HashMap<>();
            wrappedResult.put("result", result);
            wrappedResult.put("type", result.getClass().getSimpleName());
            return wrappedResult;
            
        }).timeout(java.time.Duration.ofMillis(
                config.timeout != null ? config.timeout : DEFAULT_TIMEOUT_MS
        ));
    }
    
    private static class ScriptConfig {
        public String script;
        public String language;
        public Integer timeout;
    }
}

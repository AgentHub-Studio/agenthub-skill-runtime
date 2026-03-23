package dev.cezar.agenthub.skillruntime.application.executor.impl;

import dev.cezar.agenthub.skillruntime.domain.port.ToolExecutor;

import dev.cezar.agenthub.skillruntime.domain.model.Tool;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.HttpHeaders;
import org.springframework.http.HttpMethod;
import org.springframework.http.MediaType;
import org.springframework.stereotype.Component;
import org.springframework.web.reactive.function.client.WebClient;
import reactor.core.publisher.Mono;

import java.util.Map;

/**
 * Executor para tools do tipo HTTP.
 * <p>
 * Realiza chamadas HTTP (GET, POST, PUT, DELETE, PATCH) usando WebClient.
 * Suporta headers customizados, query params, e corpo de requisição.
 * </p>
 * <p>
 * Configuração esperada da tool:
 * <pre>{@code
 * {
 *   "url": "https://api.example.com/endpoint",
 *   "method": "POST",  // GET, POST, PUT, DELETE, PATCH
 *   "headers": {       // opcional
 *     "Authorization": "Bearer token",
 *     "Content-Type": "application/json"
 *   },
 *   "timeout": 10000   // opcional, em ms
 * }
 * }</pre>
 * </p>
 * <p>
 * Input esperado:
 * <pre>{@code
 * {
 *   "body": {...},           // corpo da requisição (para POST/PUT/PATCH)
 *   "queryParams": {...},    // query parameters
 *   "pathParams": {...}      // path parameters para substituição na URL
 * }
 * }</pre>
 * </p>
 *
 * @since 1.0.0
 */
@Slf4j
@Component
public class HttpToolExecutor implements ToolExecutor {

    private final WebClient webClient;

    public HttpToolExecutor() {
        this.webClient = WebClient.builder()
                .defaultHeader(HttpHeaders.USER_AGENT, "AgentHub-SkillRuntime/1.0")
                .build();
    }

    @Override
    public String getSupportedType() {
        return "HTTP";
    }

    @Override
    public Mono<Map<String, Object>> execute(Tool tool, Map<String, Object> input, ExecutionContext context) {
        log.info("Executing HTTP tool: toolId={}, method={}", tool.id(), getMethod(tool));

        String url = buildUrl(tool, input);
        HttpMethod method = HttpMethod.valueOf(getMethod(tool));
        Map<String, String> headers = getHeaders(tool);

        WebClient.RequestBodySpec request = webClient.method(method)
                .uri(url)
                .headers(httpHeaders -> headers.forEach(httpHeaders::add));

        // Adiciona body se presente (POST, PUT, PATCH)
        if (input.containsKey("body") && 
            (method == HttpMethod.POST || method == HttpMethod.PUT || method == HttpMethod.PATCH)) {
            request.contentType(MediaType.APPLICATION_JSON)
                    .bodyValue(input.get("body"));
        }

        return request.retrieve()
                .bodyToMono(Map.class)
                .map(response -> {
                    log.info("HTTP tool executed successfully: toolId={}, url={}", tool.id(), url);
                    return Map.of(
                            "status", "success",
                            "response", response,
                            "url", url,
                            "method", method.name()
                    );
                })
                .onErrorResume(error -> {
                    log.error("HTTP tool execution failed: toolId={}, url={}, error={}",
                            tool.id(), url, error.getMessage());
                    return Mono.just(Map.of(
                            "status", "error",
                            "error", error.getMessage(),
                            "url", url,
                            "method", method.name()
                    ));
                });
    }

    @Override
    public Mono<Void> validate(Tool tool, Map<String, Object> input) {
        // Valida URL
        String url = (String) tool.config().get("url");
        if (url == null || url.isBlank()) {
            return Mono.error(new IllegalArgumentException("HTTP tool requires 'url' in config"));
        }

        // Valida método HTTP
        String method = getMethod(tool);
        try {
            HttpMethod.valueOf(method);
        } catch (IllegalArgumentException e) {
            return Mono.error(new IllegalArgumentException(
                    "Invalid HTTP method: " + method + ". Must be GET, POST, PUT, DELETE, or PATCH"));
        }

        return Mono.empty();
    }

    /**
     * Constrói URL completa com path params e query params.
     */
    private String buildUrl(Tool tool, Map<String, Object> input) {
        String url = (String) tool.config().get("url");

        // Substitui path params: /users/{userId} → /users/123
        if (input.containsKey("pathParams")) {
            Map<String, Object> pathParams = (Map<String, Object>) input.get("pathParams");
            for (Map.Entry<String, Object> entry : pathParams.entrySet()) {
                url = url.replace("{" + entry.getKey() + "}", String.valueOf(entry.getValue()));
            }
        }

        // Adiciona query params: ?key=value&foo=bar
        if (input.containsKey("queryParams")) {
            Map<String, Object> queryParams = (Map<String, Object>) input.get("queryParams");
            StringBuilder urlBuilder = new StringBuilder(url);
            urlBuilder.append(url.contains("?") ? "&" : "?");
            
            queryParams.forEach((key, value) -> 
                urlBuilder.append(key).append("=").append(value).append("&")
            );
            
            url = urlBuilder.toString();
            // Remove último & se existir
            if (url.endsWith("&")) {
                url = url.substring(0, url.length() - 1);
            }
        }

        return url;
    }

    /**
     * Extrai método HTTP da configuração (padrão: GET).
     */
    private String getMethod(Tool tool) {
        return (String) tool.config().getOrDefault("method", "GET");
    }

    /**
     * Extrai headers customizados da configuração.
     */
    @SuppressWarnings("unchecked")
    private Map<String, String> getHeaders(Tool tool) {
        return (Map<String, String>) tool.config().getOrDefault("headers", Map.of());
    }
}

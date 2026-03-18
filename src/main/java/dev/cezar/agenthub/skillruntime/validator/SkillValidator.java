package dev.cezar.agenthub.skillruntime.validator;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.networknt.schema.JsonSchema;
import com.networknt.schema.JsonSchemaFactory;
import com.networknt.schema.SpecVersion;
import com.networknt.schema.ValidationMessage;
import dev.cezar.agenthub.skillruntime.domain.Skill;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

import java.util.Map;
import java.util.Set;
import java.util.stream.Collectors;

/**
 * Validador de inputs de skills usando JSON Schema.
 * 
 * Valida que os inputs fornecidos pelo usuário estejam de acordo com o
 * inputSchema definido na Skill.
 * 
 * Exemplos de validação:
 * - Campos obrigatórios presentes
 * - Tipos corretos (string, number, boolean, array, object)
 * - Formatos válidos (email, uri, date-time, etc.)
 * - Constraints (minLength, maxLength, minimum, maximum, etc.)
 * - Enums (valores permitidos)
 * 
 * @since 1.0.0
 */
@Slf4j
@Component
public class SkillValidator {

    private final ObjectMapper objectMapper;
    private final JsonSchemaFactory schemaFactory;
    
    public SkillValidator(ObjectMapper objectMapper) {
        this.objectMapper = objectMapper;
        // Usa JSON Schema Draft 2020-12 (mais recente)
        this.schemaFactory = JsonSchemaFactory.getInstance(SpecVersion.VersionFlag.V202012);
    }
    
    /**
     * Valida input contra o inputSchema da skill.
     * 
     * @param skill skill com inputSchema definido
     * @param input dados a serem validados
     * @return Mono que completa se válido, ou emite erro se inválido
     */
    public Mono<Void> validateInput(Skill skill, Map<String, Object> input) {
        return Mono.fromCallable(() -> {
            log.debug("Validating input for skill: {}", skill.slug());
            
            // Verifica se skill tem inputSchema
            if (skill.schema() == null || !skill.schema().containsKey("inputSchema")) {
                log.warn("Skill {} has no inputSchema defined, skipping validation", skill.slug());
                return null;
            }
            
            Object inputSchemaObj = skill.schema().get("inputSchema");
            if (inputSchemaObj == null) {
                log.warn("Skill {} inputSchema is null, skipping validation", skill.slug());
                return null;
            }
            
            // Converte inputSchema para JsonNode
            JsonNode schemaNode;
            if (inputSchemaObj instanceof JsonNode) {
                schemaNode = (JsonNode) inputSchemaObj;
            } else if (inputSchemaObj instanceof Map) {
                schemaNode = objectMapper.valueToTree(inputSchemaObj);
            } else {
                schemaNode = objectMapper.readTree(inputSchemaObj.toString());
            }
            
            // Cria validador JSON Schema
            JsonSchema schema = schemaFactory.getSchema(schemaNode);
            
            // Converte input para JsonNode
            JsonNode inputNode = objectMapper.valueToTree(input != null ? input : Map.of());
            
            // Valida
            Set<ValidationMessage> errors = schema.validate(inputNode);
            
            if (!errors.isEmpty()) {
                String errorMessages = errors.stream()
                        .map(ValidationMessage::getMessage)
                        .collect(Collectors.joining("; "));
                
                log.error("Input validation failed for skill {}: {}", skill.slug(), errorMessages);
                
                throw new SkillInputValidationException(
                        skill.slug(),
                        errorMessages,
                        errors
                );
            }
            
            log.debug("Input validation passed for skill: {}", skill.slug());
            return null;
        });
    }
    
    /**
     * Valida output contra o outputSchema da skill.
     * Útil para validar que a tool retornou dados no formato esperado.
     * 
     * @param skill skill com outputSchema definido
     * @param output dados retornados pela tool
     * @return Mono que completa se válido, ou emite erro se inválido
     */
    public Mono<Void> validateOutput(Skill skill, Map<String, Object> output) {
        return Mono.fromCallable(() -> {
            log.debug("Validating output for skill: {}", skill.slug());
            
            // Verifica se skill tem outputSchema
            if (skill.schema() == null || !skill.schema().containsKey("outputSchema")) {
                log.debug("Skill {} has no outputSchema defined, skipping validation", skill.slug());
                return null;
            }
            
            Object outputSchemaObj = skill.schema().get("outputSchema");
            if (outputSchemaObj == null) {
                log.debug("Skill {} outputSchema is null, skipping validation", skill.slug());
                return null;
            }
            
            // Converte outputSchema para JsonNode
            JsonNode schemaNode;
            if (outputSchemaObj instanceof JsonNode) {
                schemaNode = (JsonNode) outputSchemaObj;
            } else if (outputSchemaObj instanceof Map) {
                schemaNode = objectMapper.valueToTree(outputSchemaObj);
            } else {
                schemaNode = objectMapper.readTree(outputSchemaObj.toString());
            }
            
            // Cria validador JSON Schema
            JsonSchema schema = schemaFactory.getSchema(schemaNode);
            
            // Converte output para JsonNode
            JsonNode outputNode = objectMapper.valueToTree(output != null ? output : Map.of());
            
            // Valida
            Set<ValidationMessage> errors = schema.validate(outputNode);
            
            if (!errors.isEmpty()) {
                String errorMessages = errors.stream()
                        .map(ValidationMessage::getMessage)
                        .collect(Collectors.joining("; "));
                
                log.warn("Output validation failed for skill {}: {}", skill.slug(), errorMessages);
                
                throw new SkillOutputValidationException(
                        skill.slug(),
                        errorMessages,
                        errors
                );
            }
            
            log.debug("Output validation passed for skill: {}", skill.slug());
            return null;
        });
    }
    
    /**
     * Exceção lançada quando input não passa na validação do schema.
     */
    public static class SkillInputValidationException extends RuntimeException {
        private final String skillSlug;
        private final Set<ValidationMessage> validationErrors;
        
        public SkillInputValidationException(
                String skillSlug,
                String message,
                Set<ValidationMessage> validationErrors) {
            super(String.format("Input validation failed for skill '%s': %s", skillSlug, message));
            this.skillSlug = skillSlug;
            this.validationErrors = validationErrors;
        }
        
        public String getSkillSlug() {
            return skillSlug;
        }
        
        public Set<ValidationMessage> getValidationErrors() {
            return validationErrors;
        }
    }
    
    /**
     * Exceção lançada quando output não passa na validação do schema.
     */
    public static class SkillOutputValidationException extends RuntimeException {
        private final String skillSlug;
        private final Set<ValidationMessage> validationErrors;
        
        public SkillOutputValidationException(
                String skillSlug,
                String message,
                Set<ValidationMessage> validationErrors) {
            super(String.format("Output validation failed for skill '%s': %s", skillSlug, message));
            this.skillSlug = skillSlug;
            this.validationErrors = validationErrors;
        }
        
        public String getSkillSlug() {
            return skillSlug;
        }
        
        public Set<ValidationMessage> getValidationErrors() {
            return validationErrors;
        }
    }
}

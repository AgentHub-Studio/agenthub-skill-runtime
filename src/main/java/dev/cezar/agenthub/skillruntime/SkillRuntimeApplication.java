package dev.cezar.agenthub.skillruntime;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

/**
 * AgentHub Skill Runtime - Tool Resolution and Execution Engine.
 * <p>
 * Runtime responsável por:
 * <ul>
 *   <li>Resolver skills para tools concretas</li>
 *   <li>Executar tools (HTTP, SQL, DocumentSearch, MCP, Script)</li>
 *   <li>Gerenciar tool executors</li>
 *   <li>Registrar execuções para auditoria</li>
 * </ul>
 * </p>
 *
 * @since 1.0.0
 */
@SpringBootApplication
public class SkillRuntimeApplication {

    public static void main(String[] args) {
        SpringApplication.run(SkillRuntimeApplication.class, args);
    }
}

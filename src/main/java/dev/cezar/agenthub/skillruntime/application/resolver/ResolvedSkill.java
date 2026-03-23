package dev.cezar.agenthub.skillruntime.application.resolver;

import dev.cezar.agenthub.skillruntime.domain.model.Skill;
import dev.cezar.agenthub.skillruntime.domain.model.Tool;

/**
 * Container para Skill resolvida com sua Tool selecionada.
 * 
 * @param skill Skill abstrata
 * @param tool Tool concreta selecionada para executar a Skill
 */
public record ResolvedSkill(
        Skill skill,
        Tool tool
) {
}

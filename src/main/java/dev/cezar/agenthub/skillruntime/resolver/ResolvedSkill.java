package dev.cezar.agenthub.skillruntime.resolver;

import dev.cezar.agenthub.skillruntime.domain.Skill;
import dev.cezar.agenthub.skillruntime.domain.Tool;

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

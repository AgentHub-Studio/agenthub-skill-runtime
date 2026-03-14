package dev.cezar.agenthub.skillruntime.resolver;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

@DisplayName("SkillResolver - Resolução de skills para tools")
class SkillResolverTest {

    private SkillResolver resolver;

    @BeforeEach
    void setUp() {
        // Mock do backend URL (não será usado em testes unitários)
        resolver = new SkillResolver("http://localhost:8080");
    }

    @Test
    @DisplayName("Deve criar resolver com URL do backend configurada")
    void shouldCreateResolverWithBackendUrl() {
        assertThat(resolver).isNotNull();
    }

    @Test
    @DisplayName("Deve ter métodos públicos para resolução por slug e ID")
    void shouldHavePublicResolutionMethods() throws Exception {
        // Verifica que os métodos existem via reflection
        assertThat(resolver.getClass().getMethod("resolveBySlug",
                java.util.UUID.class, String.class)).isNotNull();
        
        assertThat(resolver.getClass().getMethod("resolveById",
                java.util.UUID.class, java.util.UUID.class)).isNotNull();
    }

    @Test
    @DisplayName("Deve ter exception customizada para erros de resolução")
    void shouldHaveCustomResolutionException() {
        // Verifica que a exception interna existe
        Class<?>[] declaredClasses = SkillResolver.class.getDeclaredClasses();
        boolean hasResolutionException = false;
        
        for (Class<?> declaredClass : declaredClasses) {
            if (declaredClass.getSimpleName().contains("SkillResolutionException")) {
                hasResolutionException = true;
                break;
            }
        }
        
        assertThat(hasResolutionException).isTrue();
    }

    // NOTA: Testes completos de integração com o backend devem ser feitos
    // em testes de integração (usando WireMock ou similar para mockar o backend)
    // Estes testes unitários apenas validam a estrutura da classe.
}

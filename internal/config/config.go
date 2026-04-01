package config

import (
	"fmt"
	"strings"

	"github.com/caarlos0/env/v11"
)

// Config holds all configuration for agenthub-skill-runtime.
type Config struct {
	ServerPort         string `env:"PORT"               envDefault:"8083"`
	DatabaseURL        string `env:"DATABASE_URL"       envRequired:"true"`
	KeycloakBaseURL    string `env:"KEYCLOAK_BASE_URL"  envRequired:"true"`
	KeycloakPublicURL  string `env:"KEYCLOAK_PUBLIC_URL"`
	CORSAllowedOrigins string `env:"CORS_ORIGINS"       envDefault:"*"`
	LogLevel           string `env:"LOG_LEVEL"          envDefault:"info"`
	OTLPEndpoint       string `env:"OTLP_ENDPOINT"`
	RabbitMQURL        string `env:"RABBITMQ_URL"`
}

// Load parses configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	if cfg.KeycloakPublicURL == "" {
		cfg.KeycloakPublicURL = cfg.KeycloakBaseURL
	}
	return cfg, nil
}

// CORSOrigins returns the CORS allowed origins as a slice.
func (c *Config) CORSOrigins() []string {
	return strings.Split(c.CORSAllowedOrigins, ",")
}

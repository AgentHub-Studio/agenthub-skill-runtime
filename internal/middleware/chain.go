package middleware

import (
	"net/http"
)

// Chain holds the configured middleware stack.
type Chain struct {
	keycloakBaseURL string
	corsOrigins     []string
}

// New creates a new middleware Chain.
func New(keycloakBaseURL string, corsOrigins []string) *Chain {
	return &Chain{
		keycloakBaseURL: keycloakBaseURL,
		corsOrigins:     corsOrigins,
	}
}

// CORSHandler returns the CORS middleware for use at the root router level.
// Must be applied before any route groups so that OPTIONS preflight requests
// are handled before chi returns 405 Method Not Allowed.
func (c *Chain) CORSHandler() func(http.Handler) http.Handler {
	return CORS(c.corsOrigins)
}

// Public returns middleware for unauthenticated routes: Recovery → RequestID → Logger.
// CORS is applied at root router level via CORSHandler(), not here.
func (c *Chain) Public() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{
		Recovery,
		RequestID,
		Logger,
	}
}

// Protected returns middleware for authenticated routes: Public stack + Auth.
// Auth validates the JWT via Keycloak JWKS and injects tenantID into context.
func (c *Chain) Protected() []func(http.Handler) http.Handler {
	return append(c.Public(),
		Auth(c.keycloakBaseURL),
	)
}

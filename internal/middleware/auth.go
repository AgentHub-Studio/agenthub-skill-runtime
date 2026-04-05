package middleware

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// tenantIDKey is the context key for the tenant ID.
const tenantIDKey contextKey = "tenantID"

// claimsKey is the context key for JWT claims.
const claimsKey contextKey = "jwtClaims"

// rawTokenKey is the context key for the raw Bearer JWT string.
const rawTokenKey contextKey = "rawToken"

var issuerTenantRe = regexp.MustCompile(`/realms/([^/]+)`)

// jwksCacheEntry holds a cached JWKS response with an expiry time.
type jwksCacheEntry struct {
	keys      map[string]*rsa.PublicKey
	expiresAt time.Time
}

// jwksCache caches JWKS keys per tenantID.
var (
	jwksMu    sync.Mutex
	jwksCache = make(map[string]*jwksCacheEntry)
)

const jwksCacheTTL = 5 * time.Minute

// jwksResponse represents a JSON Web Key Set from Keycloak.
type jwksResponse struct {
	Keys []struct {
		Kid string `json:"kid"`
		Kty string `json:"kty"`
		Alg string `json:"alg"`
		Use string `json:"use"`
		N   string `json:"n"`
		E   string `json:"e"`
	} `json:"keys"`
}

// fetchJWKS fetches and caches JWKS for the given tenantID from Keycloak.
func fetchJWKS(keycloakBaseURL, tenantID string) (map[string]*rsa.PublicKey, error) {
	jwksMu.Lock()
	entry, ok := jwksCache[tenantID]
	jwksMu.Unlock()

	if ok && time.Now().Before(entry.expiresAt) {
		return entry.keys, nil
	}

	url := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/certs", keycloakBaseURL, tenantID)
	resp, err := http.Get(url) //nolint:gosec // URL is constructed from validated config
	if err != nil {
		return nil, fmt.Errorf("auth: fetch JWKS for tenant %q: %w", tenantID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth: JWKS endpoint returned %d for tenant %q", resp.StatusCode, tenantID)
	}

	var jwks jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("auth: decode JWKS for tenant %q: %w", tenantID, err)
	}

	keys := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" || k.Use != "sig" {
			continue
		}
		pub, err := parseRSAPublicKey(k.N, k.E)
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
	}

	jwksMu.Lock()
	jwksCache[tenantID] = &jwksCacheEntry{keys: keys, expiresAt: time.Now().Add(jwksCacheTTL)}
	jwksMu.Unlock()

	return keys, nil
}

// parseRSAPublicKey builds an *rsa.PublicKey from base64url-encoded n and e.
func parseRSAPublicKey(nStr, eStr string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nStr)
	if err != nil {
		return nil, fmt.Errorf("auth: decode RSA modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eStr)
	if err != nil {
		return nil, fmt.Errorf("auth: decode RSA exponent: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	eBig := new(big.Int).SetBytes(eBytes)

	return &rsa.PublicKey{N: n, E: int(eBig.Int64())}, nil
}

// extractTenantID extracts the Keycloak realm slug from the JWT issuer claim.
// Expected issuer format: https://keycloak.example.com/realms/{tenantID}
func extractTenantID(issuer string) (string, error) {
	m := issuerTenantRe.FindStringSubmatch(issuer)
	if len(m) < 2 {
		return "", fmt.Errorf("auth: cannot extract tenantID from issuer %q", issuer)
	}
	return m[1], nil
}

// Auth returns a JWT validation middleware backed by Keycloak JWKS.
// It extracts the tenantID from the JWT issuer and injects it into the request context.
func Auth(keycloakBaseURL string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				writeUnauthorized(w, "missing or invalid Authorization header")
				return
			}
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

			// Parse unverified to extract issuer and kid.
			unverified, _, err := jwt.NewParser().ParseUnverified(tokenStr, jwt.MapClaims{})
			if err != nil {
				writeUnauthorized(w, "invalid token format")
				return
			}

			claims, ok := unverified.Claims.(jwt.MapClaims)
			if !ok {
				writeUnauthorized(w, "invalid token claims")
				return
			}

			issuer, _ := claims["iss"].(string)
			tenantID, err := extractTenantID(issuer)
			if err != nil {
				writeUnauthorized(w, "cannot resolve tenant from token issuer")
				return
			}

			keys, err := fetchJWKS(keycloakBaseURL, tenantID)
			if err != nil {
				writeUnauthorized(w, "failed to fetch JWKS")
				return
			}

			// Verify signature using the matching key.
			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
					return nil, fmt.Errorf("auth: unexpected signing method: %v", t.Header["alg"])
				}
				kid, _ := t.Header["kid"].(string)
				if kid == "" {
					// Try first key if no kid header.
					for _, k := range keys {
						return k, nil
					}
					return nil, fmt.Errorf("auth: no keys available")
				}
				k, ok := keys[kid]
				if !ok {
					return nil, fmt.Errorf("auth: unknown kid %q", kid)
				}
				return k, nil
			}, jwt.WithValidMethods([]string{"RS256"}))

			if err != nil || !token.Valid {
				writeUnauthorized(w, "token validation failed")
				return
			}

			ctx := context.WithValue(r.Context(), tenantIDKey, tenantID)
			ctx = context.WithValue(ctx, claimsKey, token.Claims)
			ctx = context.WithValue(ctx, rawTokenKey, tokenStr)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// TenantIDFromContext returns the tenant ID stored in ctx by Auth middleware.
func TenantIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(tenantIDKey).(string)
	return id
}

// ClaimsFromContext returns the JWT claims stored in ctx by Auth middleware.
func ClaimsFromContext(ctx context.Context) jwt.Claims {
	c, _ := ctx.Value(claimsKey).(jwt.Claims)
	return c
}

// RawTokenFromContext returns the raw Bearer JWT string stored in ctx by Auth middleware.
func RawTokenFromContext(ctx context.Context) string {
	t, _ := ctx.Value(rawTokenKey).(string)
	return t
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

package middleware

import (
	"strings"
	"testing"
)

func TestBuildJWKSURL_EscapesTenantPathSegment(t *testing.T) {
	got, err := buildJWKSURL("https://keycloak.example/auth", "tenant/a b")
	if err != nil {
		t.Fatalf("buildJWKSURL: %v", err)
	}

	want := "https://keycloak.example/auth/realms/tenant%2Fa%20b/protocol/openid-connect/certs"
	if got.String() != want {
		t.Fatalf("JWKS URL: got %q want %q", got.String(), want)
	}
}

func TestBuildJWKSURL_RejectsUnsafeBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{name: "empty", baseURL: "", want: "empty"},
		{name: "relative", baseURL: "keycloak:8080", want: "absolute"},
		{name: "unsupported scheme", baseURL: "ftp://keycloak.example", want: "scheme"},
		{name: "userinfo", baseURL: "https://user:pass@keycloak.example", want: "user info"},
		{name: "control character", baseURL: "https://keycloak.example\n.evil", want: "control"},
		{name: "query", baseURL: "https://keycloak.example?next=http://evil.example", want: "query"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildJWKSURL(tt.baseURL, "tenant-a")
			if err == nil {
				t.Fatal("expected unsafe base URL to be rejected")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error: got %q want substring %q", err.Error(), tt.want)
			}
		})
	}
}

func TestBuildJWKSURL_RejectsUnsafeTenantID(t *testing.T) {
	_, err := buildJWKSURL("https://keycloak.example", "tenant-a\nforged")
	if err == nil {
		t.Fatal("expected unsafe tenant ID to be rejected")
	}
	if !strings.Contains(err.Error(), "tenant ID contains invalid control characters") {
		t.Fatalf("error: %v", err)
	}
}

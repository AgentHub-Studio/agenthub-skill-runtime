package http

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDialOutboundRejectsDNSResolvedMetadataBeforeDial(t *testing.T) {
	dialed := false
	executor := &HTTPToolExecutor{
		resolveHost: func(_ context.Context, host string) ([]net.IPAddr, error) {
			require.Equal(t, "attacker.example.test", host)
			return []net.IPAddr{{IP: net.ParseIP("169.254.169.254")}}, nil
		},
		dialContext: func(context.Context, string, string) (net.Conn, error) {
			dialed = true
			return nil, errors.New("must not dial a blocked target")
		},
	}

	_, err := executor.dialOutbound(context.Background(), "tcp", "attacker.example.test:443", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "blocked address")
	assert.False(t, dialed)
}

func TestDialOutboundPinsAllowedPublicAddress(t *testing.T) {
	expected := errors.New("dial reached approved address")
	var dialAddress string
	executor := &HTTPToolExecutor{
		resolveHost: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}}, nil
		},
		dialContext: func(_ context.Context, _ string, address string) (net.Conn, error) {
			dialAddress = address
			return nil, expected
		},
	}

	_, err := executor.dialOutbound(context.Background(), "tcp", "public.example.test:443", "")
	require.ErrorIs(t, err, expected)
	assert.Equal(t, "8.8.8.8:443", dialAddress)
}

func TestProtectedClientRejectsRedirectToMetadata(t *testing.T) {
	executor := NewHTTPToolExecutor("")
	client := executor.newHTTPClient("")
	req := httptest.NewRequest(http.MethodGet, "http://169.254.169.254/latest/meta-data/", nil)
	err := client.CheckRedirect(req, []*http.Request{httptest.NewRequest(http.MethodGet, "https://public.example.test", nil)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redirect blocked")
}

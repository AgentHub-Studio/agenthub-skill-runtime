package mcp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPHTTPDialBlocksDNSRebindingBeforeDial(t *testing.T) {
	dialed := false
	executor := &MCPToolExecutor{
		resolveHost: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("169.254.169.254")}}, nil
		},
		dialContext: func(context.Context, string, string) (net.Conn, error) {
			dialed = true
			return nil, errors.New("must not dial a blocked target")
		},
	}

	_, err := executor.dialHTTPOutbound(context.Background(), "tcp", "attacker.example.test:443")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "blocked address")
	assert.False(t, dialed)
}

func TestMCPHTTPDialPinsValidatedAddress(t *testing.T) {
	expected := errors.New("dial attempted")
	var dialAddress string
	executor := &MCPToolExecutor{
		resolveHost: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}}, nil
		},
		dialContext: func(_ context.Context, _ string, address string) (net.Conn, error) {
			dialAddress = address
			return nil, expected
		},
	}

	_, err := executor.dialHTTPOutbound(context.Background(), "tcp", "public.example.test:443")
	require.ErrorIs(t, err, expected)
	assert.Equal(t, "8.8.8.8:443", dialAddress)
}

func TestMCPHTTPExecutorBlocksPrivateEndpointBeforeRequest(t *testing.T) {
	executor := NewMCPToolExecutor(nil)
	_, err := executor.executeHTTP(context.Background(), &mcpServerConfig{
		HTTPBaseURL: "http://169.254.169.254",
	}, "read", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "blocked HTTP endpoint")
}

func TestMCPHTTPClientDisablesProxyAndBlocksPrivateRedirect(t *testing.T) {
	executor := NewMCPToolExecutor(nil)
	client := executor.newProtectedHTTPClient()
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	assert.Nil(t, transport.Proxy)

	redirect, err := http.NewRequest(http.MethodGet, "http://169.254.169.254/latest/meta-data", nil)
	require.NoError(t, err)
	require.NotNil(t, client.CheckRedirect)
	err = client.CheckRedirect(redirect, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redirect blocked")
}

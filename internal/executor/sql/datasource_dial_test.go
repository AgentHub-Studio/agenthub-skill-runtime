package sql

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatasourceDialRejectsHostnameResolvedToMetadataAddress(t *testing.T) {
	executor := &SQLToolExecutor{
		resolveHost: func(_ context.Context, host string) ([]net.IPAddr, error) {
			require.Equal(t, "attacker.example.test", host)
			return []net.IPAddr{{IP: net.ParseIP("169.254.169.254")}}, nil
		},
	}

	_, err := executor.resolveDatasourceHost(context.Background(), "attacker.example.test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "blocked address")
}

func TestDatasourceDialPinsAllowedVPNAddress(t *testing.T) {
	executor := &SQLToolExecutor{
		resolveHost: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("10.42.0.15")}}, nil
		},
	}

	host, err := executor.resolveDatasourceHost(context.Background(), "vpn-db.example.test")
	require.NoError(t, err)
	assert.Equal(t, "10.42.0.15", host)
}

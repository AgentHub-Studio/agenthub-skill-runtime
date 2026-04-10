package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateURL_RFC1918_10x_Blocked(t *testing.T) {
	err := ValidateURL("http://10.0.0.1/api/data")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private")
}

func TestValidateURL_RFC1918_172x_Blocked(t *testing.T) {
	err := ValidateURL("http://172.16.0.1/api")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private")
}

func TestValidateURL_RFC1918_192168_Blocked(t *testing.T) {
	err := ValidateURL("http://192.168.1.1/api")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private")
}

func TestValidateURL_Loopback_Blocked(t *testing.T) {
	err := ValidateURL("http://127.0.0.1/api")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private")
}

func TestValidateURL_LoopbackLocalhost_Blocked(t *testing.T) {
	err := ValidateURL("http://127.0.0.1:8080/internal")
	require.Error(t, err)
}

func TestValidateURL_LinkLocal_Blocked(t *testing.T) {
	err := ValidateURL("http://169.254.169.254/latest/meta-data/") // AWS metadata
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private")
}

func TestValidateURL_CGNAT_Blocked(t *testing.T) {
	err := ValidateURL("http://100.64.0.1/api")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private")
}

func TestValidateURL_ClusterLocal_Blocked(t *testing.T) {
	err := ValidateURL("http://keycloak.agenthub.svc.cluster.local/auth")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cluster")
}

func TestValidateURL_ClusterLocalSuffix_Blocked(t *testing.T) {
	err := ValidateURL("http://minio.cluster.local/buckets")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cluster")
}

func TestValidateURL_InvalidURL_ReturnsError(t *testing.T) {
	err := ValidateURL("not-a-url")
	// "not-a-url" parses as a relative URL with no host — should return an error.
	require.Error(t, err)
}

func TestValidateURL_EmptyHost_ReturnsError(t *testing.T) {
	err := ValidateURL("http:///path")
	require.Error(t, err)
}

func TestValidateURL_IPv6Loopback_Blocked(t *testing.T) {
	err := ValidateURL("http://[::1]/api")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private")
}

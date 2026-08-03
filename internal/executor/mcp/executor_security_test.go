package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStdioCommandAllowlistPinsApprovedExecutable(t *testing.T) {
	command := filepath.Join(t.TempDir(), "mcp-server")
	require.NoError(t, os.WriteFile(command, []byte("#!/bin/sh\nexit 0\n"), 0o755))

	allowlist, err := parseStdioCommandAllowlist(command)
	require.NoError(t, err)

	resolved, err := resolveStdioCommand(command, allowlist)
	require.NoError(t, err)
	assert.Equal(t, command, resolved)
}

func TestParseStdioCommandAllowlistRejectsRelativeCommand(t *testing.T) {
	_, err := parseStdioCommandAllowlist("mcp-server")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "absolute")
}

func TestNewMCPToolExecutorLoadsDeploymentAllowlist(t *testing.T) {
	command := filepath.Join(t.TempDir(), "mcp-server")
	require.NoError(t, os.WriteFile(command, []byte("#!/bin/sh\nexit 0\n"), 0o755))
	t.Setenv(mcpStdioAllowedCommandsEnv, command)

	executor := NewMCPToolExecutor(nil)
	require.NoError(t, executor.stdioCommandConfigErr)

	resolved, err := resolveStdioCommand(command, executor.stdioCommandAllowlist)
	require.NoError(t, err)
	assert.Equal(t, command, resolved)
}

func TestResolveStdioCommandRejectsTenantCommandOutsideAllowlist(t *testing.T) {
	approved := filepath.Join(t.TempDir(), "approved-mcp-server")
	require.NoError(t, os.WriteFile(approved, []byte("#!/bin/sh\nexit 0\n"), 0o755))

	allowlist, err := parseStdioCommandAllowlist(approved)
	require.NoError(t, err)

	_, err = resolveStdioCommand("/bin/sh", allowlist)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "allowlist")
}

func TestStdioEnvironmentDoesNotInheritOrPermitLoaderOverrides(t *testing.T) {
	environment, err := stdioEnvironment(map[string]string{"API_TOKEN": "tenant-secret"})
	require.NoError(t, err)
	assert.Equal(t, []string{"PATH=" + defaultStdioCommandPath, "API_TOKEN=tenant-secret"}, environment)

	_, err = stdioEnvironment(map[string]string{"LD_PRELOAD": "/tmp/untrusted.so"})
	require.Error(t, err)

	_, err = stdioEnvironment(map[string]string{"PATH": "/tmp"})
	require.Error(t, err)
}

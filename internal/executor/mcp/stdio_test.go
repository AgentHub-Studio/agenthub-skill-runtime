package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestExecuteStdioRunsHandshakeAndTool(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	t.Setenv("DATABASE_URL", "runtime-secret-must-not-reach-mcp")
	allowlist, err := parseStdioCommandAllowlist(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}

	result, err := executeStdio(ctx, &mcpServerConfig{
		TransportType: "stdio",
		Command:       os.Args[0],
		Args:          []string{"-test.run=TestExecuteStdioHelperProcess", "--"},
		Env: map[string]string{
			"GO_WANT_SKILL_MCP_HELPER": "1",
			"SKILL_MCP_TEST_VALUE":     "available",
		},
	}, "echo", map[string]any{"message": "hello"}, allowlist)

	if err != nil {
		t.Fatal(err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected MCP result error: %s", result.Error)
	}
	if result.Output["echo"] != "hello" || result.Output["env"] != "available" {
		t.Fatalf("unexpected MCP output: %#v", result.Output)
	}
	if result.Output["database"] != "" {
		t.Fatalf("stdio MCP child inherited runtime secret: %#v", result.Output)
	}
}

// TestExecuteStdioHelperProcess runs in a real subprocess started by the
// executor test. It verifies the initialize notification precedes tools/call
// and receives the configured environment without using a network mock.
func TestExecuteStdioHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_SKILL_MCP_HELPER") != "1" {
		return
	}

	initialized := false
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024), maxStdioRPCMessageSize)
	for scanner.Scan() {
		var request mcpRequest
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}

		switch request.Method {
		case "initialize":
			writeSkillHelperResponse(mcpResponse{
				JSONRPC: "2.0",
				ID:      request.ID,
				Result:  map[string]any{"protocolVersion": "2025-03-26"},
			})
		case "notifications/initialized":
			initialized = true
		case "tools/call":
			if !initialized {
				fmt.Fprintln(os.Stderr, "tools/call arrived before initialized notification")
				os.Exit(2)
			}
			message, _ := request.Params["arguments"].(map[string]any)["message"].(string)
			writeSkillHelperResponse(mcpResponse{
				JSONRPC: "2.0",
				ID:      request.ID,
				Result: map[string]any{
					"echo":     message,
					"env":      os.Getenv("SKILL_MCP_TEST_VALUE"),
					"database": os.Getenv("DATABASE_URL"),
				},
			})
		default:
			writeSkillHelperResponse(mcpResponse{
				JSONRPC: "2.0",
				ID:      request.ID,
				Error:   &mcpError{Code: -32601, Message: "method not found"},
			})
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Exit(0)
}

func writeSkillHelperResponse(response mcpResponse) {
	data, err := json.Marshal(response)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Println(string(data))
}

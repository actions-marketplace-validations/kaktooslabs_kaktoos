//go:build e2e

package e2e

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// mcpSpec is a minimal spec whose 200 response requires both id and name.
const mcpSpec = `
openapi: 3.0.3
info: {title: T, version: 1.0.0}
paths:
  /users/{id}:
    get:
      operationId: getUser
      parameters:
        - name: id
          in: path
          required: true
          schema: {type: string}
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                required: [id, name]
                properties:
                  id: {type: string}
                  name: {type: string}
`

const mcpScenario = `
name: get user
steps:
  - name: get
    operation: getUser
    request:
      path:
        id: "1"
`

// mcpClient drives a `kaktoos mcp` subprocess over real newline-delimited
// JSON-RPC frames on stdin/stdout.
type mcpClient struct {
	t   *testing.T
	cmd *exec.Cmd
	in  io.WriteCloser
	out *bufio.Reader
	id  int
}

func startMCP(t *testing.T) *mcpClient {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(root, "kaktoos-mcp-test")
	build := exec.Command("go", "build", "-o", binary, "./cmd/kaktoos")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, output)
	}

	cmd := exec.Command(binary, "mcp")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stdin.Close()
		cmd.Process.Kill()
		cmd.Wait()
		os.Remove(binary)
	})

	c := &mcpClient{t: t, cmd: cmd, in: stdin, out: bufio.NewReader(stdout)}
	c.handshake()
	return c
}

// send writes a request and returns the matching response, skipping any
// notifications the server emits in between.
func (c *mcpClient) send(method string, params any) map[string]any {
	c.t.Helper()
	c.id++
	id := c.id
	req := map[string]any{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		req["params"] = params
	}
	c.write(req)

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		line, err := c.out.ReadBytes('\n')
		if err != nil {
			c.t.Fatalf("reading response to %s: %v", method, err)
		}
		var msg map[string]any
		if err := json.Unmarshal(line, &msg); err != nil {
			c.t.Fatalf("non-JSON frame from server: %q", line)
		}
		if got, ok := msg["id"].(float64); !ok || int(got) != id {
			continue // a notification or an unrelated response
		}
		if e, ok := msg["error"]; ok {
			c.t.Fatalf("%s returned a JSON-RPC error: %v", method, e)
		}
		return msg
	}
	c.t.Fatalf("timed out waiting for a response to %s", method)
	return nil
}

func (c *mcpClient) write(msg any) {
	c.t.Helper()
	data, err := json.Marshal(msg)
	if err != nil {
		c.t.Fatal(err)
	}
	if _, err := fmt.Fprintf(c.in, "%s\n", data); err != nil {
		c.t.Fatalf("writing to server stdin: %v", err)
	}
}

func (c *mcpClient) handshake() {
	c.t.Helper()
	c.send("initialize", map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "kaktoos-e2e", "version": "0.0.1"},
	})
	c.write(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
}

// callTool returns the tool's structured content.
func (c *mcpClient) callTool(name string, args map[string]any) map[string]any {
	c.t.Helper()
	resp := c.send("tools/call", map[string]any{"name": name, "arguments": args})
	result, ok := resp["result"].(map[string]any)
	if !ok {
		c.t.Fatalf("tools/call %s returned no result: %v", name, resp)
	}
	if isErr, _ := result["isError"].(bool); isErr {
		c.t.Fatalf("tool %s reported an error: %v", name, result["content"])
	}
	structured, ok := result["structuredContent"].(map[string]any)
	if !ok {
		c.t.Fatalf("tool %s returned no structuredContent: %v", name, result)
	}
	return structured
}

func writeMCPSpec(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "openapi.yml")
	if err := os.WriteFile(p, []byte(mcpSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestE2E_MCP_ToolsList(t *testing.T) {
	c := startMCP(t)
	resp := c.send("tools/list", map[string]any{})
	result := resp["result"].(map[string]any)
	tools, _ := result["tools"].([]any)

	got := map[string]bool{}
	for _, tool := range tools {
		got[tool.(map[string]any)["name"].(string)] = true
	}
	for _, want := range []string{"list_operations", "validate_scenario", "run_workflow"} {
		if !got[want] {
			t.Errorf("tool %q missing from tools/list; got %v", want, got)
		}
	}
	if len(tools) != 3 {
		t.Errorf("expected exactly three tools, got %d", len(tools))
	}
}

func TestE2E_MCP_ListOperations(t *testing.T) {
	c := startMCP(t)
	out := c.callTool("list_operations", map[string]any{"openapi_path": writeMCPSpec(t)})
	ops, _ := out["operations"].([]any)
	if len(ops) != 1 {
		t.Fatalf("expected one operation, got %v", out)
	}
	if id := ops[0].(map[string]any)["operation_id"]; id != "getUser" {
		t.Errorf("expected getUser, got %v", id)
	}
}

func TestE2E_MCP_ValidateScenario(t *testing.T) {
	c := startMCP(t)
	spec := writeMCPSpec(t)

	out := c.callTool("validate_scenario", map[string]any{"scenario_yaml": mcpScenario, "openapi_path": spec})
	if valid, _ := out["valid"].(bool); !valid {
		t.Errorf("expected the scenario to validate, got %v", out)
	}

	bad := strings.Replace(mcpScenario, "getUser", "getUsr", 1)
	out = c.callTool("validate_scenario", map[string]any{"scenario_yaml": bad, "openapi_path": spec})
	if valid, _ := out["valid"].(bool); valid {
		t.Errorf("expected the typo'd operation to be rejected, got %v", out)
	}
}

// TestE2E_MCP_RunWorkflow proves the whole stack over the real transport: a
// broken response surfaces a pinpointed schema violation, a fixed one passes.
func TestE2E_MCP_RunWorkflow(t *testing.T) {
	broken := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if broken {
			w.Write([]byte(`{"id":"1"}`)) // name is required but absent
			return
		}
		w.Write([]byte(`{"id":"1","name":"n"}`))
	}))
	defer server.Close()

	c := startMCP(t)
	args := map[string]any{
		"scenario_yaml": mcpScenario,
		"openapi_path":  writeMCPSpec(t),
		"environment":   fmt.Sprintf("base_url: %s\n", server.URL),
	}

	out := c.callTool("run_workflow", args)
	if passed, _ := out["passed"].(bool); passed {
		t.Fatalf("expected the broken response to fail, got %v", out)
	}
	steps := out["steps"].([]any)
	violations, _ := steps[0].(map[string]any)["schema_violations"].([]any)
	if len(violations) != 1 {
		t.Fatalf("expected one schema violation, got %v", steps[0])
	}
	v := violations[0].(map[string]any)
	if v["kind"] != "required_field_missing" || v["path"] != "$.name" {
		t.Errorf("expected required_field_missing at $.name, got %v", v)
	}

	broken = false
	out = c.callTool("run_workflow", args)
	if passed, _ := out["passed"].(bool); !passed {
		t.Fatalf("expected the fixed response to pass, got %v", out)
	}
}

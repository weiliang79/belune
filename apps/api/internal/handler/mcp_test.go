package handler_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weiliang79/belune/internal/testutil"
)

// mcpHeaders builds the headers a streamable-HTTP MCP client sends: a Bearer
// token plus an Accept header advertising both response forms — the
// stateless transport rejects a POST missing either, per the streamable HTTP
// spec, regardless of which one the server actually replies with.
func mcpHeaders(token string) map[string]string {
	h := testutil.AuthHeader(token)
	h["Accept"] = "application/json, text/event-stream"
	return h
}

// callTool issues one JSON-RPC tools/call request and returns the raw HTTP
// response — callers decode it differently depending on whether they expect
// a JSON-RPC envelope (success) or the plain {"error":...} body a
// middleware rejection writes before the request ever reaches the MCP
// server.
func callTool(t *testing.T, token, name string) *http.Response {
	t.Helper()
	return env.DoRequest(t, "POST", "/mcp", map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      name,
			"arguments": map[string]any{},
		},
	}, mcpHeaders(token))
}

type jsonRPCToolResult struct {
	Result *struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// TestMCP_ReadScopedTokenCanListProjects is the PR1 checkpoint: transport,
// auth and one tool all wired correctly end to end.
func TestMCP_ReadScopedTokenCanListProjects(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "mcp-admin@test.com", "password123")
	project := env.CreateProject(t, adminToken, "MCP Project", "mcp-project")
	projectID := extractID(project["id"])

	readToken := mintScoped(t, adminToken, []string{"read"})

	resp := callTool(t, readToken, "list_projects")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var rpc jsonRPCToolResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&rpc))
	require.Nil(t, rpc.Error, "tools/call returned a JSON-RPC error: %+v", rpc.Error)
	require.NotNil(t, rpc.Result)
	require.Len(t, rpc.Result.Content, 1)
	assert.Equal(t, "text", rpc.Result.Content[0].Type)

	var projects []map[string]any
	require.NoError(t, json.Unmarshal([]byte(rpc.Result.Content[0].Text), &projects))
	require.Len(t, projects, 1)
	assert.Equal(t, projectID, projects[0]["id"])
	assert.Equal(t, "mcp-project", projects[0]["slug"])
}

// TestMCP_ScopeEnforced asserts the trap the build plan called out twice:
// RequireScopeByMethod would have demanded "write" for this POST, the exact
// inverse of a read-only server. A metrics-only token (the one scope that
// does NOT satisfy "read" in the lattice) must be rejected before the
// request ever reaches the MCP server.
func TestMCP_ScopeEnforced(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "mcp-scope@test.com", "password123")

	metricsToken := mintScoped(t, adminToken, []string{"metrics"})
	resp := callTool(t, metricsToken, "list_projects")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	body := testutil.ReadJSON(t, resp)
	assert.Equal(t, "token lacks required scope: read", body["error"])
}

// TestMCP_SessionRejected pins the design decision that /mcp accepts only a
// personal access token: MCP clients are machines, and accepting a session
// JWT there would blur the boundary the PAT model draws (and would skip
// CSRF, which already exempts the Bearer branch, with no compensating
// control). A session JWT authenticates fine everywhere else, so this must
// be rejected structurally, not merely by scope.
func TestMCP_SessionRejected(t *testing.T) {
	resetDB(t)
	adminToken := env.SetupAdmin(t, "mcp-session@test.com", "password123")

	resp := callTool(t, adminToken, "list_projects")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	body := testutil.ReadJSON(t, resp)
	assert.Equal(t, "this action requires a personal access token, not a session", body["error"])
}

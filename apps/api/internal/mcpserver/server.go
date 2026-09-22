// Package mcpserver implements Belune's read-only Model Context Protocol
// server. It is registered as a single stateless JSON-RPC-over-HTTP handler
// at POST /mcp (see internal/server/routes.go) so an AI assistant can inspect
// projects, applications, deployments and infrastructure state through the
// same personal-access-token model a CI script would use.
//
// Phase 1 (0.1.x #4) is read-only: mutating tools are out of scope
// permanently, not just for this release, so registerXTools functions in
// this package must never add one. Tool handlers read request identity
// (role, user id, project pin) off the context via internal/server/middleware
// accessors — the SDK threads the originating *http.Request's context
// through to every tool call, so the same Auth/RequireScope/RequireToken
// chain that gates the route gates each tool invocation too.
package mcpserver

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/weiliang79/belune/internal/store/generated"
	"github.com/weiliang79/belune/internal/version"
)

// New builds the MCP server and wraps it in a stateless streamable-HTTP
// handler. Stateless + JSONResponse: every tool here is a plain
// request/response with nothing server-initiated, so there is no SSE stream,
// no session store, and nothing lost on a control-plane restart — each
// request stands alone, authenticated by its own Bearer token exactly like
// REST.
func New(queries *generated.Queries) http.Handler {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "belune",
		Version: version.Version,
	}, nil)

	registerProjectTools(srv, queries)

	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return srv
	}, &mcp.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
	})
}

package handler

import "net/http"

// HandleMCP serves the read-only MCP (Model Context Protocol) server. The
// route (POST /mcp) gates identity and access — RequireToken, RequireScope,
// and RequireProjectAccess all run before this is reached — the transport
// and tools themselves live in internal/mcpserver.
//
// Deliberately excluded from the generated OpenAPI reference: it is a single
// stateless JSON-RPC endpoint, not a REST resource, so documenting it as one
// operation with an opaque body would mislead more than it would inform. See
// apidocSkipNonRESTPrefixes in apidoc_generate_test.go.
func (h *Handler) HandleMCP(w http.ResponseWriter, r *http.Request) {
	h.mcpHandler.ServeHTTP(w, r)
}

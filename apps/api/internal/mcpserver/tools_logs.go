package mcpserver

import (
	"bytes"
	"context"
	"log/slog"
	"strings"

	"github.com/docker/docker/pkg/stdcopy"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/weiliang79/belune/internal/naming"
	"github.com/weiliang79/belune/internal/runtime"
	"github.com/weiliang79/belune/internal/store/generated"
)

const (
	defaultLogTailLines = 200
	maxLogTailLines     = 1000
)

type applicationLogsInput struct {
	ApplicationID string `json:"application_id" jsonschema:"the application's id"`
	Tail          int    `json:"tail,omitempty" jsonschema:"number of most recent log lines to return (default 200, max 1000)"`
}

// registerLogTools registers the one tool in this package that reaches
// outside the database: a live container log tail, read straight from the
// runtime rather than the historical container_logs table.
//
// ⚠️ Uses ContainerLogsTail, never ContainerLogs — the latter is unbounded
// and is what the platform log viewer explicitly avoids reading in full.
// Logs are returned verbatim, with no redaction beyond what
// internal/pkg/redact already strips elsewhere in the product: connection
// strings and API keys printed at container boot land in the tool's output
// exactly as Docker recorded them. Whoever connects a client to this server
// is choosing to hand it that.
func registerLogTools(srv *mcp.Server, queries *generated.Queries, runtimes runtime.Runtimes) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_application_logs",
		Description: "Tail an application's live container output — the last N lines, not a " +
			"stream. Returned verbatim: container logs often contain connection strings or API " +
			"keys printed at startup. Defaults to 200 lines, capped at 1000.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in applicationLogsInput) (*mcp.CallToolResult, any, error) {
		appID, err := parseUUID(in.ApplicationID)
		if err != nil {
			return nil, nil, err
		}
		// Not authorizeApplication: that would re-join applications to
		// projects a second time via GetApplicationOwnerUserID for data this
		// query's own join to projects (for project_slug/server_id) already
		// has. Same pin + ownership check, inlined against this row instead.
		row, err := queries.GetApplicationLogAccess(ctx, appID)
		if err != nil {
			return nil, nil, notFoundOr(ctx, "application not found")
		}
		if !pinAllows(ctx, uuidToString(row.ProjectID)) {
			return nil, nil, errAccessDenied
		}
		if !canAccessOwned(ctx, row.ProjectUserID, row.ProjectShared) {
			return nil, nil, errAccessDenied
		}

		tail := in.Tail
		if tail <= 0 {
			tail = defaultLogTailLines
		}
		if tail > maxLogTailLines {
			tail = maxLogTailLines
		}

		rt, err := runtimes.For(ctx, row.ServerID)
		if err != nil {
			return nil, nil, internalError("failed to reach the application's server", err)
		}

		containerName := naming.ContainerName(row.ProjectSlug, row.Slug, uuidToString(row.ID))
		rc, err := rt.ContainerLogsTail(ctx, containerName, tail)
		if err != nil {
			return nil, nil, internalError("failed to read container logs", err)
		}
		defer rc.Close()

		// The container is not a TTY, so the stream is stdcopy-multiplexed: an
		// 8-byte header per frame whose length bytes are frequently printable
		// ASCII, so reading it raw puts stray letters in front of real log
		// text. Demuxed the same way GetPlatformLogs and
		// updateHelperFailureReason do. Unlike those, the RFC3339Nano
		// timestamp Docker prefixes every line with is kept rather than
		// stripped — a display pane already has its own relative-time chrome,
		// but an assistant reading this text has only what's in it, and "when
		// did this happen" is exactly the kind of thing worth keeping.
		var buf bytes.Buffer
		_, copyErr := stdcopy.StdCopy(&buf, &buf, rc)
		if copyErr != nil && buf.Len() == 0 {
			return nil, nil, internalError("failed to read container logs", copyErr)
		}

		// Container output is arbitrary bytes, not guaranteed valid UTF-8 — a
		// binary crash dump, or a multi-byte sequence cut off at the tail
		// boundary. json.Marshal would silently substitute U+FFFD for any
		// invalid byte anyway; doing it explicitly here makes that a
		// documented choice instead of an implicit side effect.
		text := strings.ToValidUTF8(buf.String(), "�")
		if copyErr != nil {
			// Some bytes were captured before the stream ended abnormally —
			// still worth returning, but silently presenting a partial read
			// as a complete one would be worse than flagging it. Logged
			// server-side (see internalError's own reasoning) rather than
			// embedding copyErr's raw text in what's otherwise log content.
			slog.Warn("mcpserver: container log stream ended before EOF, returning a truncated tail", "error", copyErr)
			text += "\n[... log read ended early; output may be truncated]"
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})
}

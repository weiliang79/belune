package mcpserver

import (
	"bytes"
	"context"
	"errors"
	"fmt"

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
		row, err := queries.GetApplicationWithProjectSlug(ctx, appID)
		if err != nil {
			return nil, nil, errors.New("application not found")
		}
		if err := authorizeApplication(ctx, queries, row.ID, row.ProjectID); err != nil {
			return nil, nil, err
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
			return nil, nil, fmt.Errorf("reaching the application's server: %w", err)
		}

		containerName := naming.ContainerName(row.ProjectSlug, row.Slug, uuidToString(row.ID))
		rc, err := rt.ContainerLogsTail(ctx, containerName, tail)
		if err != nil {
			return nil, nil, fmt.Errorf("reading container logs: %w", err)
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
		if _, err := stdcopy.StdCopy(&buf, &buf, rc); err != nil && buf.Len() == 0 {
			return nil, nil, fmt.Errorf("reading container logs: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: buf.String()}},
		}, nil, nil
	})
}

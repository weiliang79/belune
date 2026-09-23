package mcpserver

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/weiliang79/belune/internal/server/middleware"
	"github.com/weiliang79/belune/internal/store/generated"
)

// project is the tool-facing shape of a project row — narrower than the
// generated row, which carries server_id and other fields with no reader
// value here.
type project struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Shared    bool   `json:"shared"`
	CreatedAt string `json:"created_at"`
}

func registerProjectTools(srv *mcp.Server, queries *generated.Queries) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_projects",
		Description: "List every project the caller's token can reach: every project on the " +
			"install for an admin token, or the token owner's own projects plus any shared with " +
			"them otherwise. A project-pinned token sees only its pinned project.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		projects, err := listProjectsForCaller(ctx, queries)
		if err != nil {
			return nil, nil, err
		}
		return textResult(projects)
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_project",
		Description: "Get one project by id, including whether it is shared with other Members.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in projectIDInput) (*mcp.CallToolResult, any, error) {
		id, err := parseUUID(in.ProjectID)
		if err != nil {
			return nil, nil, err
		}
		p, err := authorizeProject(ctx, queries, id)
		if err != nil {
			return nil, nil, err
		}
		return textResult(toProject(p.ID, p.Name, p.Slug, p.Shared, p.CreatedAt))
	})
}

func toProject(id pgtype.UUID, name, slug string, shared bool, createdAt pgtype.Timestamptz) project {
	return project{
		ID:        uuidToString(id),
		Name:      name,
		Slug:      slug,
		Shared:    shared,
		CreatedAt: formatTimestamp(createdAt),
	}
}

// listProjectsForCaller mirrors handler.Handler.ListProjects: an admin sees
// every project on the install, a member sees their own and any shared with
// them, and a project-pinned token is narrowed to that one project. The rule
// is reimplemented here, not called into the handler package, because a tool
// call has no http.ResponseWriter to hand a handler method — and because
// internal/handler will need to import this package to wire the route,
// so the reverse import isn't available.
func listProjectsForCaller(ctx context.Context, queries *generated.Queries) ([]project, error) {
	role := middleware.RoleFromContext(ctx)
	pinned := middleware.TokenProjectFromContext(ctx)

	if role == "admin" {
		rows, err := queries.ListAllProjects(ctx)
		if err != nil {
			return nil, internalError("failed to list projects", err)
		}
		return mapProjects(rows, pinned, func(p generated.ListAllProjectsRow) (pgtype.UUID, string, string, bool, pgtype.Timestamptz) {
			return p.ID, p.Name, p.Slug, p.Shared, p.CreatedAt
		}), nil
	}

	var userID pgtype.UUID
	if err := userID.Scan(middleware.UserIDFromContext(ctx)); err != nil {
		return nil, internalError("failed to list projects", err)
	}

	rows, err := queries.ListProjectsByUser(ctx, userID)
	if err != nil {
		return nil, internalError("failed to list projects", err)
	}
	return mapProjects(rows, pinned, func(p generated.ListProjectsByUserRow) (pgtype.UUID, string, string, bool, pgtype.Timestamptz) {
		return p.ID, p.Name, p.Slug, p.Shared, p.CreatedAt
	}), nil
}

// mapProjects narrows rows to the pinned project id (when pinned is
// non-empty) and converts each to the tool-facing shape. Always returns a
// non-nil slice so the tool's JSON output is "[]", never "null", when
// nothing matches.
func mapProjects[T any](rows []T, pinned string, fields func(T) (pgtype.UUID, string, string, bool, pgtype.Timestamptz)) []project {
	out := make([]project, 0, len(rows))
	for _, r := range rows {
		id, name, slug, shared, createdAt := fields(r)
		idStr := uuidToString(id)
		if pinned != "" && idStr != pinned {
			continue
		}
		out = append(out, toProject(id, name, slug, shared, createdAt))
	}
	return out
}

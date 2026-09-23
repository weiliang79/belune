package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/weiliang79/belune/internal/server/middleware"
	"github.com/weiliang79/belune/internal/store/generated"
)

var errAccessDenied = errors.New("access denied")

// internalError logs the real cause server-side and returns a generic
// message as the tool's error. Returning err directly would embed its text
// — a raw DB error can name a table, column or constraint — verbatim in
// CallToolResult's content, sent to whichever client is connected. This is
// the same boundary handler.writeError already draws for the REST API;
// nothing in this package logged failures before this existed, so an
// operator had no server-side signal either.
func internalError(msg string, err error) error {
	slog.Error("mcpserver: "+msg, "error", err)
	return errors.New(msg)
}

// notFoundOr reports a resource lookup failure as errAccessDenied for a
// non-admin caller, and as the real notFoundMsg for an admin — mirroring
// handler.canAccessOwned, which treats an owner-lookup failure as a plain
// access failure for anyone but an admin. Collapsing "doesn't exist" into
// "access denied" for a non-admin is deliberate: distinguishing the two
// would let a token use a resource's mere existence as a cross-tenant
// oracle, which canAccessOwned was built to prevent on the REST side.
func notFoundOr(ctx context.Context, notFoundMsg string) error {
	if middleware.RoleFromContext(ctx) == "admin" {
		return errors.New(notFoundMsg)
	}
	return errAccessDenied
}

func uuidToString(u pgtype.UUID) string {
	return uuid.UUID(u.Bytes).String()
}

func parseUUID(s string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(s); err != nil {
		return id, errors.New("invalid id")
	}
	return id, nil
}

// formatTimestamp renders a nullable timestamp as RFC3339, or "" (which
// callers pair with an `omitempty` json tag) when the column was never set.
func formatTimestamp(t pgtype.Timestamptz) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Format(time.RFC3339)
}

// textResult wraps a value as the tool's sole text content block, the shape
// every tool in this package returns. A tool-level failure should be
// returned as the handler's error instead of calling this — the SDK packs a
// returned error into CallToolResult with IsError set, which is how a
// caller's client distinguishes "tool ran, here's the answer" from "the call
// failed" without inspecting content itself.
func textResult(v any) (*mcp.CallToolResult, any, error) {
	body, err := json.Marshal(v)
	if err != nil {
		return nil, nil, internalError("failed to encode result", err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(body)}},
	}, nil, nil
}

// canAccessOwned mirrors handler.canAccessOwned: an admin always passes, a
// shared project grants every Member the same access as its owner, and
// otherwise only the owner passes. Kept in this package (not called from
// internal/handler) because internal/handler imports this package to wire
// the /mcp route, so the reverse import isn't available.
func canAccessOwned(ctx context.Context, ownerID pgtype.UUID, shared bool) bool {
	if middleware.RoleFromContext(ctx) == "admin" {
		return true
	}
	if shared {
		return true
	}
	var userID pgtype.UUID
	userID.Scan(middleware.UserIDFromContext(ctx))
	return ownerID == userID
}

// pinAllows reports whether a project-pinned token may reach projectID —
// true for an unpinned token or one pinned to exactly this project. Every
// tool that resolves a project id must call this: unlike a REST route, an
// MCP tool call has no {projectId} URL param for
// middleware.RequireProjectAccess to enforce the pin against, so each tool
// does it itself.
func pinAllows(ctx context.Context, projectID string) bool {
	pinned := middleware.TokenProjectFromContext(ctx)
	return pinned == "" || pinned == projectID
}

// authorizeProject checks pin + ownership for a project id supplied
// directly as a tool argument, and returns the fetched row so callers
// building a project DTO (get_project) don't fetch it twice.
func authorizeProject(ctx context.Context, queries *generated.Queries, projectID pgtype.UUID) (generated.Project, error) {
	if !pinAllows(ctx, uuidToString(projectID)) {
		return generated.Project{}, errAccessDenied
	}
	project, err := queries.GetProject(ctx, projectID)
	if err != nil {
		return generated.Project{}, notFoundOr(ctx, "project not found")
	}
	if !canAccessOwned(ctx, project.UserID, project.Shared) {
		return generated.Project{}, errAccessDenied
	}
	return project, nil
}

// authorizeApplication checks pin + ownership given an application's own id
// and its parent project id — both already in hand from whatever row lookup
// (GetApplication, GetApplicationWithProjectSlug) the caller did to do its
// own job, so this never fetches the application itself.
func authorizeApplication(ctx context.Context, queries *generated.Queries, applicationID, projectID pgtype.UUID) error {
	if !pinAllows(ctx, uuidToString(projectID)) {
		return errAccessDenied
	}
	owner, err := queries.GetApplicationOwnerUserID(ctx, applicationID)
	if err != nil {
		return notFoundOr(ctx, "application not found")
	}
	if !canAccessOwned(ctx, owner.UserID, owner.Shared) {
		return errAccessDenied
	}
	return nil
}

// authorizeDatabase mirrors authorizeApplication for a database.
func authorizeDatabase(ctx context.Context, queries *generated.Queries, databaseID, projectID pgtype.UUID) error {
	if !pinAllows(ctx, uuidToString(projectID)) {
		return errAccessDenied
	}
	owner, err := queries.GetDatabaseOwnerUserID(ctx, databaseID)
	if err != nil {
		return notFoundOr(ctx, "database not found")
	}
	if !canAccessOwned(ctx, owner.UserID, owner.Shared) {
		return errAccessDenied
	}
	return nil
}

// projectIDInput is the shared argument shape for every tool scoped to one
// project (get_project, list_applications, list_databases,
// list_project_backups).
type projectIDInput struct {
	ProjectID string `json:"project_id" jsonschema:"the project's id"`
}

// applicationIDInput is the shared argument shape for every tool scoped to
// one application (get_application, list_deployments, get_application_logs).
type applicationIDInput struct {
	ApplicationID string `json:"application_id" jsonschema:"the application's id"`
}

// databaseIDInput is the shared argument shape for every tool scoped to one
// database (get_database).
type databaseIDInput struct {
	DatabaseID string `json:"database_id" jsonschema:"the database's id"`
}

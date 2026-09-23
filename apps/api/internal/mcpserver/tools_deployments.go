package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/weiliang79/belune/internal/store/generated"
)

const (
	defaultDeploymentsLimit = 20
	maxDeploymentsLimit     = 100
)

// deployment is the tool-facing shape of a deployment row. build_logs is
// deliberately omitted — like a container log tail, a build log has no
// bound on this row and belongs to a future tool that can apply one, not a
// field tagging along on every list call.
type deployment struct {
	ID              string `json:"id"`
	Status          string `json:"status"`
	TriggeredBy     string `json:"triggered_by"`
	CommitSHA       string `json:"commit_sha,omitempty"`
	CommitMessage   string `json:"commit_message,omitempty"`
	CommitAuthor    string `json:"commit_author,omitempty"`
	ImageTag        string `json:"image_tag,omitempty"`
	ErrorMessage    string `json:"error_message,omitempty"`
	HealthStatus    string `json:"health_status,omitempty"`
	HealthMessage   string `json:"health_message,omitempty"`
	StartedAt       string `json:"started_at,omitempty"`
	BuildStartedAt  string `json:"build_started_at,omitempty"`
	BuildEndedAt    string `json:"build_ended_at,omitempty"`
	DeployStartedAt string `json:"deploy_started_at,omitempty"`
	FinishedAt      string `json:"finished_at,omitempty"`
}

func toDeployment(d generated.Deployment) deployment {
	return deployment{
		ID:              uuidToString(d.ID),
		Status:          d.Status,
		TriggeredBy:     d.TriggeredBy,
		CommitSHA:       d.CommitSha.String,
		CommitMessage:   d.CommitMessage.String,
		CommitAuthor:    d.CommitAuthor.String,
		ImageTag:        d.ImageTag.String,
		ErrorMessage:    d.ErrorMessage.String,
		HealthStatus:    d.HealthStatus.String,
		HealthMessage:   d.HealthMessage.String,
		StartedAt:       formatTimestamp(d.StartedAt),
		BuildStartedAt:  formatTimestamp(d.BuildStartedAt),
		BuildEndedAt:    formatTimestamp(d.BuildEndedAt),
		DeployStartedAt: formatTimestamp(d.DeployStartedAt),
		FinishedAt:      formatTimestamp(d.FinishedAt),
	}
}

type listDeploymentsInput struct {
	ApplicationID string `json:"application_id" jsonschema:"the application's id"`
	Limit         int    `json:"limit,omitempty" jsonschema:"maximum number of deployments to return, newest first (default 20, max 100)"`
}

func registerDeploymentTools(srv *mcp.Server, queries *generated.Queries) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_deployments",
		Description: "List an application's deployments, newest first: status, build outcome, " +
			"commit info, and the post-deploy health-check result. Bounded — defaults to 20, capped at 100.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listDeploymentsInput) (*mcp.CallToolResult, any, error) {
		appID, err := parseUUID(in.ApplicationID)
		if err != nil {
			return nil, nil, err
		}
		app, err := queries.GetApplication(ctx, appID)
		if err != nil {
			return nil, nil, notFoundOr(ctx, "application not found")
		}
		if err := authorizeApplication(ctx, queries, app.ID, app.ProjectID); err != nil {
			return nil, nil, err
		}

		limit := clampLimit(in.Limit, defaultDeploymentsLimit, maxDeploymentsLimit)

		rows, err := queries.ListRecentDeploymentsByApplication(ctx, generated.ListRecentDeploymentsByApplicationParams{
			ApplicationID: appID,
			Limit:         int32(limit),
		})
		if err != nil {
			return nil, nil, internalError("failed to list deployments", err)
		}

		out := make([]deployment, 0, len(rows))
		for _, d := range rows {
			out = append(out, toDeployment(d))
		}
		return textResult(out)
	})
}

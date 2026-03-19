import { createFileRoute } from "@tanstack/react-router";
import { useApplication } from "@/lib/hooks/use-applications";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { formatDate } from "@/lib/utils/format";
import { StatusBadge } from "@/lib/components/status-badge";

export const Route = createFileRoute(
  "/_app/projects/$projectId/applications/$applicationId/",
)({
  component: ApplicationOverview,
});

function ApplicationOverview() {
  const { projectId, applicationId } = Route.useParams();
  const { data: application } = useApplication(projectId, applicationId);

  if (!application) return null;

  return (
    <div className="grid gap-4 sm:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle>Application Info</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          <div className="flex justify-between">
            <span className="text-muted-foreground">Type</span>
            <Badge variant="outline" className="capitalize">{application.type}</Badge>
          </div>
          {application.type === "git" && (
            <div className="flex justify-between">
              <span className="text-muted-foreground">Build Type</span>
              <Badge variant="outline" className="capitalize">{application.build_type}</Badge>
            </div>
          )}
          {application.source_image && (
            <div className="flex justify-between">
              <span className="text-muted-foreground">Image</span>
              <span className="font-mono text-xs">
                {application.source_image}
              </span>
            </div>
          )}
          {application.source_repo && (
            <div className="flex justify-between">
              <span className="text-muted-foreground">Repository</span>
              <a
                href={application.source_repo}
                target="_blank"
                rel="noopener noreferrer"
                className="max-w-64 truncate font-mono text-xs hover:underline"
              >
                {application.source_repo
                  .replace(/^https?:\/\//, "")
                  .replace(/\.git$/, "")}
              </a>
            </div>
          )}
          <div className="flex justify-between">
            <span className="text-muted-foreground">Status</span>
            <StatusBadge status={application.status} />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Timestamps</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          <div className="flex justify-between">
            <span className="text-muted-foreground">Created</span>
            <span>{formatDate(application.created_at)}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-muted-foreground">Updated</span>
            <span>{formatDate(application.updated_at)}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-muted-foreground">ID</span>
            <span className="font-mono text-xs">{application.id}</span>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

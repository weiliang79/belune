import { createFileRoute } from "@tanstack/react-router";
import { ContainerLogViewer } from "@/components/logs/container-log-viewer";

export const Route = createFileRoute(
  "/_app/projects/$projectId/applications/$applicationId/logs",
)({
  component: LogsPage,
});

function LogsPage() {
  const { projectId, applicationId } = Route.useParams();
  return (
    <ContainerLogViewer
      source="application"
      projectId={projectId}
      sourceId={applicationId}
    />
  );
}

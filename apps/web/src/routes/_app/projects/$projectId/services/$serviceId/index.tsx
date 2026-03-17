import { createFileRoute } from "@tanstack/react-router";
import { useService } from "@/lib/hooks/use-services";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { formatDate } from "@/lib/utils/format";

export const Route = createFileRoute(
  "/_app/projects/$projectId/services/$serviceId/",
)({
  component: ServiceOverview,
});

function ServiceOverview() {
  const { projectId, serviceId } = Route.useParams();
  const { data: service } = useService(projectId, serviceId);

  if (!service) return null;

  return (
    <div className="grid gap-4 sm:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle>Service Info</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          <div className="flex justify-between">
            <span className="text-muted-foreground">Type</span>
            <Badge variant="outline">{service.type}</Badge>
          </div>
          <div className="flex justify-between">
            <span className="text-muted-foreground">Build Type</span>
            <Badge variant="outline">{service.build_type}</Badge>
          </div>
          {service.source_image && (
            <div className="flex justify-between">
              <span className="text-muted-foreground">Image</span>
              <span className="font-mono text-xs">{service.source_image}</span>
            </div>
          )}
          {service.source_repo && (
            <div className="flex justify-between">
              <span className="text-muted-foreground">Repository</span>
              <span className="max-w-48 truncate font-mono text-xs">
                {service.source_repo}
              </span>
            </div>
          )}
          <div className="flex justify-between">
            <span className="text-muted-foreground">Status</span>
            <span>{service.status}</span>
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
            <span>{formatDate(service.created_at)}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-muted-foreground">Updated</span>
            <span>{formatDate(service.updated_at)}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-muted-foreground">ID</span>
            <span className="font-mono text-xs">{service.id}</span>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

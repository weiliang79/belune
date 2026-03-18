import { createFileRoute } from "@tanstack/react-router";
import { useDeployments } from "@/lib/hooks/use-deployments";
import { useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "@/lib/hooks/query-keys";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { formatDate, formatDuration } from "@/lib/utils/format";
import { useState, useEffect, useRef } from "react";

export const Route = createFileRoute(
  "/_app/projects/$projectId/services/$serviceId/deployments",
)({
  component: DeploymentsPage,
});

const statusVariant: Record<
  string,
  "default" | "secondary" | "destructive" | "outline"
> = {
  success: "default",
  pending: "secondary",
  building: "secondary",
  deploying: "secondary",
  failed: "destructive",
};

function BuildLogStream({
  projectId,
  serviceId,
  deploymentId,
}: {
  projectId: string;
  serviceId: string;
  deploymentId: string;
}) {
  const [lines, setLines] = useState<string[]>([]);
  const scrollRef = useRef<HTMLPreElement>(null);
  const queryClient = useQueryClient();

  useEffect(() => {
    const url = `/api/projects/${projectId}/services/${serviceId}/deployments/${deploymentId}/build-logs`;
    const source = new EventSource(url, { withCredentials: true });

    source.onmessage = (event) => {
      setLines((prev) => [...prev, event.data]);
    };

    source.addEventListener("done", () => {
      source.close();
      queryClient.invalidateQueries({
        queryKey: queryKeys.deployments.all(projectId, serviceId),
      });
    });

    source.addEventListener("error", (event) => {
      if (event instanceof MessageEvent) {
        setLines((prev) => [...prev, `Error: ${event.data}`]);
      }
      source.close();
    });

    return () => {
      source.close();
    };
  }, [projectId, serviceId, deploymentId, queryClient]);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [lines]);

  return (
    <pre
      ref={scrollRef}
      className="bg-zinc-950 mt-3 max-h-64 overflow-auto rounded p-3 font-mono text-xs text-zinc-200"
    >
      {lines.length === 0 ? (
        <span className="text-zinc-500">Waiting for build output...</span>
      ) : (
        lines.map((line, i) => (
          <div key={i}>{line}</div>
        ))
      )}
    </pre>
  );
}

function DeploymentsPage() {
  const { projectId, serviceId } = Route.useParams();
  const { data: deployments, isLoading } = useDeployments(projectId, serviceId);
  const [expandedId, setExpandedId] = useState<string | null>(null);

  if (isLoading) {
    return <div className="text-muted-foreground">Loading deployments...</div>;
  }

  if (!deployments || deployments.length === 0) {
    return (
      <Card>
        <CardContent className="text-muted-foreground py-12 text-center">
          No deployments yet. Deploy your service to see history here.
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-3">
      {deployments.map((d) => {
        const duration =
          d.finished_at && d.started_at
            ? formatDuration(
                new Date(d.finished_at).getTime() -
                  new Date(d.started_at).getTime(),
              )
            : null;

        const isBuilding = d.status === "building" || d.status === "pending";

        return (
          <Card
            key={d.id}
            className="hover:bg-muted/50 cursor-pointer transition-colors"
            onClick={() => setExpandedId(expandedId === d.id ? null : d.id)}
          >
            <CardContent className="py-3">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <Badge variant={statusVariant[d.status] ?? "outline"}>
                    {d.status}
                  </Badge>
                  <span className="text-muted-foreground text-sm">
                    {d.triggered_by}
                  </span>
                  {d.commit_sha && (
                    <span className="text-muted-foreground font-mono text-xs">
                      {d.commit_sha.slice(0, 7)}
                    </span>
                  )}
                </div>
                <div className="text-muted-foreground flex items-center gap-3 text-xs">
                  {duration && <span>{duration}</span>}
                  <span>{formatDate(d.started_at)}</span>
                </div>
              </div>
              {expandedId === d.id && isBuilding && (
                <BuildLogStream
                  projectId={projectId}
                  serviceId={serviceId}
                  deploymentId={d.id}
                />
              )}
              {expandedId === d.id && !isBuilding && d.build_logs && (
                <pre className="bg-muted mt-3 max-h-64 overflow-auto rounded p-3 font-mono text-xs">
                  {d.build_logs}
                </pre>
              )}
              {expandedId === d.id && d.error_message && (
                <div className="bg-destructive/10 text-destructive mt-3 rounded p-3 text-sm">
                  {d.error_message}
                </div>
              )}
            </CardContent>
          </Card>
        );
      })}
    </div>
  );
}

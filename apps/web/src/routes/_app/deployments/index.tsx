import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
import { useGlobalDeployments } from "@/lib/hooks/use-global-deployments";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { AppBreadcrumb } from "@/lib/components/app-breadcrumb";
import { formatDate, formatDuration } from "@/lib/utils/format";
import type { GlobalDeployment } from "@/lib/types";

export const Route = createFileRoute("/_app/deployments/")({
  component: GlobalDeploymentsPage,
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

const PAGE_SIZE = 50;

function DeploymentRow({ d }: { d: GlobalDeployment }) {
  const duration =
    d.finished_at && d.started_at
      ? formatDuration(
          new Date(d.finished_at).getTime() - new Date(d.started_at).getTime(),
        )
      : null;

  return (
    <div className="flex items-center justify-between py-3">
      <div className="flex min-w-0 flex-1 items-center gap-3">
        <Badge
          variant={statusVariant[d.status] ?? "outline"}
          className="shrink-0"
        >
          {d.status}
        </Badge>
        <div className="min-w-0">
          <div className="flex items-center gap-1.5 text-sm font-medium">
            <span className="text-muted-foreground">{d.project_name}</span>
            <span className="text-muted-foreground">/</span>
            <span>{d.application_name}</span>
          </div>
          <div className="text-muted-foreground flex items-center gap-2 text-xs">
            <span className="capitalize">{d.triggered_by}</span>
            {d.commit_sha && (
              <span className="font-mono">{d.commit_sha.slice(0, 7)}</span>
            )}
          </div>
        </div>
      </div>
      <div className="text-muted-foreground flex shrink-0 items-center gap-3 text-xs">
        {duration && <span>{duration}</span>}
        <span>{formatDate(d.started_at)}</span>
      </div>
    </div>
  );
}

function GlobalDeploymentsPage() {
  const [offset, setOffset] = useState(0);
  const { data: deployments, isLoading } = useGlobalDeployments({
    limit: PAGE_SIZE,
    offset,
  });

  return (
    <div className="space-y-6">
      <AppBreadcrumb items={[{ label: "Deployments" }]} />
      <div>
        <h1 className="text-2xl font-bold">Deployments</h1>
        <p className="text-muted-foreground">
          All deployments across your applications.
        </p>
      </div>

      {isLoading ? (
        <Card>
          <CardContent className="divide-y p-0">
            {[1, 2, 3, 4, 5].map((i) => (
              <div key={i} className="flex items-center justify-between px-4 py-3">
                <div className="flex items-center gap-3">
                  <Skeleton className="h-5 w-16 rounded-full" />
                  <div className="space-y-1">
                    <Skeleton className="h-4 w-40" />
                    <Skeleton className="h-3 w-24" />
                  </div>
                </div>
                <Skeleton className="h-3 w-32" />
              </div>
            ))}
          </CardContent>
        </Card>
      ) : !deployments || deployments.length === 0 ? (
        <Card>
          <CardContent className="text-muted-foreground py-12 text-center">
            {offset > 0
              ? "No more deployments."
              : "No deployments yet."}
          </CardContent>
        </Card>
      ) : (
        <Card>
          <CardContent className="divide-y p-0">
            {deployments.map((d) => (
              <Link
                key={d.id}
                to="/projects/$projectId/applications/$applicationId/deployments"
                params={{ projectId: d.project_id, applicationId: d.application_id }}
                className="hover:bg-muted/50 block px-4 transition-colors"
              >
                <DeploymentRow d={d} />
              </Link>
            ))}
          </CardContent>
        </Card>
      )}

      {/* Pagination */}
      <div className="flex items-center justify-between">
        <Button
          variant="outline"
          size="sm"
          disabled={offset === 0}
          onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
        >
          Previous
        </Button>
        <span className="text-muted-foreground text-sm">
          Showing {offset + 1}–{offset + (deployments?.length ?? 0)}
        </span>
        <Button
          variant="outline"
          size="sm"
          disabled={!deployments || deployments.length < PAGE_SIZE}
          onClick={() => setOffset(offset + PAGE_SIZE)}
        >
          Next
        </Button>
      </div>
    </div>
  );
}

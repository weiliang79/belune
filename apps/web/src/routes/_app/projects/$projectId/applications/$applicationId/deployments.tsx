import { createFileRoute } from "@tanstack/react-router";
import { useDeployments, useRollbackDeployment } from "@/lib/hooks/use-deployments";
import { useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "@/lib/hooks/query-keys";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import type { Deployment } from "@/lib/types";
import { formatDate, formatDuration } from "@/lib/utils/format";
import { useState, useEffect, useRef, useCallback } from "react";
import { useChannel } from "@/lib/hooks/use-websocket";

export const Route = createFileRoute(
  "/_app/projects/$projectId/applications/$applicationId/deployments",
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

function useBuildLogStream(
  projectId: string,
  applicationId: string,
  deploymentId: string,
  isBuilding: boolean,
) {
  const [lines, setLines] = useState<string[]>([]);
  const queryClient = useQueryClient();

  const handleMessage = useCallback(
    (event: string, data: unknown) => {
      if (event === "done") {
        queryClient.invalidateQueries({
          queryKey: queryKeys.deployments.all(projectId, applicationId),
        });
        return;
      }
      if (typeof data === "string") {
        setLines((prev) => [...prev, data]);
      }
    },
    [queryClient, projectId, applicationId],
  );

  const channel = isBuilding ? `build-logs:${deploymentId}` : null;
  useChannel(channel, handleMessage);

  return lines;
}

function BuildLogViewer({ lines }: { lines: string[] }) {
  const scrollRef = useRef<HTMLPreElement>(null);

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

function RollbackButton({
  deployment,
  projectId,
  applicationId,
}: {
  deployment: Deployment;
  projectId: string;
  applicationId: string;
}) {
  const { mutate: rollback, isPending } = useRollbackDeployment(projectId, applicationId);

  return (
    <AlertDialog>
      <AlertDialogTrigger
        render={
          <Button
            size="sm"
            variant="outline"
            disabled={isPending}
            onClick={(e) => e.stopPropagation()}
          />
        }
      >
        {isPending ? "Rolling back..." : "Rollback"}
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Rollback to this deployment?</AlertDialogTitle>
          <AlertDialogDescription>
            This will redeploy the image from{" "}
            <strong>{formatDate(deployment.started_at)}</strong>
            {deployment.commit_sha && (
              <> (commit <code>{deployment.commit_sha.slice(0, 7)}</code>)</>
            )}
            . A new deployment will be created with the stored image tag.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel onClick={(e) => e.stopPropagation()}>
            Cancel
          </AlertDialogCancel>
          <AlertDialogAction
            onClick={(e) => {
              e.stopPropagation();
              rollback(deployment.id);
            }}
          >
            Rollback
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

function DeploymentCard({
  deployment: d,
  projectId,
  applicationId,
}: {
  deployment: Deployment;
  projectId: string;
  applicationId: string;
}) {
  const [expanded, setExpanded] = useState(false);
  const isBuilding = d.status === "building" || d.status === "pending";
  const lines = useBuildLogStream(projectId, applicationId, d.id, isBuilding);
  const canRollback = d.status === "success" && !!d.image_tag;

  // Overall duration (started → finished)
  const totalDuration =
    d.finished_at && d.started_at
      ? formatDuration(new Date(d.finished_at).getTime() - new Date(d.started_at).getTime())
      : null;

  // Build phase: build_started_at → build_ended_at
  const buildDuration =
    d.build_started_at && d.build_ended_at
      ? formatDuration(new Date(d.build_ended_at).getTime() - new Date(d.build_started_at).getTime())
      : null;

  // Deploy phase: deploy_started_at → finished_at
  const deployDuration =
    d.deploy_started_at && d.finished_at
      ? formatDuration(new Date(d.finished_at).getTime() - new Date(d.deploy_started_at).getTime())
      : null;

  const hasSplit = buildDuration !== null && deployDuration !== null;

  return (
    <Card
      className="hover:bg-muted/50 cursor-pointer transition-colors"
      onClick={() => setExpanded(!expanded)}
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
          <div className="flex items-center gap-3">
            {canRollback && (
              <RollbackButton
                deployment={d}
                projectId={projectId}
                applicationId={applicationId}
              />
            )}
            <div className="text-muted-foreground flex items-center gap-3 text-xs">
              {hasSplit ? (
                <span title={`Build: ${buildDuration} · Deploy: ${deployDuration}`}>
                  {buildDuration} + {deployDuration}
                </span>
              ) : totalDuration ? (
                <span>{totalDuration}</span>
              ) : null}
              <span>{formatDate(d.started_at)}</span>
            </div>
          </div>
        </div>
        {expanded && isBuilding && <BuildLogViewer lines={lines} />}
        {expanded && !isBuilding && d.build_logs && (
          <pre className="bg-muted mt-3 max-h-64 overflow-auto rounded p-3 font-mono text-xs">
            {d.build_logs}
          </pre>
        )}
        {expanded && d.error_message && (
          <div className="bg-destructive/10 text-destructive mt-3 rounded p-3 text-sm">
            {d.error_message}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function DeploymentsPage() {
  const { projectId, applicationId } = Route.useParams();
  const { data: deployments, isLoading, error } = useDeployments(projectId, applicationId);

  if (isLoading) {
    return (
      <div className="space-y-3">
        {[1, 2, 3].map((i) => (
          <Card key={i}>
            <CardContent className="py-3">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <Skeleton className="h-5 w-16 rounded-full" />
                  <Skeleton className="h-4 w-20" />
                </div>
                <Skeleton className="h-4 w-32" />
              </div>
            </CardContent>
          </Card>
        ))}
      </div>
    );
  }

  if (error) {
    return (
      <Card>
        <CardContent className="text-destructive py-12 text-center">
          Failed to load deployments: {error.message}
        </CardContent>
      </Card>
    );
  }

  if (!deployments || deployments.length === 0) {
    return (
      <Card>
        <CardContent className="text-muted-foreground py-12 text-center">
          No deployments yet. Deploy your application to see history here.
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-3">
      {deployments.map((d) => (
        <DeploymentCard
          key={d.id}
          deployment={d}
          projectId={projectId}
          applicationId={applicationId}
        />
      ))}
    </div>
  );
}

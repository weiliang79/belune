import {
  createFileRoute,
  Link,
  Outlet,
  useRouterState,
} from "@tanstack/react-router";
import {
  useApplication,
  useDeployApplication,
  useStopApplication,
  useStartApplication,
  useRestartApplication,
} from "@/lib/hooks/use-applications";
import { useProject } from "@/lib/hooks/use-projects";
import { useAppMetricsStream } from "@/lib/hooks/use-metrics";
import { useChannel } from "@/lib/hooks/use-websocket";
import { useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "@/lib/hooks/query-keys";
import { useCallback } from "react";
import { toast } from "sonner";
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
import { cn } from "@/lib/utils";
import { Skeleton } from "@/components/ui/skeleton";
import { AppBreadcrumb } from "@/lib/components/app-breadcrumb";
import { StatusBadge } from "@/lib/components/status-badge";
import { AppMetricsContext } from "@/lib/contexts/app-metrics-context";
import { RouteError } from "@/lib/components/route-error";

export const Route = createFileRoute(
  "/_app/projects/$projectId/applications/$applicationId",
)({
  component: ApplicationLayout,
  errorComponent: RouteError,
});

function ApplicationLayout() {
  const { projectId, applicationId } = Route.useParams();
  const { data: application, isLoading } = useApplication(projectId, applicationId);
  const { data: project } = useProject(projectId);
  const appMetrics = useAppMetricsStream(projectId, applicationId, true);
  const qc = useQueryClient();

  // Subscribe to real-time container status changes
  const handleContainerStatus = useCallback(
    () => {
      qc.invalidateQueries({
        queryKey: queryKeys.applications.detail(projectId, applicationId),
      });
    },
    [qc, projectId, applicationId],
  );
  useChannel(`container-status:${applicationId}`, handleContainerStatus);

  const deploy = useDeployApplication(projectId, applicationId);
  const stop = useStopApplication(projectId, applicationId);
  const start = useStartApplication(projectId, applicationId);
  const restart = useRestartApplication(projectId, applicationId);
  const routerState = useRouterState();
  const currentPath = routerState.location.pathname;

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-4 w-64" />
        <div className="flex items-center gap-3">
          <Skeleton className="h-8 w-48" />
          <Skeleton className="h-6 w-16 rounded-full" />
        </div>
        <div className="flex gap-1 border-b">
          {[1, 2, 3, 4, 5].map((i) => (
            <Skeleton key={i} className="h-9 w-20" />
          ))}
        </div>
      </div>
    );
  }

  if (!application) {
    return <div className="text-destructive">Application not found.</div>;
  }

  const basePath = `/projects/${projectId}/applications/${applicationId}`;
  const tabs = [
    { to: basePath, label: "Overview", exact: true },
    { to: `${basePath}/deployments`, label: "Deployments" },
    { to: `${basePath}/metrics`, label: "Metrics" },
    { to: `${basePath}/logs`, label: "Logs" },
    { to: `${basePath}/env`, label: "Env Vars" },
    { to: `${basePath}/domains`, label: "Domains" },
    { to: `${basePath}/previews`, label: "Previews" },
    { to: `${basePath}/terminal`, label: "Terminal" },
    { to: `${basePath}/settings`, label: "Settings" },
  ];

  return (
    <div className="space-y-6">
      <AppBreadcrumb
        items={[
          { label: "Projects", to: "/projects" },
          {
            label: project?.name ?? "Project",
            to: `/projects/${projectId}`,
          },
          { label: application.name },
        ]}
      />

      <div className="flex items-center justify-between">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-bold">{application.name}</h1>
            <StatusBadge status={application.status} />
          </div>
          <p className="text-muted-foreground text-sm">{application.slug}</p>
        </div>
        <div className="flex gap-2">
          <Button
            size="sm"
            onClick={() => {
              toast.promise(deploy.mutateAsync(), {
                loading: "Deploying...",
                success: "Deployment started",
                error: (err) => err.message,
              });
            }}
            disabled={deploy.isPending}
          >
            {deploy.isPending ? "Deploying..." : "Deploy"}
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() => {
              toast.promise(restart.mutateAsync(), {
                loading: "Restarting...",
                success: "Application restarted",
                error: (err) => err.message,
              });
            }}
            disabled={restart.isPending || application.status !== "running"}
          >
            Restart
          </Button>
          {application.status === "running" ? (
            <AlertDialog>
              <AlertDialogTrigger
                render={
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={stop.isPending}
                  />
                }
              >
                {stop.isPending ? "Stopping..." : "Stop"}
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>
                    Stop {application.name}?
                  </AlertDialogTitle>
                  <AlertDialogDescription>
                    This will stop the running container. You can start it
                    again later.
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>Cancel</AlertDialogCancel>
                  <AlertDialogAction
                    onClick={() => {
                      toast.promise(stop.mutateAsync(), {
                        loading: "Stopping...",
                        success: "Application stopped",
                        error: (err) => err.message,
                      });
                    }}
                  >
                    Stop
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          ) : (
            <Button
              size="sm"
              variant="outline"
              onClick={() => {
                toast.promise(start.mutateAsync(), {
                  loading: "Starting...",
                  success: "Application started",
                  error: (err) => err.message,
                });
              }}
              disabled={start.isPending || application.status === "deploying" || application.status === "building"}
            >
              {start.isPending ? "Starting..." : "Start"}
            </Button>
          )}
        </div>
      </div>

      <nav aria-label="Application navigation" className="flex gap-1 overflow-x-auto border-b">
        {tabs.map((tab) => {
          const isActive = tab.exact
            ? currentPath === tab.to
            : currentPath.startsWith(tab.to);
          return (
            <Link
              key={tab.to}
              to={tab.to}
              aria-current={isActive ? "page" : undefined}
              className={cn(
                "border-b-2 px-3 py-2 text-sm font-medium whitespace-nowrap transition-colors",
                isActive
                  ? "border-primary text-foreground"
                  : "text-muted-foreground hover:text-foreground border-transparent",
              )}
            >
              {tab.label}
            </Link>
          );
        })}
      </nav>

      <AppMetricsContext value={appMetrics}>
        <Outlet />
      </AppMetricsContext>
    </div>
  );
}

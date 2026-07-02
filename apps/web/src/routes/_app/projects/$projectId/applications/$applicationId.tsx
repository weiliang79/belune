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
import { useDeployments } from "@/lib/hooks/use-deployments";
import { useDomains } from "@/lib/hooks/use-domains";
import { useProjectMetrics } from "@/lib/hooks/use-project-metrics";
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
import { AppWindowIcon, ExternalLinkIcon } from "lucide-react";
import { cn } from "@/lib/utils";
import { formatUptime } from "@/lib/utils/format";
import { Skeleton } from "@/components/ui/skeleton";
import { useBreadcrumbLabel } from "@/lib/hooks/use-breadcrumb";
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
  const { data: application, isLoading } = useApplication(
    projectId,
    applicationId,
  );
  const { data: project } = useProject(projectId);
  const { data: deployments } = useDeployments(projectId, applicationId);
  const { data: domains } = useDomains(projectId, applicationId);
  const { data: projectMetrics } = useProjectMetrics(projectId);
  const appMetrics = useAppMetricsStream(projectId, applicationId, true);
  const qc = useQueryClient();

  // Subscribe to real-time container status changes
  const handleContainerStatus = useCallback(() => {
    qc.invalidateQueries({
      queryKey: queryKeys.applications.detail(projectId, applicationId),
    });
  }, [qc, projectId, applicationId]);
  useChannel(`container-status:${applicationId}`, handleContainerStatus);

  const deploy = useDeployApplication(projectId, applicationId);
  const stop = useStopApplication(projectId, applicationId);
  const start = useStartApplication(projectId, applicationId);
  const restart = useRestartApplication(projectId, applicationId);
  const routerState = useRouterState();
  const currentPath = routerState.location.pathname;

  useBreadcrumbLabel(projectId, project?.name);
  useBreadcrumbLabel(applicationId, application?.name);

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
  const primaryDomain = domains?.[0]?.hostname;
  const uptimeSeconds = projectMetrics?.[applicationId]?.uptime_seconds;
  const tabs: { to: string; label: string; exact?: boolean; count?: number }[] =
    [
      { to: basePath, label: "Overview", exact: true },
      { to: `${basePath}/env`, label: "Env Vars" },
      { to: `${basePath}/domains`, label: "Domains", count: domains?.length },
      {
        to: `${basePath}/deployments`,
        label: "Deployments",
        count: deployments?.length,
      },
      { to: `${basePath}/previews`, label: "Previews" },
      { to: `${basePath}/logs`, label: "Logs" },
      { to: `${basePath}/metrics`, label: "Metrics" },
      { to: `${basePath}/terminal`, label: "Terminal" },
      { to: `${basePath}/settings`, label: "Settings" },
    ];

  return (
    <div className="space-y-6">

      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex items-start gap-3">
          <div className="bg-elev text-text-muted grid size-11 shrink-0 place-items-center rounded-xl">
            <AppWindowIcon aria-hidden="true" className="size-5" />
          </div>
          <div className="min-w-0">
            <div className="flex items-center gap-2.5">
              <h1 className="truncate text-2xl font-semibold tracking-tight">
                {application.name}
              </h1>
              <StatusBadge status={application.status} />
              {application.status === "running" && uptimeSeconds ? (
                <span className="text-text-faint text-sm">
                  Up {formatUptime(uptimeSeconds)}
                </span>
              ) : null}
            </div>
            <div className="text-text-faint flex flex-wrap items-center gap-x-2 gap-y-0.5 text-sm">
              <span className="truncate font-mono">{application.slug}</span>
              {primaryDomain && (
                <a
                  href={`https://${primaryDomain}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-text-muted hover:text-foreground inline-flex items-center gap-1 font-mono"
                >
                  <ExternalLinkIcon aria-hidden="true" className="size-3" />
                  {primaryDomain}
                </a>
              )}
            </div>
          </div>
        </div>
        <div className="flex gap-2">
          {primaryDomain && (
            <Button
              size="sm"
              variant="outline"
              render={
                <a
                  href={`https://${primaryDomain}`}
                  target="_blank"
                  rel="noopener noreferrer"
                />
              }
            >
              <ExternalLinkIcon aria-hidden="true" className="size-4" />
              Open URL
            </Button>
          )}
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
                  <AlertDialogTitle>Stop {application.name}?</AlertDialogTitle>
                  <AlertDialogDescription>
                    This will stop the running container. You can start it again
                    later.
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
              disabled={
                start.isPending ||
                application.status === "deploying" ||
                application.status === "building"
              }
            >
              {start.isPending ? "Starting..." : "Start"}
            </Button>
          )}
        </div>
      </div>

      <nav
        aria-label="Application navigation"
        className="flex gap-1 overflow-x-auto border-b"
      >
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
                "flex items-center gap-1.5 border-b-2 px-3 py-2 text-sm font-medium whitespace-nowrap transition-colors",
                isActive
                  ? "border-primary text-foreground"
                  : "text-muted-foreground hover:text-foreground border-transparent",
              )}
            >
              {tab.label}
              {tab.count != null && tab.count > 0 && (
                <span className="bg-elev text-text-muted rounded-full px-1.5 py-0.5 font-mono text-[10px] leading-none">
                  {tab.count}
                </span>
              )}
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

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
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { AppBreadcrumb } from "@/lib/components/app-breadcrumb";
import { StatusBadge } from "@/lib/components/status-badge";

export const Route = createFileRoute(
  "/_app/projects/$projectId/applications/$applicationId",
)({
  component: ApplicationLayout,
});

function ApplicationLayout() {
  const { projectId, applicationId } = Route.useParams();
  const { data: application, isLoading } = useApplication(projectId, applicationId);
  const { data: project } = useProject(projectId);
  const deploy = useDeployApplication(projectId, applicationId);
  const stop = useStopApplication(projectId, applicationId);
  const start = useStartApplication(projectId, applicationId);
  const restart = useRestartApplication(projectId, applicationId);
  const routerState = useRouterState();
  const currentPath = routerState.location.pathname;

  if (isLoading) {
    return <div className="text-muted-foreground">Loading application...</div>;
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
            onClick={() => deploy.mutate()}
            disabled={deploy.isPending}
          >
            {deploy.isPending ? "Deploying..." : "Deploy"}
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() => restart.mutate()}
            disabled={restart.isPending || application.status !== "running"}
          >
            Restart
          </Button>
          {application.status === "running" ? (
            <Button
              size="sm"
              variant="outline"
              onClick={() => stop.mutate()}
              disabled={stop.isPending}
            >
              {stop.isPending ? "Stopping..." : "Stop"}
            </Button>
          ) : (
            <Button
              size="sm"
              variant="outline"
              onClick={() => start.mutate()}
              disabled={start.isPending || application.status === "deploying" || application.status === "building"}
            >
              {start.isPending ? "Starting..." : "Start"}
            </Button>
          )}
        </div>
      </div>

      <nav className="flex gap-1 overflow-x-auto border-b">
        {tabs.map((tab) => {
          const isActive = tab.exact
            ? currentPath === tab.to
            : currentPath.startsWith(tab.to);
          return (
            <Link
              key={tab.to}
              to={tab.to}
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

      <Outlet />
    </div>
  );
}

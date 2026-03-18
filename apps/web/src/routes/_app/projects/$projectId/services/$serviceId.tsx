import {
  createFileRoute,
  Link,
  Outlet,
  useRouterState,
} from "@tanstack/react-router";
import {
  useService,
  useDeployService,
  useStopService,
  useStartService,
  useRestartService,
} from "@/lib/hooks/use-services";
import { useProject } from "@/lib/hooks/use-projects";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { AppBreadcrumb } from "@/lib/components/app-breadcrumb";
import { StatusBadge } from "@/lib/components/status-badge";

export const Route = createFileRoute(
  "/_app/projects/$projectId/services/$serviceId",
)({
  component: ServiceLayout,
});

function ServiceLayout() {
  const { projectId, serviceId } = Route.useParams();
  const { data: service, isLoading } = useService(projectId, serviceId);
  const { data: project } = useProject(projectId);
  const deploy = useDeployService(projectId, serviceId);
  const stop = useStopService(projectId, serviceId);
  const start = useStartService(projectId, serviceId);
  const restart = useRestartService(projectId, serviceId);
  const routerState = useRouterState();
  const currentPath = routerState.location.pathname;

  if (isLoading) {
    return <div className="text-muted-foreground">Loading application...</div>;
  }

  if (!service) {
    return <div className="text-destructive">Application not found.</div>;
  }

  const basePath = `/projects/${projectId}/services/${serviceId}`;
  const tabs = [
    { to: basePath, label: "Overview", exact: true },
    { to: `${basePath}/deployments`, label: "Deployments" },
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
          { label: service.name },
        ]}
      />

      <div className="flex items-center justify-between">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-bold">{service.name}</h1>
            <StatusBadge status={service.status} />
          </div>
          <p className="text-muted-foreground text-sm">{service.slug}</p>
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
            disabled={restart.isPending || service.status !== "running"}
          >
            Restart
          </Button>
          {service.status === "running" ? (
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
              disabled={start.isPending || service.status === "deploying" || service.status === "building"}
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
              to={tab.to as any}
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

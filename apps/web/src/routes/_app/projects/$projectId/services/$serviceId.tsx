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
  useRestartService,
} from "@/lib/hooks/use-services";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export const Route = createFileRoute(
  "/_app/projects/$projectId/services/$serviceId",
)({
  component: ServiceLayout,
});

function ServiceLayout() {
  const { projectId, serviceId } = Route.useParams();
  const { data: service, isLoading } = useService(projectId, serviceId);
  const deploy = useDeployService(projectId, serviceId);
  const stop = useStopService(projectId, serviceId);
  const restart = useRestartService(projectId, serviceId);
  const routerState = useRouterState();
  const currentPath = routerState.location.pathname;

  if (isLoading) {
    return <div className="text-muted-foreground">Loading service...</div>;
  }

  if (!service) {
    return <div className="text-destructive">Service not found.</div>;
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

  const statusColor =
    {
      running: "bg-green-500",
      stopped: "bg-gray-500",
      deploying: "bg-yellow-500",
      failed: "bg-red-500",
    }[service.status] ?? "bg-gray-500";

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <h1 className="text-2xl font-bold">{service.name}</h1>
          <Badge variant="outline" className="gap-1.5">
            <span className={cn("size-2 rounded-full", statusColor)} />
            {service.status}
          </Badge>
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
            disabled={restart.isPending}
          >
            Restart
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() => stop.mutate()}
            disabled={stop.isPending}
          >
            Stop
          </Button>
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

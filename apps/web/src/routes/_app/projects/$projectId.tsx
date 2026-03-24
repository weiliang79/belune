import {
  createFileRoute,
  Link,
  Outlet,
  useRouterState,
} from "@tanstack/react-router";
import { useProject } from "@/lib/hooks/use-projects";
import { cn } from "@/lib/utils";
import { Skeleton } from "@/components/ui/skeleton";
import { AppBreadcrumb } from "@/lib/components/app-breadcrumb";

export const Route = createFileRoute("/_app/projects/$projectId")({
  component: ProjectLayout,
});

function ProjectLayout() {
  const { projectId } = Route.useParams();
  const { data: project, isLoading } = useProject(projectId);
  const routerState = useRouterState();
  const currentPath = routerState.location.pathname;

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-4 w-32" />
        <div className="flex gap-1 border-b">
          {[1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-9 w-24" />
          ))}
        </div>
      </div>
    );
  }

  if (!project) {
    return <div className="text-destructive">Project not found.</div>;
  }

  // Hide project chrome when viewing a service or database detail page
  const isChildDetail =
    currentPath.includes("/applications/") || currentPath.includes("/databases/");

  if (isChildDetail) {
    return <Outlet />;
  }

  const tabs = [
    { to: `/projects/${projectId}`, label: "Applications" },
    { to: `/projects/${projectId}/env`, label: "Environment" },
    { to: `/projects/${projectId}/settings`, label: "Settings" },
  ];

  return (
    <div className="space-y-6">
      <AppBreadcrumb
        items={[
          { label: "Projects", to: "/projects" },
          { label: project.name },
        ]}
      />

      <div>
        <h1 className="text-2xl font-bold">{project.name}</h1>
        <p className="text-muted-foreground text-sm">{project.slug}</p>
      </div>

      <nav className="flex gap-1 border-b">
        {tabs.map((tab) => {
          const isActive =
            tab.to === `/projects/${projectId}`
              ? !currentPath.startsWith(`/projects/${projectId}/settings`) &&
                !currentPath.startsWith(`/projects/${projectId}/env`)
              : currentPath.startsWith(tab.to);
          return (
            <Link
              key={tab.to}
              to={tab.to}
              className={cn(
                "border-b-2 px-4 py-2 text-sm font-medium transition-colors",
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

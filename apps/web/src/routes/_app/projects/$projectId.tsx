import {
  createFileRoute,
  Link,
  Outlet,
  useRouterState,
} from "@tanstack/react-router";
import { useProject } from "@/lib/hooks/use-projects";
import { cn } from "@/lib/utils";

export const Route = createFileRoute("/_app/projects/$projectId")({
  component: ProjectLayout,
});

function ProjectLayout() {
  const { projectId } = Route.useParams();
  const { data: project, isLoading } = useProject(projectId);
  const routerState = useRouterState();
  const currentPath = routerState.location.pathname;

  if (isLoading) {
    return <div className="text-muted-foreground">Loading project...</div>;
  }

  if (!project) {
    return <div className="text-destructive">Project not found.</div>;
  }

  const tabs = [
    { to: `/projects/${projectId}`, label: "Overview", exact: true },
    { to: `/projects/${projectId}/services`, label: "Services" },
    { to: `/projects/${projectId}/databases`, label: "Databases" },
  ];

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">{project.name}</h1>
        <p className="text-muted-foreground text-sm">{project.slug}</p>
      </div>

      <nav className="flex gap-1 border-b">
        {tabs.map((tab) => {
          const isActive = tab.exact
            ? currentPath === tab.to
            : currentPath.startsWith(tab.to);
          return (
            <Link
              key={tab.to}
              to={tab.to as any}
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

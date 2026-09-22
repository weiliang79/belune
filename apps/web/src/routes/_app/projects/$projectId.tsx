import {
  createFileRoute,
  Outlet,
  useRouterState,
} from "@tanstack/react-router";
import {
  LayoutDashboard,
  Archive,
  SlidersHorizontal,
  Settings,
} from "lucide-react";
import { useProject } from "@/lib/hooks/use-projects";
import { useApplications } from "@/lib/hooks/use-applications";
import { useDatabases } from "@/lib/hooks/use-databases";
import { PageTabLinks, type PageTabLink } from "@/components/ui/page-tabs";
import { Skeleton } from "@/components/ui/skeleton";
import { useBreadcrumbLabel } from "@/lib/hooks/use-breadcrumb";
import { ProjectHeader } from "@/components/projects/project-header";
import { RouteError } from "@/lib/components/route-error";
import { RouteSkeleton } from "@/lib/components/route-skeleton";
import { projectLayoutMode } from "@/lib/utils/project-layout-mode";

export const Route = createFileRoute("/_app/projects/$projectId")({
  component: ProjectLayout,
  errorComponent: RouteError,
});

function ProjectLayout() {
  const { projectId } = Route.useParams();
  const { data: project, isLoading } = useProject(projectId);
  const { data: applications } = useApplications(projectId);
  const { data: databases } = useDatabases(projectId);
  const routerState = useRouterState();
  const currentPath = routerState.location.pathname;

  useBreadcrumbLabel(projectId, project?.name);

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

  // Hide project chrome when viewing a service or database detail page. Read
  // from the live (optimistic) pathname so this reacts the instant a
  // navigation starts, in step with the Topbar breadcrumb above it — reading
  // the settled path instead left this header showing (mismatched against an
  // already-updated breadcrumb) for the entire time the child route took to
  // load. This only guards the forward direction (arriving at a child-detail
  // page); leaving one, the incoming project header is already correct and
  // any stale content beneath it is the same brief, everywhere-normal
  // pending-route flash every other transition in the app has. See
  // projectLayoutMode's own doc comment for what each outcome means — it's
  // extracted so this predicate is unit-tested rather than only
  // click-through-verified.
  const resolvedPath = routerState.resolvedLocation?.pathname ?? currentPath;
  const mode = projectLayoutMode(currentPath, resolvedPath);

  if (mode !== "chrome") {
    return mode === "outlet" ? <Outlet /> : <RouteSkeleton />;
  }

  const tabs: PageTabLink[] = [
    {
      to: `/projects/${projectId}`,
      label: "Overview",
      exact: true,
      icon: LayoutDashboard,
    },
    { to: `/projects/${projectId}/backups`, label: "Backups", icon: Archive },
    {
      to: `/projects/${projectId}/env`,
      label: "Env Vars",
      icon: SlidersHorizontal,
    },
    {
      to: `/projects/${projectId}/settings`,
      label: "Settings",
      icon: Settings,
    },
  ];

  return (
    <div className="space-y-6">
      <ProjectHeader
        project={project}
        applications={applications}
        databases={databases}
      />

      <PageTabLinks ariaLabel="Project navigation" items={tabs} />

      <Outlet />
    </div>
  );
}

import { createFileRoute, Link } from "@tanstack/react-router";
import { useProjects } from "@/lib/hooks/use-projects";
import { useServices } from "@/lib/hooks/use-services";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { buttonVariants } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { formatDate } from "@/lib/utils/format";
import { AppBreadcrumb } from "@/lib/components/app-breadcrumb";
import type { Project } from "@/lib/types";

export const Route = createFileRoute("/_app/projects/")({
  component: ProjectsPage,
});

function ProjectServiceCount({ projectId }: { projectId: string }) {
  const { data: services } = useServices(projectId);
  if (!services) return null;
  const running = services.filter((s) => s.status === "running").length;
  return (
    <p className="text-muted-foreground text-xs">
      {running} / {services.length} applications running
    </p>
  );
}

function ProjectsPage() {
  const { data: projects, isLoading } = useProjects();

  if (isLoading) {
    return <div className="text-muted-foreground">Loading projects...</div>;
  }

  return (
    <div className="space-y-6">
      <AppBreadcrumb items={[{ label: "Projects" }]} />
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Projects</h1>
          <p className="text-muted-foreground">
            Manage your projects and services.
          </p>
        </div>
        <Link to="/projects/new" className={buttonVariants()}>
          New Project
        </Link>
      </div>

      {!projects || projects.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <p className="text-muted-foreground mb-4">No projects yet.</p>
            <Link to="/projects/new" className={buttonVariants()}>
              Create your first project
            </Link>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {projects.map((project) => (
            <Link
              key={project.id}
              to="/projects/$projectId"
              params={{ projectId: project.id }}
            >
              <Card className="hover:bg-muted/50 cursor-pointer transition-colors">
                <CardHeader className="pb-2">
                  <div className="flex items-start justify-between">
                    <CardTitle className="text-base">{project.name}</CardTitle>
                    <Badge variant="secondary">{project.slug}</Badge>
                  </div>
                </CardHeader>
                <CardContent>
                  <ProjectServiceCount projectId={project.id} />
                  <p className="text-muted-foreground text-xs">
                    Created {formatDate(project.created_at)}
                  </p>
                </CardContent>
              </Card>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}

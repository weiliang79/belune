import { createFileRoute, Link } from "@tanstack/react-router";
import { useProjects } from "@/lib/hooks/use-projects";
import { useApplications } from "@/lib/hooks/use-applications";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { buttonVariants } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { formatDate } from "@/lib/utils/format";
import { AppBreadcrumb } from "@/lib/components/app-breadcrumb";
import { FolderOpenIcon } from "lucide-react";
import { useDatabases } from "@/lib/hooks/use-databases";
import { cn } from "@/lib/utils";
import { Separator } from "@/components/ui/separator";

export const Route = createFileRoute("/_app/projects/")({
  component: ProjectsPage,
});

function ProjectApplicationCount({ projectId }: { projectId: string }) {
  const { data: applications } = useApplications(projectId);
  const { data: databases } = useDatabases(projectId);
  if (!applications || !databases) return null;
  const total = applications.length + databases.length;
  const running =
    applications.filter((s) => s.status === "running").length +
    databases.filter((s) => s.status === "running").length;

  function getStatusColor() {
    if (total === 0) return "bg-gray-500";
    if (running === total) return "bg-green-500";
    if (running === 0) return "bg-red-500";
    return "bg-yellow-500";
  }
  return (
    <Badge variant="outline">
      <span className={cn("size-2 rounded-full", getStatusColor())} />
      {running} / {applications.length + databases.length} Apps
    </Badge>
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
            Manage your projects and applications.
          </p>
        </div>
        <Link to="/projects/new" className={buttonVariants()}>
          New Project
        </Link>
      </div>

      <Separator />

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
                    <div className="flex items-start gap-2">
                      <FolderOpenIcon className="text-muted-foreground size-4" />
                      <div className="flex flex-col">
                        <CardTitle className="text-base leading-none">
                          {project.name}
                        </CardTitle>
                        <CardDescription className="text-sm">
                          {project.slug}
                        </CardDescription>
                      </div>
                    </div>
                    <ProjectApplicationCount projectId={project.id} />
                  </div>
                </CardHeader>
                <CardContent>
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

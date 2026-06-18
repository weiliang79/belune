import { useState } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import {
  AppWindowIcon,
  ChevronRightIcon,
  DatabaseIcon,
  LayersIcon,
} from "lucide-react";
import { useProject } from "@/lib/hooks/use-projects";
import { useApplications } from "@/lib/hooks/use-applications";
import { useDatabases } from "@/lib/hooks/use-databases";
import { Badge } from "@/components/ui/badge";
import { StatusPill } from "@/components/ui/status-pill";
import { Skeleton } from "@/components/ui/skeleton";
import { ApplicationFormDialog } from "@/components/applications/application-form-dialog";
import { DatabaseFormDialog } from "@/components/databases/database-form-dialog";
import { ProjectHeader } from "@/components/projects/project-header";

export const Route = createFileRoute("/_app/projects/$projectId/")({
  component: ProjectOverview,
});

function ServiceRow({
  to,
  params,
  icon,
  name,
  slug,
  status,
  meta,
}: {
  to: string;
  params: Record<string, string>;
  icon: React.ReactNode;
  name: string;
  slug: string;
  status: string;
  meta: React.ReactNode;
}) {
  return (
    <Link
      to={to}
      params={params}
      className="hover:bg-card-hover group flex items-center gap-3 px-4 py-3 transition-colors"
    >
      <div className="bg-elev text-text-muted grid size-9 shrink-0 place-items-center rounded-lg">
        {icon}
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="group-hover:text-primary truncate text-sm font-medium transition-colors">
            {name}
          </span>
          <span className="text-text-faint hidden truncate font-mono text-xs sm:inline">
            {slug}
          </span>
        </div>
        <div className="mt-1 flex items-center gap-2">{meta}</div>
      </div>
      <StatusPill status={status} />
      <ChevronRightIcon
        aria-hidden="true"
        className="text-text-faint size-4 shrink-0"
      />
    </Link>
  );
}

function ProjectOverview() {
  const { projectId } = Route.useParams();
  const { data: project } = useProject(projectId);
  const { data: applications, isLoading: applicationsLoading } =
    useApplications(projectId);
  const { data: databases, isLoading: databasesLoading } =
    useDatabases(projectId);

  const [appDialogOpen, setAppDialogOpen] = useState(false);
  const [dbDialogOpen, setDbDialogOpen] = useState(false);

  const loading = applicationsLoading || databasesLoading;
  const hasApplications = !!applications && applications.length > 0;
  const hasDatabases = !!databases && databases.length > 0;
  const isEmpty = !hasApplications && !hasDatabases;

  return (
    <div className="space-y-8">
      <ProjectHeader
        project={project}
        onAddApplication={() => setAppDialogOpen(true)}
        onAddDatabase={() => setDbDialogOpen(true)}
      />

      <ApplicationFormDialog
        projectId={projectId}
        open={appDialogOpen}
        onOpenChange={setAppDialogOpen}
      />
      <DatabaseFormDialog
        projectId={projectId}
        open={dbDialogOpen}
        onOpenChange={setDbDialogOpen}
      />

      {loading ? (
        <div className="divide-border overflow-hidden rounded-xl border">
          {[1, 2, 3].map((i) => (
            <div key={i} className="flex items-center gap-3 px-4 py-3">
              <Skeleton className="size-9 rounded-lg" />
              <div className="flex-1 space-y-1.5">
                <Skeleton className="h-4 w-40" />
                <Skeleton className="h-3 w-24" />
              </div>
              <Skeleton className="h-5 w-16 rounded-full" />
            </div>
          ))}
        </div>
      ) : isEmpty ? (
        <div className="rounded-xl border border-dashed py-16 text-center">
          <LayersIcon
            aria-hidden="true"
            className="text-text-faint mx-auto size-8"
          />
          <p className="text-muted-foreground mt-3 text-sm">
            No applications or databases yet.
          </p>
          <p className="text-text-faint mt-1 text-xs">
            Use{" "}
            <span className="text-foreground font-medium">New Application</span>{" "}
            or <span className="text-foreground font-medium">New Database</span>{" "}
            above to get started.
          </p>
        </div>
      ) : (
        <div className="divide-border divide-y overflow-hidden rounded-xl border">
          {applications?.map((application) => (
            <ServiceRow
              key={application.id}
              to="/projects/$projectId/applications/$applicationId"
              params={{ projectId, applicationId: application.id }}
              icon={<AppWindowIcon aria-hidden="true" className="size-4.5" />}
              name={application.name}
              slug={application.slug}
              status={application.status}
              meta={
                <>
                  <Badge variant="outline" className="capitalize">
                    {application.type}
                  </Badge>
                  <span className="text-text-faint truncate font-mono text-xs">
                    {application.type === "image"
                      ? application.source_image
                      : application.build_type}
                  </span>
                </>
              }
            />
          ))}
          {databases?.map((db) => (
            <ServiceRow
              key={db.id}
              to="/projects/$projectId/databases/$databaseId"
              params={{ projectId, databaseId: db.id }}
              icon={<DatabaseIcon aria-hidden="true" className="size-4.5" />}
              name={db.name}
              slug={db.slug}
              status={db.status}
              meta={
                <span className="text-text-faint font-mono text-xs">
                  {db.type}:{db.version}
                </span>
              }
            />
          ))}
        </div>
      )}
    </div>
  );
}

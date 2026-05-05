import { useState } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { useApplications } from "@/lib/hooks/use-applications";
import { useDatabases } from "@/lib/hooks/use-databases";
import { StatusBadge } from "@/lib/components/status-badge";
import {
  Database as DatabaseIcon,
  AppWindowIcon,
} from "lucide-react";
import { ApplicationFormDialog } from "@/components/applications/application-form-dialog";
import { DatabaseFormDialog } from "@/components/databases/database-form-dialog";
import { ProjectHeader } from "@/components/projects/project-header";

export const Route = createFileRoute("/_app/projects/$projectId/")({
  component: ProjectOverview,
});

function ProjectOverview() {
  const { projectId } = Route.useParams();
  const { data: applications, isLoading: applicationsLoading } =
    useApplications(projectId);
  const { data: databases, isLoading: databasesLoading } =
    useDatabases(projectId);

  const [appDialogOpen, setAppDialogOpen] = useState(false);
  const [dbDialogOpen, setDbDialogOpen] = useState(false);

  if (applicationsLoading || databasesLoading) {
    return <div className="text-muted-foreground">Loading resources...</div>;
  }

  const hasApplications = applications && applications.length > 0;
  const hasDatabases = databases && databases.length > 0;
  const isEmpty = !hasApplications && !hasDatabases;

  return (
    <div className="space-y-8">
      <ProjectHeader
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

      {isEmpty ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <p className="text-muted-foreground mb-4">
              No applications or databases yet. Click "Add New" above to get
              started.
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-3 sm:grid-cols-2">
          {applications?.map((application) => (
            <Link
              key={application.id}
              to="/projects/$projectId/applications/$applicationId"
              params={{ projectId, applicationId: application.id }}
            >
              <Card className="hover:bg-muted/50 cursor-pointer transition-colors">
                <CardHeader className="pb-2">
                  <div className="flex items-start justify-between">
                    <div className="flex items-start gap-2">
                      <AppWindowIcon className="text-muted-foreground size-4" />
                      <div className="flex flex-col">
                        <CardTitle className="text-base leading-none">
                          {application.name}
                        </CardTitle>
                        <CardDescription className="text-sm">
                          {application.slug}
                        </CardDescription>
                      </div>
                    </div>
                    <StatusBadge status={application.status} />
                  </div>
                </CardHeader>
                <CardContent>
                  <div className="text-muted-foreground flex gap-2 text-xs">
                    <Badge variant="outline" className="capitalize">
                      {application.type}
                    </Badge>
                    {application.type === "image" ? (
                      <Badge variant="outline">
                        {application.source_image}
                      </Badge>
                    ) : (
                      <Badge variant="outline" className="capitalize">
                        {application.build_type}
                      </Badge>
                    )}
                  </div>
                </CardContent>
              </Card>
            </Link>
          ))}
          {databases?.map((db) => (
            <Link
              key={db.id}
              to="/projects/$projectId/databases/$databaseId"
              params={{ projectId, databaseId: db.id }}
            >
              <Card className="hover:bg-muted/50 cursor-pointer transition-colors">
                <CardHeader className="pb-2">
                  <div className="flex items-start justify-between">
                    <div className="flex items-start gap-2">
                      <DatabaseIcon className="text-muted-foreground size-4" />
                      <div className="flex flex-col">
                        <CardTitle className="text-base leading-none">
                          {db.name}
                        </CardTitle>
                        <CardDescription className="text-sm">
                          {db.slug}
                        </CardDescription>
                      </div>
                    </div>
                    <StatusBadge status={db.status} />
                  </div>
                </CardHeader>
                <CardContent>
                  <div className="text-muted-foreground flex gap-2 text-xs">
                    <Badge variant="outline">
                      {db.type}:{db.version}
                    </Badge>
                  </div>
                </CardContent>
              </Card>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}

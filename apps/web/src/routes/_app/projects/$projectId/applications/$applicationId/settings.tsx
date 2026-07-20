import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useApplication } from "@/lib/hooks/use-applications";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Separator } from "@/components/ui/separator";
import { ApplicationSettingsForm } from "@/components/applications/application-settings-form";
import { BuildCacheSection } from "@/components/applications/build-cache-section";
import { RuntimeSection } from "@/components/applications/runtime-section";
import { ResourcesSection } from "@/components/applications/resources-section";
import { HealthCheckSection } from "@/components/applications/health-check-section";
import { DeleteApplicationDialog } from "@/components/applications/delete-application-dialog";

export const Route = createFileRoute(
  "/_app/projects/$projectId/applications/$applicationId/settings",
)({
  component: ApplicationSettingsPage,
});

function ApplicationSettingsPage() {
  const { projectId, applicationId } = Route.useParams();
  const { data: application } = useApplication(projectId, applicationId);
  const [deleteOpen, setDeleteOpen] = useState(false);

  if (!application) {
    return (
      <div className="space-y-6">
        <Card>
          <CardHeader>
            <Skeleton className="h-5 w-48" />
          </CardHeader>
          <CardContent className="space-y-4">
            {[1, 2, 3].map((i) => (
              <div key={i} className="space-y-2">
                <Skeleton className="h-4 w-24" />
                <Skeleton className="h-9 w-full" />
              </div>
            ))}
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <ApplicationSettingsForm
        projectId={projectId}
        applicationId={applicationId}
        application={application}
      />

      <Separator />

      <ResourcesSection
        projectId={projectId}
        applicationId={applicationId}
        application={application}
      />

      <Separator />

      <HealthCheckSection
        projectId={projectId}
        applicationId={applicationId}
        application={application}
      />

      {application.type === "git" && (
        <>
          <Separator />
          <BuildCacheSection
            projectId={projectId}
            applicationId={applicationId}
          />
        </>
      )}

      <Separator />

      <RuntimeSection
        projectId={projectId}
        applicationId={applicationId}
        application={application}
      />

      <Separator />

      <Card className="border-destructive/50">
        <CardHeader>
          <CardTitle className="text-destructive">Danger Zone</CardTitle>
        </CardHeader>
        <CardContent>
          <Button variant="destructive" onClick={() => setDeleteOpen(true)}>
            Delete Application
          </Button>
        </CardContent>
      </Card>

      <DeleteApplicationDialog
        projectId={projectId}
        applicationId={applicationId}
        applicationName={application.name}
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
      />
    </div>
  );
}

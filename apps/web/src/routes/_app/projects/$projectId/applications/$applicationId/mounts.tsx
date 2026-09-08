import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import {
  SegmentedControl,
  SegmentedControlItem,
} from "@/components/ui/segmented-control";
import { VolumesSection } from "@/components/applications/volumes-section";
import { VolumeBackupConfigsSection } from "@/components/applications/volume-backup-configs-section";
import { FileMountsSection } from "@/components/applications/file-mounts-section";
import { useProject } from "@/lib/hooks/use-projects";
import { useAuthStore } from "@/lib/stores/auth";

export const Route = createFileRoute(
  "/_app/projects/$projectId/applications/$applicationId/mounts",
)({
  component: ApplicationMountsPage,
});

function ApplicationMountsPage() {
  const { projectId, applicationId } = Route.useParams();
  const [view, setView] = useState("volumes");
  const { data: project } = useProject(projectId);
  const currentUser = useAuthStore((s) => s.user);
  const canDelete =
    currentUser?.role === "admin" || currentUser?.id === project?.user_id;

  return (
    <div className="space-y-6">
      <SegmentedControl value={view} onValueChange={setView}>
        <SegmentedControlItem value="volumes">Volumes</SegmentedControlItem>
        <SegmentedControlItem value="files">File Mounts</SegmentedControlItem>
      </SegmentedControl>

      {view === "volumes" ? (
        <>
          <VolumesSection
            projectId={projectId}
            applicationId={applicationId}
            canDelete={canDelete}
          />
          <VolumeBackupConfigsSection
            projectId={projectId}
            applicationId={applicationId}
          />
        </>
      ) : (
        <FileMountsSection
          projectId={projectId}
          applicationId={applicationId}
        />
      )}
    </div>
  );
}

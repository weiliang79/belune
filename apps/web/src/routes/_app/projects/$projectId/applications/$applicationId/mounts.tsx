import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import {
  SegmentedControl,
  SegmentedControlItem,
} from "@/components/ui/segmented-control";
import { VolumesSection } from "@/components/applications/volumes-section";
import { VolumeBackupConfigsSection } from "@/components/applications/volume-backup-configs-section";
import { FileMountsSection } from "@/components/applications/file-mounts-section";

export const Route = createFileRoute(
  "/_app/projects/$projectId/applications/$applicationId/mounts",
)({
  component: ApplicationMountsPage,
});

function ApplicationMountsPage() {
  const { projectId, applicationId } = Route.useParams();
  const [view, setView] = useState("volumes");

  return (
    <div className="space-y-6">
      <SegmentedControl value={view} onValueChange={setView}>
        <SegmentedControlItem value="volumes">Volumes</SegmentedControlItem>
        <SegmentedControlItem value="files">File Mounts</SegmentedControlItem>
      </SegmentedControl>

      {view === "volumes" ? (
        <>
          <VolumesSection projectId={projectId} applicationId={applicationId} />
          <VolumeBackupConfigsSection
            projectId={projectId}
            applicationId={applicationId}
          />
        </>
      ) : (
        <FileMountsSection projectId={projectId} applicationId={applicationId} />
      )}
    </div>
  );
}

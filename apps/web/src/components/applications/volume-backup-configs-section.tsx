import { useState } from "react";
import { PlusIcon } from "lucide-react";
import type { AppVolumeBackupConfig } from "@/lib/types";
import { useVolumes } from "@/lib/hooks/use-volumes";
import { useBackupDestinations } from "@/lib/hooks/use-backup-destinations";
import { useAppVolumeBackupConfigs } from "@/lib/hooks/use-volume-backups";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { VolumeBackupConfigForm } from "./volume-backup-config-form";
import { VolumeBackupConfigDrawer } from "./volume-backup-config-drawer";

interface Props {
  projectId: string;
  applicationId: string;
}

export function VolumeBackupConfigsSection({ projectId, applicationId }: Props) {
  const { data: configs, isLoading } = useAppVolumeBackupConfigs(
    projectId,
    applicationId,
  );
  const { data: destinations } = useBackupDestinations(projectId);
  const { data: volumes } = useVolumes(projectId, applicationId);

  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<AppVolumeBackupConfig | null>(null);
  const [manageTarget, setManageTarget] = useState<AppVolumeBackupConfig | null>(
    null,
  );

  const destName = (id: string) =>
    destinations?.find((d) => d.id === id)?.name ?? "Unknown destination";
  const noDestinations = destinations?.length === 0;
  const noVolumes = volumes?.length === 0;

  const openAdd = () => {
    setEditing(null);
    setFormOpen(true);
  };
  const openEdit = (cfg: AppVolumeBackupConfig) => {
    setManageTarget(null);
    setEditing(cfg);
    setFormOpen(true);
  };

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <CardTitle>Backups</CardTitle>
            <p className="text-muted-foreground mt-1 text-sm">
              Snapshot a volume to a project destination, on a schedule or on
              demand. A volume can back up to multiple destinations.
            </p>
          </div>
          <Button
            size="sm"
            className="shrink-0"
            onClick={openAdd}
            disabled={noDestinations || noVolumes}
          >
            <PlusIcon aria-hidden="true" className="size-4" />
            Add Backup
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        {noVolumes ? (
          <p className="text-muted-foreground text-sm">
            Add a volume first, then configure backups for it.
          </p>
        ) : noDestinations ? (
          <p className="text-muted-foreground text-sm">
            Add a destination on the project{" "}
            <span className="font-medium">Backups</span> tab before configuring
            backups.
          </p>
        ) : isLoading ? (
          <Skeleton className="h-16 w-full" />
        ) : !configs || configs.length === 0 ? (
          <p className="text-muted-foreground py-4 text-center text-sm">
            No backups configured yet.
          </p>
        ) : (
          <ul className="divide-border divide-y">
            {configs.map((c) => (
              <li
                key={c.id}
                className="flex items-center justify-between gap-3 py-3"
              >
                <div className="min-w-0 space-y-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-medium">{c.volume_name}</span>
                    <span className="text-text-faint font-mono text-xs">
                      {c.mount_path}
                    </span>
                    {c.enabled ? (
                      <Badge variant="outline">Active</Badge>
                    ) : (
                      <Badge variant="secondary">Disabled</Badge>
                    )}
                  </div>
                  <p className="text-text-faint text-xs">
                    {destName(c.destination_id)}
                    {" · "}
                    {c.schedule ? (
                      <code className="font-mono">{c.schedule}</code>
                    ) : (
                      "manual only"
                    )}
                    {c.prefix ? ` · ${c.prefix}` : ""}
                    {c.keep_latest != null ? ` · keep ${c.keep_latest}` : ""}
                  </p>
                </div>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => setManageTarget(c)}
                >
                  Manage
                </Button>
              </li>
            ))}
          </ul>
        )}
      </CardContent>

      <VolumeBackupConfigForm
        projectId={projectId}
        applicationId={applicationId}
        config={editing}
        open={formOpen}
        onOpenChange={setFormOpen}
      />
      {manageTarget && (
        <VolumeBackupConfigDrawer
          projectId={projectId}
          applicationId={applicationId}
          config={manageTarget}
          destinationName={destName(manageTarget.destination_id)}
          open={manageTarget !== null}
          onOpenChange={(o) => !o && setManageTarget(null)}
          onEdit={openEdit}
        />
      )}
    </Card>
  );
}

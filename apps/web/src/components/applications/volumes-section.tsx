import { useState } from "react";
import { toast } from "sonner";
import {
  DatabaseBackupIcon,
  HardDriveIcon,
  InfoIcon,
  Trash2Icon,
} from "lucide-react";
import {
  useVolumes,
  useCreateVolume,
  useDeleteVolume,
} from "@/lib/hooks/use-volumes";
import type { ApplicationVolume } from "@/lib/types";
import { VolumeBackupsDialog } from "./volume-backups-dialog";
import { formatBytes } from "@/lib/utils/format";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";

interface Props {
  projectId: string;
  applicationId: string;
}

export function VolumesSection({ projectId, applicationId }: Props) {
  const { data: volumes, isLoading } = useVolumes(projectId, applicationId);
  const createVolume = useCreateVolume(projectId, applicationId);
  const deleteVolume = useDeleteVolume(projectId, applicationId);

  const [addOpen, setAddOpen] = useState(false);
  const [name, setName] = useState("");
  const [mountPath, setMountPath] = useState("");

  const [removeTarget, setRemoveTarget] = useState<ApplicationVolume | null>(
    null,
  );
  const [deleteData, setDeleteData] = useState(false);
  const [backupsTarget, setBackupsTarget] = useState<ApplicationVolume | null>(
    null,
  );

  const resetAdd = () => {
    setName("");
    setMountPath("");
  };

  const submitAdd = () => {
    toast.promise(
      createVolume.mutateAsync({ name, mount_path: mountPath }).then(() => {
        setAddOpen(false);
        resetAdd();
      }),
      {
        loading: "Creating volume...",
        success: "Volume created — it will be mounted on the next deploy",
        error: (err) => err.message,
      },
    );
  };

  const submitRemove = () => {
    if (!removeTarget) return;
    toast.promise(
      deleteVolume
        .mutateAsync({ volumeId: removeTarget.id, deleteData })
        .then((res) => {
          setRemoveTarget(null);
          setDeleteData(false);
          if (res.warning) toast.warning(res.warning);
        }),
      {
        loading: "Removing volume...",
        success: "Volume removed",
        error: (err) => err.message,
      },
    );
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Volumes</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-start justify-between gap-4">
          <p className="text-muted-foreground text-sm">
            Persistent storage mounted into the container. Data survives
            redeploys and restarts.
          </p>
          <Button
            size="sm"
            className="shrink-0"
            onClick={() => {
              resetAdd();
              setAddOpen(true);
            }}
          >
            Add Volume
          </Button>
        </div>

        <div className="bg-muted/40 text-muted-foreground flex items-start gap-2 rounded-lg border p-3 text-sm">
          <InfoIcon aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
          <span>
            Adding or removing a volume takes effect on the next deploy — the
            container is recreated with the updated mounts. Redeploy the
            application to apply changes.
          </span>
        </div>

        {isLoading ? (
          <div className="space-y-3">
            {[1, 2].map((i) => (
              <Skeleton key={i} className="h-16 w-full" />
            ))}
          </div>
        ) : !volumes || volumes.length === 0 ? (
          <div className="text-muted-foreground flex flex-col items-center gap-2 rounded-lg border border-dashed p-10 text-center text-sm">
            <HardDriveIcon aria-hidden="true" className="size-6" />
            <p>
              No volumes yet. Add one to give this application persistent
              storage.
            </p>
          </div>
        ) : (
          <div className="space-y-3">
            {volumes.map((vol) => (
              <div
                key={vol.id}
                className="flex items-center justify-between gap-3 rounded-lg border p-4"
              >
                <div className="flex min-w-0 items-center gap-3">
                  <div className="bg-elev text-text-muted grid size-9 shrink-0 place-items-center rounded-lg">
                    <HardDriveIcon aria-hidden="true" className="size-4" />
                  </div>
                  <div className="min-w-0">
                    <div className="truncate font-medium">{vol.name}</div>
                    <div className="text-text-faint truncate font-mono text-sm">
                      {vol.mount_path}
                    </div>
                  </div>
                </div>
                <div className="flex shrink-0 items-center gap-2">
                  <span className="text-text-faint mr-2 text-sm tabular-nums">
                    {formatBytes(vol.size_bytes)}
                  </span>
                  <Button
                    size="sm"
                    variant="outline"
                    aria-label={`Backups for volume ${vol.name}`}
                    onClick={() => setBackupsTarget(vol)}
                  >
                    <DatabaseBackupIcon aria-hidden="true" className="size-4" />
                    Backups
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    aria-label={`Remove volume ${vol.name}`}
                    onClick={() => {
                      setDeleteData(false);
                      setRemoveTarget(vol);
                    }}
                  >
                    <Trash2Icon aria-hidden="true" className="size-4" />
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>

      {/* Add volume dialog */}
      <Dialog open={addOpen} onOpenChange={setAddOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Add Volume</DialogTitle>
            <DialogDescription>
              Give the volume a name and the absolute path where it should be
              mounted inside the container.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label htmlFor="volume-name">Name</Label>
              <Input
                id="volume-name"
                placeholder="data"
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
              <p className="text-text-faint text-xs">
                Lowercase letters, numbers and hyphens. Used to identify the
                stored volume.
              </p>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="volume-mount-path">Mount path</Label>
              <Input
                id="volume-mount-path"
                placeholder="/data"
                className="font-mono"
                value={mountPath}
                onChange={(e) => setMountPath(e.target.value)}
              />
              <p className="text-text-faint text-xs">
                Absolute path inside the container, e.g. <code>/data</code>.
                Cannot be a system path like <code>/</code>, <code>/tmp</code>{" "}
                or <code>/etc</code>.
              </p>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setAddOpen(false)}>
              Cancel
            </Button>
            <Button
              onClick={submitAdd}
              disabled={
                !name.trim() || !mountPath.trim() || createVolume.isPending
              }
            >
              {createVolume.isPending ? "Creating..." : "Add Volume"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Remove volume dialog */}
      <AlertDialog
        open={removeTarget !== null}
        onOpenChange={(open) => {
          if (!open) {
            setRemoveTarget(null);
            setDeleteData(false);
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Remove {removeTarget?.name}?</AlertDialogTitle>
            <AlertDialogDescription>
              The volume will be detached from the application on its next
              deploy. By default the stored data is kept and can be reattached
              by recreating a volume at the same mount path.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <label className="flex items-start gap-2 rounded-md border p-3 text-sm">
            <input
              type="checkbox"
              className="mt-0.5 size-4"
              checked={deleteData}
              onChange={(e) => setDeleteData(e.target.checked)}
            />
            <span>
              <span className="text-destructive font-medium">
                Also permanently delete the stored data
              </span>
              <span className="text-muted-foreground block">
                This cannot be undone. The underlying data volume is destroyed.
              </span>
            </span>
          </label>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={submitRemove}
              disabled={deleteVolume.isPending}
            >
              {deleteData ? "Delete volume & data" : "Remove volume"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {backupsTarget && (
        <VolumeBackupsDialog
          projectId={projectId}
          applicationId={applicationId}
          volume={backupsTarget}
          open={backupsTarget !== null}
          onOpenChange={(o) => !o && setBackupsTarget(null)}
        />
      )}
    </Card>
  );
}

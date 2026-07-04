import { useState } from "react";
import { toast } from "sonner";
import {
  CloudIcon,
  DatabaseBackupIcon,
  RotateCcwIcon,
  ScrollTextIcon,
} from "lucide-react";
import type { ApplicationVolume, VolumeBackup } from "@/lib/types";
import { useBackupDestinations } from "@/lib/hooks/use-backup-destinations";
import {
  useVolumeBackupConfigs,
  useVolumeBackups,
  useCreateVolumeBackupConfig,
  useDeleteVolumeBackupConfig,
  useRunVolumeBackup,
  useRestoreVolumeBackup,
} from "@/lib/hooks/use-volume-backups";
import { formatBytes, formatRelativeTime } from "@/lib/utils/format";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogDescription,
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
  volume: ApplicationVolume;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

function statusVariant(status: string): "default" | "secondary" | "destructive" {
  if (status === "succeeded") return "default";
  if (status === "failed") return "destructive";
  return "secondary";
}

export function VolumeBackupsDialog({
  projectId,
  applicationId,
  volume,
  open,
  onOpenChange,
}: Props) {
  const { data: destinations } = useBackupDestinations(projectId);
  const { data: configs } = useVolumeBackupConfigs(
    projectId,
    applicationId,
    volume.id,
    open,
  );
  const { data: backups } = useVolumeBackups(
    projectId,
    applicationId,
    volume.id,
    open,
  );
  const createConfig = useCreateVolumeBackupConfig(
    projectId,
    applicationId,
    volume.id,
  );
  const deleteConfig = useDeleteVolumeBackupConfig(
    projectId,
    applicationId,
    volume.id,
  );
  const runBackup = useRunVolumeBackup(projectId, applicationId, volume.id);
  const restore = useRestoreVolumeBackup(projectId, applicationId, volume.id);

  const config = configs?.[0];
  const [destinationId, setDestinationId] = useState("");
  const [quiesce, setQuiesce] = useState(false);
  const [logView, setLogView] = useState<VolumeBackup | null>(null);
  const [restoreTarget, setRestoreTarget] = useState<VolumeBackup | null>(null);

  const destName = (id: string) =>
    destinations?.find((d) => d.id === id)?.name ?? "destination";

  const saveConfig = () => {
    if (!destinationId) return;
    toast.promise(
      createConfig.mutateAsync({ destination_id: destinationId, quiesce }),
      {
        loading: "Saving backup config...",
        success: "Backup configured",
        error: (err) => err.message,
      },
    );
  };

  const backUpNow = () => {
    if (!config) return;
    toast.promise(runBackup.mutateAsync(config.id), {
      loading: "Starting backup...",
      success: "Backup started",
      error: (err) => err.message,
    });
  };

  const removeConfig = () => {
    if (!config) return;
    toast.promise(deleteConfig.mutateAsync(config.id), {
      loading: "Removing config...",
      success: "Backup config removed",
      error: (err) => err.message,
    });
  };

  const doRestore = () => {
    if (!restoreTarget) return;
    toast.promise(
      restore.mutateAsync(restoreTarget.id).then(() => setRestoreTarget(null)),
      {
        loading: "Starting restore...",
        success: "Restore started — the app will stop, restore, and restart",
        error: (err) => err.message,
      },
    );
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Backups — {volume.name}</DialogTitle>
          <DialogDescription>
            Snapshot <span className="font-mono">{volume.mount_path}</span> to an
            S3-compatible destination and restore it.
          </DialogDescription>
        </DialogHeader>

        {/* Config */}
        {!config ? (
          <div className="space-y-3">
            <div className="space-y-1.5">
              <Label>Destination</Label>
              <Select
                value={destinationId}
                onValueChange={(v) => setDestinationId(v ?? "")}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select destination" />
                </SelectTrigger>
                <SelectContent>
                  {(destinations ?? []).map((d) => (
                    <SelectItem key={d.id} value={d.id} icon={<CloudIcon />}>
                      {d.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {destinations && destinations.length === 0 && (
                <p className="text-muted-foreground text-xs">
                  No destinations yet — add one on the project Backups tab first.
                </p>
              )}
            </div>
            <label className="flex items-start gap-2 text-sm">
              <input
                type="checkbox"
                className="mt-0.5 size-4"
                checked={quiesce}
                onChange={(e) => setQuiesce(e.target.checked)}
              />
              <span>
                Stop the app during backup (quiesce)
                <span className="text-muted-foreground block">
                  Off = live snapshot with no downtime. Turn on for
                  consistency-critical data (e.g. an embedded database).
                </span>
              </span>
            </label>
            <Button
              onClick={saveConfig}
              disabled={!destinationId || createConfig.isPending}
            >
              Save backup config
            </Button>
          </div>
        ) : (
          <div className="flex items-center justify-between gap-3 rounded-lg border p-3">
            <div className="min-w-0 text-sm">
              <div className="flex items-center gap-2">
                <CloudIcon aria-hidden="true" className="size-4" />
                <span className="truncate font-medium">
                  {destName(config.destination_id)}
                </span>
                {config.quiesce && <Badge variant="secondary">Quiesce</Badge>}
              </div>
              {config.last_run_at && (
                <div className="text-text-faint mt-0.5 text-xs">
                  Last run {formatRelativeTime(config.last_run_at)}
                </div>
              )}
            </div>
            <div className="flex shrink-0 items-center gap-2">
              <Button size="sm" onClick={backUpNow} disabled={runBackup.isPending}>
                <DatabaseBackupIcon aria-hidden="true" className="size-4" />
                Back up now
              </Button>
              <Button size="sm" variant="outline" onClick={removeConfig}>
                Remove
              </Button>
            </div>
          </div>
        )}

        <Separator />

        {/* Runs */}
        <div className="space-y-2">
          <div className="text-sm font-medium">Recent backups</div>
          {!backups || backups.length === 0 ? (
            <p className="text-muted-foreground text-sm">No backups yet.</p>
          ) : (
            <div className="max-h-64 space-y-2 overflow-y-auto">
              {backups.map((b) => (
                <div
                  key={b.id}
                  className="flex items-center justify-between gap-3 rounded-md border p-2.5 text-sm"
                >
                  <div className="flex min-w-0 items-center gap-2">
                    <Badge variant={statusVariant(b.status)}>{b.status}</Badge>
                    <span className="text-text-faint tabular-nums">
                      {formatBytes(b.size_bytes)}
                    </span>
                    <span className="text-text-faint truncate">
                      {formatRelativeTime(b.started_at)}
                    </span>
                  </div>
                  <div className="flex shrink-0 items-center gap-1">
                    {b.log && (
                      <Button
                        size="sm"
                        variant="ghost"
                        aria-label="View log"
                        onClick={() => setLogView(b)}
                      >
                        <ScrollTextIcon aria-hidden="true" className="size-4" />
                      </Button>
                    )}
                    {b.status === "succeeded" && b.has_remote && (
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => setRestoreTarget(b)}
                      >
                        <RotateCcwIcon aria-hidden="true" className="size-4" />
                        Restore
                      </Button>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Log viewer */}
        <Dialog open={logView !== null} onOpenChange={(o) => !o && setLogView(null)}>
          <DialogContent className="max-w-2xl">
            <DialogHeader>
              <DialogTitle>Backup log</DialogTitle>
            </DialogHeader>
            <pre className="bg-muted/40 max-h-96 overflow-auto rounded-md border p-3 font-mono text-xs whitespace-pre-wrap">
              {logView?.log || "No log."}
            </pre>
          </DialogContent>
        </Dialog>

        {/* Restore confirm */}
        <AlertDialog
          open={restoreTarget !== null}
          onOpenChange={(o) => !o && setRestoreTarget(null)}
        >
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Restore this backup?</AlertDialogTitle>
              <AlertDialogDescription>
                This stops the application, replaces the current contents of{" "}
                <span className="font-mono">{volume.mount_path}</span> with the
                snapshot, and restarts the app. Current data in the volume is
                overwritten.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancel</AlertDialogCancel>
              <AlertDialogAction
                onClick={doRestore}
                disabled={restore.isPending}
              >
                Restore
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </DialogContent>
    </Dialog>
  );
}

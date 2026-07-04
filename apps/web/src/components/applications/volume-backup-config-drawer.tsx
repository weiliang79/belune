import { useMemo, useState } from "react";
import { toast } from "sonner";
import {
  CloudIcon,
  DatabaseBackupIcon,
  PencilIcon,
  RotateCcwIcon,
  ScrollTextIcon,
  Trash2Icon,
} from "lucide-react";
import type { AppVolumeBackupConfig } from "@/lib/types";
import {
  useVolumeBackups,
  useVolumeRestores,
  useDeleteVolumeBackupConfig,
  useRunVolumeBackup,
  useRestoreVolumeBackup,
} from "@/lib/hooks/use-volume-backups";
import { formatBytes, formatRelativeTime } from "@/lib/utils/format";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import {
  Dialog,
  DialogContent,
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
  config: AppVolumeBackupConfig;
  destinationName?: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onEdit: (config: AppVolumeBackupConfig) => void;
}

function statusVariant(status: string): "default" | "secondary" | "destructive" {
  if (status === "succeeded") return "default";
  if (status === "failed") return "destructive";
  return "secondary";
}

export function VolumeBackupConfigDrawer({
  projectId,
  applicationId,
  config,
  destinationName,
  open,
  onOpenChange,
  onEdit,
}: Props) {
  const volumeId = config.application_volume_id;
  const { data: allBackups } = useVolumeBackups(
    projectId,
    applicationId,
    volumeId,
    open,
  );
  const { data: allRestores } = useVolumeRestores(
    projectId,
    applicationId,
    volumeId,
    open,
  );
  const runBackup = useRunVolumeBackup(projectId, applicationId, volumeId);
  const deleteConfig = useDeleteVolumeBackupConfig(
    projectId,
    applicationId,
    volumeId,
  );
  const restore = useRestoreVolumeBackup(projectId, applicationId, volumeId);

  const [logView, setLogView] = useState<{ title: string; log: string } | null>(
    null,
  );
  const [restoreTarget, setRestoreTarget] = useState<string | null>(null);

  // Scope runs + restores to this config (a volume may have several configs).
  const backups = useMemo(
    () => (allBackups ?? []).filter((b) => b.config_id === config.id),
    [allBackups, config.id],
  );
  const restores = useMemo(() => {
    const ids = new Set(backups.map((b) => b.id));
    return (allRestores ?? []).filter(
      (r) => r.backup_id && ids.has(r.backup_id),
    );
  }, [allRestores, backups]);

  const backUpNow = () => {
    toast.promise(runBackup.mutateAsync(config.id), {
      loading: "Starting backup...",
      success: "Backup started",
      error: (err) => err.message,
    });
  };

  const removeConfig = () => {
    toast.promise(
      deleteConfig.mutateAsync(config.id).then(() => onOpenChange(false)),
      {
        loading: "Removing config...",
        success: "Backup config removed",
        error: (err) => err.message,
      },
    );
  };

  const doRestore = () => {
    if (!restoreTarget) return;
    toast.promise(
      restore.mutateAsync(restoreTarget).then(() => setRestoreTarget(null)),
      {
        loading: "Starting restore...",
        success: "Restore started — the app will stop, restore, and restart",
        error: (err) => err.message,
      },
    );
  };

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="sm:max-w-lg">
        <SheetHeader>
          <SheetTitle>Backups — {config.volume_name}</SheetTitle>
          <SheetDescription>
            Snapshot <span className="font-mono">{config.mount_path}</span> to{" "}
            {destinationName ?? "the destination"} and restore it.
          </SheetDescription>
        </SheetHeader>

        <div className="space-y-4">
          {/* Config summary + actions */}
          <div className="flex items-center justify-between gap-3 rounded-lg border p-3">
            <div className="min-w-0 text-sm">
              <div className="flex items-center gap-2">
                <CloudIcon aria-hidden="true" className="size-4" />
                <span className="truncate font-medium">
                  {destinationName ?? "Destination"}
                </span>
                {config.quiesce && <Badge variant="secondary">Quiesce</Badge>}
                {!config.enabled && <Badge variant="secondary">Disabled</Badge>}
              </div>
              <div className="text-text-faint mt-0.5 text-xs">
                {config.schedule ? (
                  <code className="font-mono">{config.schedule}</code>
                ) : (
                  "Manual only"
                )}
                {config.keep_latest != null && ` · keep ${config.keep_latest}`}
                {config.last_run_at &&
                  ` · last run ${formatRelativeTime(config.last_run_at)}`}
              </div>
            </div>
            <div className="flex shrink-0 items-center gap-2">
              <Button
                size="sm"
                onClick={backUpNow}
                disabled={runBackup.isPending}
              >
                <DatabaseBackupIcon aria-hidden="true" className="size-4" />
                Back up now
              </Button>
              <Button
                size="sm"
                variant="outline"
                aria-label="Edit config"
                onClick={() => onEdit(config)}
              >
                <PencilIcon aria-hidden="true" className="size-4" />
              </Button>
              <Button
                size="sm"
                variant="outline"
                aria-label="Delete config"
                className="text-destructive hover:bg-destructive/10 hover:text-destructive"
                onClick={removeConfig}
              >
                <Trash2Icon aria-hidden="true" className="size-4" />
              </Button>
            </div>
          </div>

          <Separator />

          {/* Backup runs */}
          <div className="space-y-2">
            <div className="text-sm font-medium">Recent backups</div>
            {backups.length === 0 ? (
              <p className="text-muted-foreground text-sm">No backups yet.</p>
            ) : (
              <div className="space-y-2">
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
                          onClick={() =>
                            setLogView({ title: "Backup log", log: b.log ?? "" })
                          }
                        >
                          <ScrollTextIcon aria-hidden="true" className="size-4" />
                        </Button>
                      )}
                      {b.status === "succeeded" && b.has_remote && (
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => setRestoreTarget(b.id)}
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

          {/* Restore runs */}
          {restores.length > 0 && (
            <>
              <Separator />
              <div className="space-y-2">
                <div className="text-sm font-medium">Recent restores</div>
                <div className="space-y-2">
                  {restores.map((rr) => (
                    <div
                      key={rr.id}
                      className="flex items-center justify-between gap-3 rounded-md border p-2.5 text-sm"
                    >
                      <div className="flex min-w-0 items-center gap-2">
                        <Badge variant={statusVariant(rr.status)}>
                          {rr.status}
                        </Badge>
                        <span className="text-text-faint truncate">
                          {formatRelativeTime(rr.started_at)}
                        </span>
                      </div>
                      {rr.log && (
                        <Button
                          size="sm"
                          variant="ghost"
                          aria-label="View restore log"
                          onClick={() =>
                            setLogView({
                              title: "Restore log",
                              log: rr.log ?? "",
                            })
                          }
                        >
                          <ScrollTextIcon aria-hidden="true" className="size-4" />
                        </Button>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            </>
          )}
        </div>

        {/* Log viewer */}
        <Dialog open={logView !== null} onOpenChange={(o) => !o && setLogView(null)}>
          <DialogContent className="max-w-2xl">
            <DialogHeader>
              <DialogTitle>{logView?.title ?? "Log"}</DialogTitle>
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
                <span className="font-mono">{config.mount_path}</span> with the
                snapshot, and restarts the app. Current data in the volume is
                overwritten.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancel</AlertDialogCancel>
              <AlertDialogAction onClick={doRestore} disabled={restore.isPending}>
                Restore
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </SheetContent>
    </Sheet>
  );
}

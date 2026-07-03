import { useState } from "react";
import { toast } from "sonner";
import {
  DatabaseBackup as DatabaseBackupIcon,
  Loader2,
  RotateCcw,
  Trash2,
  Pencil,
  Cloud,
  ScrollText,
} from "lucide-react";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from "@/components/ui/sheet";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipPositioner,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import {
  useDatabaseBackups,
  useRestoreDatabase,
  useDeleteDatabaseBackup,
} from "@/lib/hooks/use-databases";
import {
  useRunBackupConfig,
  useDeleteBackupConfig,
} from "@/lib/hooks/use-database-backup-configs";
import { formatBytes, formatDateTimeShort } from "@/lib/utils/format";
import { cn } from "@/lib/utils";
import type { Database, DatabaseBackup, DatabaseBackupConfig } from "@/lib/types";

function statusTone(status: DatabaseBackup["status"]): string {
  switch (status) {
    case "succeeded":
      return "text-status-ready";
    case "failed":
      return "text-status-error";
    default:
      return "text-status-building";
  }
}

interface Props {
  db: Database;
  config: DatabaseBackupConfig | null;
  destinationName?: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onEdit: (config: DatabaseBackupConfig) => void;
}

export function BackupConfigRunsSheet({
  db,
  config,
  destinationName,
  open,
  onOpenChange,
  onEdit,
}: Props) {
  const { data: allBackups, isLoading } = useDatabaseBackups(
    db.project_id,
    db.id,
  );
  const run = useRunBackupConfig(db.project_id, db.id);
  const restore = useRestoreDatabase(db.project_id, db.id);
  const deleteBackup = useDeleteDatabaseBackup(db.project_id, db.id);
  const deleteConfig = useDeleteBackupConfig(db.project_id, db.id);

  const [logRun, setLogRun] = useState<DatabaseBackup | null>(null);

  const runs = (allBackups ?? []).filter((b) => b.config_id === config?.id);
  const anyRunning = runs.some((b) => b.status === "running");

  if (!config) return null;

  const handleRunNow = () => {
    toast.promise(run.mutateAsync(config.id), {
      loading: "Starting backup…",
      success: "Backup started",
      error: (err) => err.message,
    });
  };

  const handleRestore = (backupId: string) => {
    toast.promise(restore.mutateAsync(backupId), {
      loading: "Starting restore…",
      success: "Restore started — the database stays online",
      error: (err) => err.message,
    });
  };

  const handleDeleteRun = (backupId: string) => {
    toast.promise(deleteBackup.mutateAsync(backupId), {
      loading: "Deleting backup…",
      success: "Backup deleted",
      error: (err) => err.message,
    });
  };

  const handleDeleteConfig = () => {
    toast.promise(deleteConfig.mutateAsync(config.id), {
      loading: "Deleting backup configuration…",
      success: () => {
        onOpenChange(false);
        return "Backup configuration deleted";
      },
      error: (err) => err.message,
    });
  };

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>Backup runs</SheetTitle>
          <SheetDescription>
            {destinationName ?? "Destination"} · {config.schedule}
          </SheetDescription>
        </SheetHeader>

        <div className="flex flex-wrap gap-2">
          <Button
            size="sm"
            onClick={handleRunNow}
            disabled={run.isPending || anyRunning}
          >
            <DatabaseBackupIcon className="mr-1 h-4 w-4" />
            Manual backup now
          </Button>
          <Button size="sm" variant="outline" onClick={() => onEdit(config)}>
            <Pencil className="mr-1 h-4 w-4" />
            Edit config
          </Button>
          <AlertDialog>
            <AlertDialogTrigger
              render={
                <Button
                  size="sm"
                  variant="outline"
                  className="text-destructive hover:bg-destructive/10 hover:text-destructive"
                />
              }
            >
              <Trash2 className="mr-1 h-4 w-4" />
              Delete config
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>Delete backup configuration?</AlertDialogTitle>
                <AlertDialogDescription>
                  This removes the schedule and deletes the backups it produced
                  (local files and remote objects). This can't be undone.
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>Cancel</AlertDialogCancel>
                <AlertDialogAction
                  onClick={handleDeleteConfig}
                  className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                >
                  Delete
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </div>

        {isLoading ? (
          <p className="text-text-faint text-sm">Loading runs…</p>
        ) : runs.length === 0 ? (
          <p className="text-text-faint py-4 text-center text-sm">
            No runs yet for this configuration.
          </p>
        ) : (
          <ul className="divide-border divide-y">
            {runs.map((b) => (
              <li
                key={b.id}
                className="flex items-center justify-between gap-3 py-3"
              >
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <span
                      className={cn(
                        "text-sm font-medium capitalize",
                        statusTone(b.status),
                      )}
                    >
                      {b.status === "running" && (
                        <Loader2 className="mr-1 inline h-3 w-3 animate-spin" />
                      )}
                      {b.status}
                    </span>
                    {b.has_remote && (
                      <Cloud
                        className="text-text-faint h-3.5 w-3.5"
                        aria-label="Uploaded to destination"
                      />
                    )}
                    {b.status === "succeeded" && (
                      <span className="text-text-faint text-xs">
                        {formatBytes(b.size_bytes)}
                      </span>
                    )}
                  </div>
                  <p className="text-text-faint text-xs">
                    {formatDateTimeShort(b.started_at)}
                  </p>
                  {b.error && (
                    <p className="text-status-error mt-0.5 truncate text-xs">
                      {b.error}
                    </p>
                  )}
                </div>
                <div className="flex items-center gap-1">
                  {b.status !== "running" && (b.log || b.error) && (
                    <Tooltip>
                      <TooltipTrigger
                        render={
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            aria-label="View log"
                            onClick={() => setLogRun(b)}
                          />
                        }
                      >
                        <ScrollText className="h-4 w-4" />
                      </TooltipTrigger>
                      <TooltipPositioner>
                        <TooltipContent>View log</TooltipContent>
                      </TooltipPositioner>
                    </Tooltip>
                  )}
                  {b.status === "succeeded" && (
                    <AlertDialog>
                      <AlertDialogTrigger
                        render={
                          <Button variant="outline" size="sm" />
                        }
                      >
                        <RotateCcw className="mr-1 h-4 w-4" />
                        Restore
                      </AlertDialogTrigger>
                      <AlertDialogContent>
                        <AlertDialogHeader>
                          <AlertDialogTitle>Restore this backup?</AlertDialogTitle>
                          <AlertDialogDescription>
                            This replaces the current contents of{" "}
                            <span className="font-medium">{db.name}</span> with
                            the backup from{" "}
                            {formatDateTimeShort(b.started_at)}. Data
                            written since then will be lost.
                          </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                          <AlertDialogCancel>Cancel</AlertDialogCancel>
                          <AlertDialogAction onClick={() => handleRestore(b.id)}>
                            Restore
                          </AlertDialogAction>
                        </AlertDialogFooter>
                      </AlertDialogContent>
                    </AlertDialog>
                  )}
                  {b.status !== "running" && (
                    <AlertDialog>
                      <Tooltip>
                        <TooltipTrigger
                          render={
                            <AlertDialogTrigger
                              render={
                                <Button
                                  variant="ghost"
                                  size="icon-sm"
                                  aria-label="Delete backup"
                                />
                              }
                            />
                          }
                        >
                          <Trash2 className="h-4 w-4" />
                        </TooltipTrigger>
                        <TooltipPositioner>
                          <TooltipContent>Delete backup</TooltipContent>
                        </TooltipPositioner>
                      </Tooltip>
                      <AlertDialogContent>
                        <AlertDialogHeader>
                          <AlertDialogTitle>Delete this backup?</AlertDialogTitle>
                          <AlertDialogDescription>
                            Permanently removes this backup (local file and any
                            remote copy). This can't be undone.
                          </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                          <AlertDialogCancel>Cancel</AlertDialogCancel>
                          <AlertDialogAction onClick={() => handleDeleteRun(b.id)}>
                            Delete
                          </AlertDialogAction>
                        </AlertDialogFooter>
                      </AlertDialogContent>
                    </AlertDialog>
                  )}
                </div>
              </li>
            ))}
          </ul>
        )}
      </SheetContent>

      <Dialog
        open={logRun !== null}
        onOpenChange={(o) => !o && setLogRun(null)}
      >
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>Backup log</DialogTitle>
            <DialogDescription>
              {logRun && (
                <>
                  {logRun.status} ·{" "}
                  {formatDateTimeShort(logRun.started_at)}
                  {logRun.status === "succeeded"
                    ? ` · ${formatBytes(logRun.size_bytes)}`
                    : ""}
                  {logRun.remote_key ? ` · ${logRun.remote_key}` : ""}
                </>
              )}
            </DialogDescription>
          </DialogHeader>
          <pre className="bg-muted max-h-[60vh] overflow-auto rounded-md p-3 font-mono text-xs whitespace-pre-wrap">
            {logRun?.log?.trim() ||
              logRun?.error ||
              "No log recorded for this run."}
          </pre>
        </DialogContent>
      </Dialog>
    </Sheet>
  );
}

import { useState } from "react";
import {
  Archive,
  DatabaseIcon,
  Loader2,
  RotateCcwIcon,
  Trash2,
} from "lucide-react";
import { toast } from "sonner";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
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
  useDeleteOrphanedBackup,
  useOrphanedBackups,
  useRestoreOrphanedBackup,
} from "@/lib/hooks/use-databases";
import { formatBytes, formatDateTimeShort } from "@/lib/utils/format";
import type { OrphanedBackup } from "@/lib/api/databases";
import {
  Tooltip,
  TooltipContent,
  TooltipPositioner,
  TooltipTrigger,
} from "@/components/ui/tooltip";

/**
 * How many OTHER backups come back alongside this one.
 *
 * Restoring reclaims every backup on the tombstone, not just the row that was
 * clicked — the replacement inherits the full history. That is the one part of
 * this action nobody would guess, so the dialog says it.
 */
function siblingCount(
  backups: OrphanedBackup[] | undefined,
  backup: OrphanedBackup,
): number {
  if (!backups) return 0;
  return (
    backups.filter((x) => x.tombstone_id === backup.tombstone_id).length - 1
  );
}

/**
 * Backups whose database has been deleted.
 *
 * These have nowhere else to appear: the per-database Backups tab went with the
 * database. Without this panel they would be storage an install keeps paying
 * for with no screen acknowledging it exists.
 */
export function OrphanedBackupsPanel({ projectId }: { projectId: string }) {
  const { data: backups, isLoading } = useOrphanedBackups(projectId);
  const restore = useRestoreOrphanedBackup(projectId);
  const remove = useDeleteOrphanedBackup(projectId);
  const [pendingId, setPendingId] = useState<string | null>(null);

  // Nothing orphaned is the normal state; an empty card would just be noise.
  if (!isLoading && (!backups || backups.length === 0)) return null;

  const handleRestore = (backupId: string, name: string) => {
    setPendingId(backupId);
    toast.promise(
      restore.mutateAsync(backupId).finally(() => setPendingId(null)),
      {
        loading: `Recreating ${name}...`,
        success: `${name} is being recreated and restored`,
        error: (err) => err.message,
      },
    );
  };

  const handleDelete = (backupId: string, name: string) => {
    setPendingId(backupId);
    toast.promise(
      remove.mutateAsync(backupId).finally(() => setPendingId(null)),
      {
        loading: "Deleting backup...",
        success: `Backup of ${name} deleted`,
        error: (err) => err.message,
      },
    );
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Archive aria-hidden="true" className="size-4" />
          Backups From Deleted Databases
        </CardTitle>
        <CardDescription>
          These outlived the databases they came from. Restoring one recreates
          the database under its original name, so applications reconnect
          without any configuration change.
        </CardDescription>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Skeleton className="h-16 w-full" />
        ) : (
          <ul className="divide-border divide-y">
            {(backups ?? []).map((b) => {
              const siblings = siblingCount(backups, b);
              return (
                <li
                  key={b.id}
                  className="flex flex-wrap items-center justify-between gap-3 py-3"
                >
                  <div className="flex min-w-0 items-center gap-3">
                    <DatabaseIcon
                      aria-hidden="true"
                      className="text-text-muted size-4 shrink-0"
                    />
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">
                        {b.database_name}
                      </p>
                      <p className="text-text-faint text-xs">
                        {b.status !== "succeeded" ? `${b.status} · ` : ""}
                        {b.database_type} &middot; {formatBytes(b.size_bytes)}{" "}
                        &middot; taken {formatDateTimeShort(b.started_at)}{" "}
                        &middot; database deleted{" "}
                        {formatDateTimeShort(b.database_deleted_at)}
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center gap-1">
                    {/* A failed or half-finished run has no archive to restore,
                      and the API rejects it — an enabled button here could only
                      ever produce an error toast. */}
                    <Tooltip>
                      <AlertDialog>
                        <TooltipTrigger
                          render={
                            <AlertDialogTrigger
                              render={
                                <Button
                                  variant="ghost"
                                  size="icon-sm"
                                  aria-label="Restore backup"
                                  disabled={
                                    pendingId === b.id ||
                                    b.status !== "succeeded"
                                  }
                                />
                              }
                            >
                              {pendingId === b.id ? (
                                <Loader2
                                  aria-hidden="true"
                                  className="animate-spin"
                                />
                              ) : (
                                <RotateCcwIcon aria-hidden="true" />
                              )}
                            </AlertDialogTrigger>
                          }
                        />

                        <TooltipPositioner>
                          <TooltipContent>Restore Backup</TooltipContent>
                        </TooltipPositioner>

                        <AlertDialogContent>
                          <AlertDialogHeader>
                            <AlertDialogTitle>
                              Restore this backup?
                            </AlertDialogTitle>
                            <AlertDialogDescription>
                              This recreates {b.database_name} with the data
                              from this backup, taken{" "}
                              {formatDateTimeShort(b.started_at)}. It comes back
                              under its original name, so applications in this
                              project reconnect without any configuration
                              change.
                            </AlertDialogDescription>
                            {siblings > 0 ? (
                              <AlertDialogDescription>
                                Its other {siblings}{" "}
                                {siblings === 1 ? "backup" : "backups"} return
                                with it. Scheduled backups and external access
                                are not restored.
                              </AlertDialogDescription>
                            ) : (
                              <AlertDialogDescription>
                                Scheduled backups and external access are not
                                restored.
                              </AlertDialogDescription>
                            )}
                          </AlertDialogHeader>
                          <AlertDialogFooter>
                            <AlertDialogCancel>Cancel</AlertDialogCancel>
                            <AlertDialogAction
                              onClick={() =>
                                handleRestore(b.id, b.database_name)
                              }
                            >
                              Restore
                            </AlertDialogAction>
                          </AlertDialogFooter>
                        </AlertDialogContent>
                      </AlertDialog>
                    </Tooltip>

                    <Tooltip>
                      <AlertDialog>
                        <TooltipTrigger
                          render={
                            <AlertDialogTrigger
                              render={
                                <Button
                                  variant="ghost"
                                  size="icon-sm"
                                  aria-label={`Delete backup of ${b.database_name}`}
                                  className="text-destructive hover:bg-destructive/10 hover:text-destructive"
                                  disabled={pendingId === b.id}
                                />
                              }
                            >
                              <Trash2 aria-hidden="true" />
                            </AlertDialogTrigger>
                          }
                        />
                        <TooltipPositioner>
                          <TooltipContent>Delete Backup</TooltipContent>
                        </TooltipPositioner>

                        <AlertDialogContent>
                          <AlertDialogHeader>
                            <AlertDialogTitle>
                              Delete this backup?
                            </AlertDialogTitle>
                            <AlertDialogDescription>
                              This permanently erases the archive of{" "}
                              {b.database_name}
                              {b.has_remote
                                ? ", including the copy at its remote destination"
                                : ""}
                              . The database it came from is already gone, so
                              this is the last copy — it cannot be undone.
                            </AlertDialogDescription>
                          </AlertDialogHeader>
                          <AlertDialogFooter>
                            <AlertDialogCancel>Cancel</AlertDialogCancel>
                            <AlertDialogAction
                              onClick={() =>
                                handleDelete(b.id, b.database_name)
                              }
                              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                            >
                              Delete
                            </AlertDialogAction>
                          </AlertDialogFooter>
                        </AlertDialogContent>
                      </AlertDialog>
                    </Tooltip>
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}

import { useState } from "react";
import { toast } from "sonner";
import { Plus, Trash2, Pencil, PlugZap, Loader2 } from "lucide-react";
import {
  useBackupDestinations,
  useDeleteBackupDestination,
  useTestBackupDestination,
} from "@/lib/hooks/use-backup-destinations";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
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
import {
  Tooltip,
  TooltipContent,
  TooltipPositioner,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { Skeleton } from "@/components/ui/skeleton";
import { DestinationFormDialog } from "./destination-form-dialog";
import type { BackupDestination } from "@/lib/types";

export function DestinationsPanel({ projectId }: { projectId: string }) {
  const { data: destinations, isLoading } = useBackupDestinations(projectId);
  const del = useDeleteBackupDestination(projectId);
  const test = useTestBackupDestination(projectId);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<BackupDestination | null>(null);
  const [testingId, setTestingId] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<BackupDestination | null>(
    null,
  );

  const openCreate = () => {
    setEditing(null);
    setDialogOpen(true);
  };
  const openEdit = (d: BackupDestination) => {
    setEditing(d);
    setDialogOpen(true);
  };

  const handleTest = (d: BackupDestination) => {
    setTestingId(d.id);
    test
      .mutateAsync(d.id)
      .then((res) => {
        if (res.ok) toast.success(`Connected to ${d.bucket}`);
        else toast.error(res.error ?? "Connection failed");
      })
      .catch((err) => toast.error(err.message))
      .finally(() => setTestingId(null));
  };

  const handleDelete = (d: BackupDestination) => {
    toast.promise(del.mutateAsync(d.id), {
      loading: "Removing destination…",
      success: "Destination removed",
      error: (err) => err.message,
    });
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between gap-3">
        <div>
          <CardTitle>Backup Destinations</CardTitle>
          <CardDescription>
            S3-compatible storage targets for this project's database backups.
          </CardDescription>
        </div>
        <Button size="sm" onClick={openCreate}>
          <Plus className="mr-1 h-4 w-4" /> Add destination
        </Button>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Skeleton className="h-16 w-full" />
        ) : !destinations || destinations.length === 0 ? (
          <p className="text-muted-foreground text-sm">
            No destinations yet. Add one so databases in this project can be
            backed up to remote storage.
          </p>
        ) : (
          <div className="divide-border divide-y">
            {destinations.map((d) => (
              <div
                key={d.id}
                className="flex items-center justify-between gap-3 py-3"
              >
                <div className="min-w-0 space-y-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-medium">{d.name}</span>
                    <Badge variant="outline" className="uppercase">
                      {d.provider}
                    </Badge>
                  </div>
                  <p className="text-muted-foreground truncate text-xs">
                    {d.bucket}
                    {d.prefix ? `/${d.prefix}` : ""}
                    {d.region ? ` · ${d.region}` : ""}
                    {d.endpoint ? ` · ${d.endpoint}` : ""}
                  </p>
                </div>
                <div className="flex items-center gap-1">
                  <Tooltip>
                    <TooltipTrigger
                      render={
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          aria-label="Test connection"
                          disabled={testingId === d.id}
                          onClick={() => handleTest(d)}
                        />
                      }
                    >
                      {testingId === d.id ? (
                        <Loader2 className="h-4 w-4 animate-spin" />
                      ) : (
                        <PlugZap className="h-4 w-4" />
                      )}
                    </TooltipTrigger>
                    <TooltipPositioner>
                      <TooltipContent>Test connection</TooltipContent>
                    </TooltipPositioner>
                  </Tooltip>
                  <Tooltip>
                    <TooltipTrigger
                      render={
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          aria-label="Edit destination"
                          onClick={() => openEdit(d)}
                        />
                      }
                    >
                      <Pencil className="h-4 w-4" />
                    </TooltipTrigger>
                    <TooltipPositioner>
                      <TooltipContent>Edit</TooltipContent>
                    </TooltipPositioner>
                  </Tooltip>
                  <Tooltip>
                    <TooltipTrigger
                      render={
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          aria-label="Remove destination"
                          className="text-destructive hover:bg-destructive/10 hover:text-destructive"
                          onClick={() => setDeleteTarget(d)}
                        />
                      }
                    >
                      <Trash2 className="h-4 w-4" />
                    </TooltipTrigger>
                    <TooltipPositioner>
                      <TooltipContent>Remove</TooltipContent>
                    </TooltipPositioner>
                  </Tooltip>
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>

      <DestinationFormDialog
        projectId={projectId}
        destination={editing}
        open={dialogOpen}
        onOpenChange={setDialogOpen}
      />

      <AlertDialog
        open={!!deleteTarget}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Remove {deleteTarget?.name}?</AlertDialogTitle>
            <AlertDialogDescription>
              This deletes the destination. It cannot be removed while a database
              backup configuration still uses it.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (deleteTarget) handleDelete(deleteTarget);
                setDeleteTarget(null);
              }}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              Remove
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  );
}

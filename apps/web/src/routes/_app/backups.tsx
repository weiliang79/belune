import { createFileRoute } from "@tanstack/react-router";
import { RouteError } from "@/lib/components/route-error";
import { toast } from "sonner";
import {
  useBackupRuns,
  useBackupStatus,
  useTriggerBackup,
} from "@/lib/hooks/use-backups";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { StatusPill } from "@/components/ui/status-pill";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

export const Route = createFileRoute("/_app/backups")({
  component: BackupsPage,
  errorComponent: RouteError,
});

function formatBytes(bytes: number) {
  if (bytes === 0) return "—";
  const mb = bytes / (1024 * 1024);
  if (mb >= 1) return `${mb.toFixed(1)} MB`;
  const kb = bytes / 1024;
  return `${kb.toFixed(0)} KB`;
}

function formatDate(iso: string | null) {
  if (!iso) return "—";
  return new Date(iso).toLocaleString();
}

function BackupsPage() {
  const { data: status, isLoading: statusLoading } = useBackupStatus();
  const { data: runs, isLoading: runsLoading } = useBackupRuns();
  const trigger = useTriggerBackup();

  const isRunning = runs?.[0]?.status === "running";
  const lastRunFailed = runs?.[0]?.status === "failed" || !!status?.last_error;

  const handleTrigger = () => {
    toast.promise(trigger.mutateAsync(), {
      loading: "Queueing backup…",
      success: "Backup queued",
      error: (err) => err.message ?? "Failed to trigger backup",
    });
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Backups</h1>
          <p className="text-muted-foreground text-sm">
            Database and configuration backups.
          </p>
        </div>
        <Button
          onClick={handleTrigger}
          disabled={trigger.isPending || isRunning}
        >
          {isRunning
            ? "Backup in progress…"
            : trigger.isPending
              ? "Queueing…"
              : "Run Backup Now"}
        </Button>
      </div>

      {/* Status card */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>Status</CardTitle>
            {status && (
              <Badge variant={lastRunFailed ? "destructive" : "default"}>
                {lastRunFailed ? "Last run failed" : "Healthy"}
              </Badge>
            )}
          </div>
        </CardHeader>
        <CardContent>
          {statusLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-4 w-48" />
              <Skeleton className="h-4 w-40" />
            </div>
          ) : status ? (
            <div className="grid grid-cols-2 gap-4 text-sm md:grid-cols-4">
              <StatusItem
                label="Last succeeded"
                value={formatDate(status.last_succeeded_at)}
              />
              <StatusItem
                label="Last attempted"
                value={formatDate(status.last_attempted_at)}
              />
              <StatusItem
                label="Remote storage"
                value={
                  <Badge
                    variant={status.remote_enabled ? "default" : "secondary"}
                  >
                    {status.remote_enabled ? "Enabled" : "Disabled"}
                  </Badge>
                }
              />
              <StatusItem
                label="Retention"
                value={`${status.retention.count} backups / ${status.retention.days} days`}
              />
              {status.last_error && (
                <div className="border-destructive/30 bg-destructive/10 col-span-2 mt-1 rounded-lg border p-3 md:col-span-4">
                  <p className="text-destructive text-xs font-semibold">
                    Last error
                  </p>
                  <p className="text-destructive/90 mt-1 font-mono text-xs">
                    {status.last_error}
                  </p>
                </div>
              )}
            </div>
          ) : null}
        </CardContent>
      </Card>

      {/* Run history */}
      <Card>
        <CardHeader>
          <CardTitle>Recent runs</CardTitle>
        </CardHeader>
        <CardContent>
          {runsLoading ? (
            <div className="space-y-2">
              {[1, 2, 3].map((i) => (
                <Skeleton key={i} className="h-10 w-full" />
              ))}
            </div>
          ) : !runs || runs.length === 0 ? (
            <p className="text-muted-foreground py-6 text-center text-sm">
              No backup runs yet. Click "Run Backup Now" to start one.
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Status</TableHead>
                  <TableHead>Started</TableHead>
                  <TableHead>Finished</TableHead>
                  <TableHead>Size</TableHead>
                  <TableHead>Remote key</TableHead>
                  <TableHead>Error</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {runs.map((run) => (
                  <TableRow key={run.id}>
                    <TableCell>
                      <StatusPill status={run.status} />
                    </TableCell>
                    <TableCell className="text-muted-foreground text-sm">
                      {formatDate(run.started_at)}
                    </TableCell>
                    <TableCell className="text-muted-foreground text-sm">
                      {formatDate(run.finished_at)}
                    </TableCell>
                    <TableCell className="text-sm">
                      {formatBytes(run.size_bytes)}
                    </TableCell>
                    <TableCell className="max-w-[200px] truncate font-mono text-xs">
                      {run.remote_key ?? "—"}
                    </TableCell>
                    <TableCell className="text-destructive max-w-[240px] truncate text-xs">
                      {run.error ?? "—"}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function StatusItem({
  label,
  value,
}: {
  label: string;
  value: React.ReactNode;
}) {
  return (
    <div>
      <p className="text-muted-foreground text-xs font-medium">{label}</p>
      <div className="mt-1">
        {typeof value === "string" ? <p>{value}</p> : value}
      </div>
    </div>
  );
}

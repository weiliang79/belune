import { toast } from "sonner";
import type { ColumnDef } from "@tanstack/react-table";
import { Archive, ArchiveRestore, Cloud, HistoryIcon } from "lucide-react";
import {
  useBackupRuns,
  useBackupStatus,
  useTriggerBackup,
  useTestBackupRemote,
} from "@/lib/hooks/use-backups";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { BlobLogViewer } from "@/components/logs/blob-log-viewer";
import { StatusPill } from "@/components/ui/status-pill";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { DataTable } from "@/components/ui/data-table";
import { CopyButton } from "@/lib/components/copy-button";
import {
  formatBytes,
  formatDateTime,
  formatDateTimeShort,
} from "@/lib/utils/format";
import type { BackupRun, BackupStatus } from "@/lib/types";

/** "YYYY-MM-DD HH:mm:ss" for table cells (null-safe). */
function fmtTableDate(iso: string | null) {
  return iso ? formatDateTime(iso) : "—";
}

/** "DD MMM YYYY, HH:mm" for summary strips (null-safe). */
function fmtSummaryDate(iso: string | null) {
  return iso ? formatDateTimeShort(iso) : "—";
}

// The install path the restore script and archives live under. Kept in sync
// with BELUNE_DIR (default /opt/belune); shown in the restore instructions so an
// operator has the exact command without guessing paths.
const BELUNE_DIR = "/opt/belune";

// buildRestoreCommand constructs the exact restore.sh invocation for a run.
// Archives are named after their remote key's basename (backup.sh writes the
// same filename locally and remotely). Encrypted (.age) archives need the age
// identity file, so we append a placeholder to remind the operator.
function buildRestoreCommand(run: BackupRun): string | null {
  if (!run.remote_key) return null;
  const filename = run.remote_key.split("/").pop() ?? run.remote_key;
  const archive = `${BELUNE_DIR}/backups/${filename}`;
  const identity = filename.endsWith(".age") ? " <age-identity-file>" : "";
  return `bash ${BELUNE_DIR}/scripts/restore.sh ${archive}${identity}`;
}

const backupColumns: ColumnDef<BackupRun>[] = [
  {
    id: "status",
    header: "Status",
    cell: ({ row: { original: run } }) => <StatusPill status={run.status} />,
  },
  {
    id: "started_at",
    header: "Started",
    accessorKey: "started_at",
    meta: { className: "text-muted-foreground text-sm" },
    cell: ({ row: { original: run } }) => fmtTableDate(run.started_at),
  },
  {
    id: "finished_at",
    header: "Finished",
    accessorKey: "finished_at",
    meta: { className: "text-muted-foreground text-sm" },
    cell: ({ row: { original: run } }) => fmtTableDate(run.finished_at),
  },
  {
    id: "size_bytes",
    header: "Size",
    accessorKey: "size_bytes",
    meta: { className: "text-sm" },
    cell: ({ row: { original: run } }) =>
      run.size_bytes ? formatBytes(run.size_bytes) : "—",
  },
  {
    id: "remote_key",
    header: "Remote key",
    meta: { className: "max-w-[200px] truncate font-mono text-xs" },
    cell: ({ row: { original: run } }) => run.remote_key ?? "—",
  },
];

// SystemBackupsPanel renders the control-plane (platform Postgres + Caddy certs)
// disaster-recovery backup status and run history. Distinct from per-database
// backups, which live on each project's Backups tab.
export function SystemBackupsPanel() {
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
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="flex items-center gap-2">
                <Archive aria-hidden="true" className="size-4" />
                System Backup
              </CardTitle>
              <CardDescription>
                Platform database and configuration (control-plane) backups.
              </CardDescription>
            </div>
            <div className="flex items-center gap-2">
              {status && (
                <Badge variant={lastRunFailed ? "destructive" : "default"}>
                  {lastRunFailed ? "Last run failed" : "Healthy"}
                </Badge>
              )}
              <Button
                onClick={handleTrigger}
                disabled={trigger.isPending || isRunning}
                size="sm"
              >
                {isRunning
                  ? "Backup in progress…"
                  : trigger.isPending
                    ? "Queueing…"
                    : "Run Backup Now"}
              </Button>
            </div>
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
                value={fmtSummaryDate(status.last_succeeded_at)}
              />
              <StatusItem
                label="Last attempted"
                value={fmtSummaryDate(status.last_attempted_at)}
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

      <RemoteStorageCard status={status} loading={statusLoading} />

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <HistoryIcon aria-hidden="true" className="size-4" />
            Recent runs
          </CardTitle>
        </CardHeader>
        <CardContent>
          <DataTable
            columns={backupColumns}
            data={runs ?? []}
            isLoading={runsLoading}
            getRowId={(r) => r.id}
            enableSorting
            emptyMessage={
              'No backup runs yet. Click "Run Backup Now" to start one.'
            }
            renderDetailPanel={({ row }) => {
              const run = row.original;
              const { log, error } = run;
              const text = log?.trim() || error?.trim();
              const restoreCmd =
                run.status === "succeeded" ? buildRestoreCommand(run) : null;
              return (
                <div className="space-y-3">
                  {restoreCmd && (
                    <div>
                      <p className="text-muted-foreground mb-1 text-xs font-medium">
                        Restore from this backup (run on the server host)
                      </p>
                      <div className="bg-elev flex items-center gap-2 rounded p-2">
                        <code className="flex-1 font-mono text-xs break-all">
                          {restoreCmd}
                        </code>
                        <CopyButton value={restoreCmd} />
                      </div>
                    </div>
                  )}
                  {text ? (
                    <BlobLogViewer blob={text} heightClass="max-h-96" />
                  ) : (
                    <span className="text-text-faint text-xs">
                      No log captured for this run.
                    </span>
                  )}
                </div>
              );
            }}
          />
        </CardContent>
      </Card>

      <RestoreHelpCard remoteEnabled={!!status?.remote_enabled} />
    </div>
  );
}

// RestoreHelpCard documents how to restore a control-plane backup. There is no
// in-app restore action by design: a restore drops and recreates the platform
// database and overwrites .env, so it must be run from the host shell (it also
// needs to work when the API itself is down). This card makes the CLI path
// discoverable instead of hidden in a runbook.
function RestoreHelpCard({ remoteEnabled }: { remoteEnabled: boolean }) {
  const example = `bash ${BELUNE_DIR}/scripts/restore.sh ${BELUNE_DIR}/backups/belune-backup-<timestamp>.tar.gz`;
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <ArchiveRestore aria-hidden="true" className="size-4" />
          Restoring a backup
        </CardTitle>
        <p className="text-muted-foreground text-sm">
          Restore is a host operation, not an in-app action — it drops and
          rebuilds the platform database, so it runs from the server shell and
          works even when this dashboard is down.
        </p>
      </CardHeader>
      <CardContent className="space-y-4 text-sm">
        <ol className="text-muted-foreground list-decimal space-y-2 pl-5">
          <li>
            SSH into the server host (where the platform runs, at{" "}
            <code className="font-mono text-xs">{BELUNE_DIR}</code>).
          </li>
          <li>
            Locate the archive:{" "}
            {remoteEnabled ? (
              <>
                either the local copy under{" "}
                <code className="font-mono text-xs">{BELUNE_DIR}/backups/</code>{" "}
                or download the object from your remote bucket (the run's{" "}
                <span className="font-medium">Remote key</span> above).
              </>
            ) : (
              <>
                under{" "}
                <code className="font-mono text-xs">{BELUNE_DIR}/backups/</code>
                .
              </>
            )}
          </li>
          <li>
            Preview first with{" "}
            <code className="font-mono text-xs">--dry-run</code>, then run the
            restore. Expand any successful run above and use{" "}
            <span className="font-medium">Copy restore command</span> to get the
            exact command.
          </li>
        </ol>

        <div>
          <p className="text-muted-foreground mb-1 text-xs font-medium">
            Verify an archive (safe, makes no changes)
          </p>
          <div className="bg-elev flex items-center gap-2 rounded p-2">
            <code className="flex-1 font-mono text-xs break-all">
              {example} --dry-run
            </code>
            <CopyButton value={`${example} --dry-run`} />
          </div>
        </div>

        <p className="text-text-faint text-xs">
          Encrypted (<code className="font-mono">.age</code>) archives require
          the age identity file as a second argument. The restore prompts for
          confirmation and writes a{" "}
          <code className="font-mono">pre-restore-&lt;timestamp&gt;.sql</code>{" "}
          snapshot before overwriting, so a bad restore can be rolled back. Full
          procedure:{" "}
          <code className="font-mono text-xs">
            docs/runbooks/disaster-recovery.md
          </code>
          .
        </p>
      </CardContent>
    </Card>
  );
}

// RemoteStorageCard shows the off-host (S3-compatible) destination for
// control-plane backups. It is read-only: this destination is configured via
// BACKUP_S3_* env vars in .env (kept out of the database so a total-loss
// restore can still find where its own backups live). A "Test connection"
// button verifies reachability without mutating anything.
function RemoteStorageCard({
  status,
  loading,
}: {
  status: BackupStatus | undefined;
  loading: boolean;
}) {
  const test = useTestBackupRemote();
  const remote = status?.remote ?? null;

  const handleTest = () => {
    toast.promise(test.mutateAsync(), {
      loading: "Testing connection…",
      success: "Connection OK — bucket reachable",
      error: (err) => err.message ?? "Connection failed",
    });
  };

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between gap-2">
          <div>
            <CardTitle className="flex items-center gap-2">
              <Cloud aria-hidden="true" className="size-4" />
              Remote storage
            </CardTitle>
            <CardDescription>
              Off-host destination for control-plane backups.
            </CardDescription>
          </div>
          {remote && (
            <Button
              onClick={handleTest}
              disabled={test.isPending}
              variant="outline"
              size="sm"
            >
              {test.isPending ? "Testing…" : "Test connection"}
            </Button>
          )}
        </div>
      </CardHeader>
      <CardContent>
        {loading ? (
          <div className="space-y-2">
            <Skeleton className="h-4 w-48" />
            <Skeleton className="h-4 w-40" />
          </div>
        ) : remote ? (
          <div className="grid grid-cols-2 gap-4 text-sm md:grid-cols-4">
            <StatusItem
              label="Endpoint"
              value={remote.endpoint || "AWS S3 (regional)"}
            />
            <StatusItem label="Region" value={remote.region || "—"} />
            <StatusItem label="Bucket" value={remote.bucket || "—"} />
            <StatusItem label="Prefix" value={remote.prefix || "—"} />
          </div>
        ) : (
          <p className="text-muted-foreground text-sm">
            Remote storage is disabled — control-plane backups are kept on-host
            only.
          </p>
        )}
        <p className="text-text-faint mt-4 text-xs">
          Configured via{" "}
          <code className="font-mono">BACKUP_REMOTE_ENABLED</code> and{" "}
          <code className="font-mono">BACKUP_S3_*</code> in{" "}
          <code className="font-mono">.env</code> on the server (restart the API
          to apply). Kept out of the database so a full restore can still locate
          its backups. Per-database backups use project destinations instead.
        </p>
      </CardContent>
    </Card>
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

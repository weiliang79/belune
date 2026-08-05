import { useState } from "react";
import { toast } from "sonner";
import { useQueryClient } from "@tanstack/react-query";
import type { ColumnDef } from "@tanstack/react-table";
import {
  Archive,
  ArchiveRestore,
  Clock,
  Cloud,
  DatabaseBackup,
  HistoryIcon,
  Settings2,
  TimerIcon,
  type LucideIcon,
} from "lucide-react";
import {
  useBackupRuns,
  useBackupStatus,
  useTriggerBackup,
  useTestBackupRemote,
  useUpdateBackupRemote,
} from "@/lib/hooks/use-backups";
import { useSettings, useUpdateSettings } from "@/lib/hooks/use-settings";
import { queryKeys } from "@/lib/hooks/query-keys";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { BlobLogViewer } from "@/components/logs/blob-log-viewer";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { StatusPill } from "@/components/ui/status-pill";
import { Switch } from "@/components/ui/switch";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { DataTable } from "@/components/ui/data-table";
import { CopyButton } from "@/lib/components/copy-button";
import {
  formatBytes,
  formatDateTime,
  formatDateTimeShort,
} from "@/lib/utils/format";
import type { BackupRemoteConfig, BackupRun, BackupStatus } from "@/lib/types";

// Matches config.DefaultControlPlaneBackupSchedule on the API — daily at
// 02:00, the cadence the retired belune-backup.timer used to run at.
const DEFAULT_BACKUP_SCHEDULE = "0 2 * * *";

const RUNS_PAGE_SIZE = 10;

// Matches the bounds validateRetentionSetting enforces server-side (see
// handler/settings.go) — client-side just gives immediate feedback.
const RETAIN_DAYS_MAX = 3650;
const RETAIN_COUNT_MAX = 1000;

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
];

// SystemBackupsPanel renders the control-plane (platform Postgres + Caddy certs)
// disaster-recovery backup status and run history. Distinct from per-database
// backups, which live on each project's Backups tab.
export function SystemBackupsPanel() {
  const { data: status, isLoading: statusLoading } = useBackupStatus();
  // Independent of the table's page, so the header badge/button always
  // reflect the actual latest run regardless of which page is showing.
  const { data: latestRuns } = useBackupRuns({ limit: 1, offset: 0 });
  const [runsOffset, setRunsOffset] = useState(0);
  const { data: runs, isLoading: runsLoading } = useBackupRuns({
    limit: RUNS_PAGE_SIZE,
    offset: runsOffset,
  });
  const trigger = useTriggerBackup();
  const [configureOpen, setConfigureOpen] = useState(false);
  // Shared with BackupScheduleSection inside the Sheet (same query, cached) —
  // read here too so the summary grid can show the schedule without opening it.
  const { data: settings } = useSettings();
  const scheduleEnabled =
    settings?.find((s) => s.key === "control_plane_backup_enabled")?.value !==
    "false";
  const scheduleCron =
    settings?.find((s) => s.key === "control_plane_backup_schedule")?.value ||
    DEFAULT_BACKUP_SCHEDULE;

  const isRunning = latestRuns?.[0]?.status === "running";
  const lastRunFailed =
    latestRuns?.[0]?.status === "failed" || !!status?.last_error;

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
              <Button
                variant="outline"
                size="sm"
                onClick={() => setConfigureOpen(true)}
              >
                <Settings2 aria-hidden="true" className="mr-1.5 size-4" />
                Configure
              </Button>
              <Button
                onClick={handleTrigger}
                disabled={trigger.isPending || isRunning}
                size="sm"
              >
                <DatabaseBackup aria-hidden="true" className="mr-1.5 size-4" />
                {isRunning
                  ? "Backup in progress…"
                  : trigger.isPending
                    ? "Queueing…"
                    : "Backup Now"}
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
            <div className="grid grid-cols-2 gap-4 text-sm md:grid-cols-3">
              <StatusItem
                label="Status"
                value={
                  <Badge variant={lastRunFailed ? "destructive" : "default"}>
                    {lastRunFailed ? "Last run failed" : "Healthy"}
                  </Badge>
                }
              />
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
                label="Schedule"
                value={
                  scheduleEnabled ? (
                    <p className="font-mono">{scheduleCron}</p>
                  ) : (
                    <Badge variant="secondary">Disabled</Badge>
                  )
                }
              />
              <StatusItem
                label="Retention"
                value={`${status.retention.count} backups / ${status.retention.days} days`}
              />
              {status.last_error && (
                <div className="border-destructive/30 bg-destructive/10 col-span-2 mt-1 rounded-lg border p-3 md:col-span-3">
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

      <ConfigureBackupsSheet
        open={configureOpen}
        onOpenChange={setConfigureOpen}
        status={status}
        loading={statusLoading}
      />

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <HistoryIcon aria-hidden="true" className="size-4" />
            Recent Runs
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
              runsOffset > 0
                ? "No more runs."
                : 'No backup runs yet. Click "Run Backup Now" to start one.'
            }
            pagination={{
              mode: "manual",
              offset: runsOffset,
              pageSize: RUNS_PAGE_SIZE,
              hasMore: (runs?.length ?? 0) === RUNS_PAGE_SIZE,
              onOffsetChange: setRunsOffset,
            }}
            renderDetailPanel={({ row }) => {
              const run = row.original;
              const { log, error } = run;
              const text = log?.trim() || error?.trim();
              const restoreCmd =
                run.status === "succeeded" ? buildRestoreCommand(run) : null;
              return (
                <div className="space-y-3">
                  <div>
                    <p className="text-muted-foreground mb-1 text-xs font-medium">
                      Remote key
                    </p>
                    <div className="bg-elev flex items-center gap-2 rounded p-2">
                      <code className="flex-1 font-mono text-xs break-all">
                        {run.remote_key ?? "-"}
                      </code>
                      {run.remote_key && <CopyButton value={run.remote_key} />}
                    </div>
                  </div>
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

// SectionHeader is the shared title row for both sections of
// BackupScheduleAndRemoteCard: an icon + title + one-line description on the
// left, the section's on/off switch on the same line on the right.
function SectionHeader({
  icon: Icon,
  title,
  description,
  checked,
  onCheckedChange,
  disabled,
  switchLabel,
}: {
  icon: LucideIcon;
  title: string;
  description: string;
  checked: boolean;
  onCheckedChange: (next: boolean) => void;
  disabled?: boolean;
  switchLabel: string;
}) {
  return (
    <div className="flex items-center justify-between gap-2">
      <div>
        <div className="flex items-center gap-2 text-base leading-snug font-medium">
          <Icon aria-hidden="true" className="size-4" />
          {title}
        </div>
        <p className="text-muted-foreground mt-0.5 text-sm">{description}</p>
      </div>
      <Switch
        checked={checked}
        disabled={disabled}
        onCheckedChange={onCheckedChange}
        aria-label={switchLabel}
      />
    </div>
  );
}

// BackupScheduleSection configures the in-app cron sweep that replaced
// belune-backup.timer — the worker checks this schedule once a minute
// (backup_schedule_task.go) and enqueues the same "Run Backup Now" task a
// manual click would. Backed by the generic settings table (no dedicated
// endpoint), so the toggle applies immediately but the schedule text needs an
// explicit Save (it's free text, not a click).
function BackupScheduleSection() {
  const { data: settings, isLoading } = useSettings();
  const updateSettings = useUpdateSettings();

  const enabled =
    settings?.find((s) => s.key === "control_plane_backup_enabled")?.value !==
    "false";
  const storedSchedule = settings?.find(
    (s) => s.key === "control_plane_backup_schedule",
  )?.value;

  // null = no unsaved edit yet, so the field tracks the server value; once the
  // operator types, draft takes over until Save resets it to null.
  const [draft, setDraft] = useState<string | null>(null);
  const schedule = draft ?? storedSchedule ?? DEFAULT_BACKUP_SCHEDULE;

  const handleToggle = (next: boolean) => {
    toast.promise(
      updateSettings.mutateAsync([
        { key: "control_plane_backup_enabled", value: next ? "true" : "false" },
      ]),
      {
        loading: "Saving…",
        success: `Scheduled backups ${next ? "enabled" : "disabled"}`,
        error: (err) => err.message ?? "Failed to save",
      },
    );
  };

  const handleSave = () => {
    toast.promise(
      updateSettings
        .mutateAsync([
          { key: "control_plane_backup_schedule", value: schedule.trim() },
        ])
        .then(() => setDraft(null)),
      {
        loading: "Saving…",
        success: "Schedule updated",
        error: (err) => err.message ?? "Invalid cron schedule",
      },
    );
  };

  return (
    <div className="space-y-3">
      <SectionHeader
        icon={Clock}
        title="Schedule"
        description="Automatic control-plane backups, on top of any manual runs."
        checked={enabled}
        disabled={isLoading || updateSettings.isPending}
        onCheckedChange={handleToggle}
        switchLabel="Scheduled backups"
      />
      {enabled && (
        <div className="space-y-2">
          <Label htmlFor="backup-schedule" className="text-sm">
            Cron schedule
          </Label>
          <div className="flex items-center gap-2">
            <Input
              id="backup-schedule"
              value={schedule}
              onChange={(e) => setDraft(e.target.value)}
              disabled={isLoading}
              className="max-w-xs font-mono text-sm"
              placeholder={DEFAULT_BACKUP_SCHEDULE}
            />
            <Button
              size="sm"
              variant="outline"
              onClick={handleSave}
              disabled={draft === null || updateSettings.isPending}
            >
              Save
            </Button>
          </div>
          <p className="text-text-faint text-xs">
            Standard 5-field cron (minute hour day-of-month month weekday),
            evaluated in the server's local time. Default: daily at 02:00 (
            <code className="font-mono">{DEFAULT_BACKUP_SCHEDULE}</code>).
          </p>
        </div>
      )}
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
          Restoring a Backup
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
                or download the object from your remote bucket (expand a run
                above for its <span className="font-medium">Remote key</span>).
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

// ConfigureBackupsSheet holds "when" (Schedule), "where" (Remote Storage), and
// "how long" (Retention) for control-plane backups — everything that used to
// sit inline on the page, now behind a Configure button so the System Backup
// card's read-only summary is what stays always-visible. Sections split by a
// Separator instead of separate Cards. Remote Storage's and Retention's forms
// are split out so their useState initializers can read real data on mount
// instead of racing the query — the "loaded" key forces exactly one remount
// when the query transitions from pending to loaded, not on every refetch
// afterward (a save's own refetch should not blow away what the operator is
// mid-typing).
function ConfigureBackupsSheet({
  open,
  onOpenChange,
  status,
  loading,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  status: BackupStatus | undefined;
  loading: boolean;
}) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="sm:max-w-lg">
        <SheetHeader>
          <SheetTitle>Configure Backups</SheetTitle>
          <SheetDescription>
            Schedule, remote storage, and retention for control-plane backups.
          </SheetDescription>
        </SheetHeader>
        <div className="space-y-6">
          <BackupScheduleSection />
          <Separator />
          {loading ? (
            <div className="space-y-2">
              <Skeleton className="h-4 w-48" />
              <Skeleton className="h-4 w-40" />
            </div>
          ) : (
            <RemoteStorageSection
              key="remote-loaded"
              remote={status?.remote ?? null}
            />
          )}
          <Separator />
          {loading ? (
            <div className="space-y-2">
              <Skeleton className="h-4 w-48" />
              <Skeleton className="h-4 w-40" />
            </div>
          ) : (
            <RetentionSection
              key="retention-loaded"
              retention={status?.retention ?? null}
            />
          )}
        </div>
      </SheetContent>
    </Sheet>
  );
}

// RetentionSection edits how long control-plane backups are kept, both
// locally and off-host — dashboard-editable settings
// (control_plane_backup_retain_days/count), falling back to the .env
// BACKUP_RETAIN_DAYS/COUNT values when unset (same shape as the schedule
// above). retention is the server-resolved EFFECTIVE value (settings table or
// .env fallback, whichever applies) — used as the initial draft so the form
// always starts from what's actually in effect, not a guessed default.
function RetentionSection({
  retention,
}: {
  retention: { days: number; count: number } | null;
}) {
  const updateSettings = useUpdateSettings();
  const qc = useQueryClient();

  const [days, setDays] = useState(String(retention?.days ?? 30));
  const [count, setCount] = useState(String(retention?.count ?? 14));

  const handleSave = () => {
    const daysNum = Number(days);
    const countNum = Number(count);
    if (
      !Number.isInteger(daysNum) ||
      daysNum < 1 ||
      daysNum > RETAIN_DAYS_MAX
    ) {
      toast.error(
        `Days must be a whole number between 1 and ${RETAIN_DAYS_MAX}`,
      );
      return;
    }
    if (
      !Number.isInteger(countNum) ||
      countNum < 1 ||
      countNum > RETAIN_COUNT_MAX
    ) {
      toast.error(
        `Count must be a whole number between 1 and ${RETAIN_COUNT_MAX}`,
      );
      return;
    }
    toast.promise(
      updateSettings
        .mutateAsync([
          {
            key: "control_plane_backup_retain_days",
            value: String(daysNum),
          },
          {
            key: "control_plane_backup_retain_count",
            value: String(countNum),
          },
        ])
        // Retention's effective value is surfaced via /backups/status (the
        // summary card), a different query than the generic settings list —
        // useUpdateSettings only invalidates the latter, so nudge this one too.
        .then(() =>
          qc.invalidateQueries({ queryKey: queryKeys.backups.status }),
        ),
      {
        loading: "Saving…",
        success: "Retention updated",
        error: (err) => err.message ?? "Failed to save",
      },
    );
  };

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2 text-base leading-snug font-medium">
        <TimerIcon aria-hidden="true" className="size-4" />
        Retention
      </div>
      <p className="text-muted-foreground text-sm">
        How long control-plane backups are kept, locally and off-host.
      </p>
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1.5">
          <Label htmlFor="retain-days">Keep for (days)</Label>
          <Input
            id="retain-days"
            type="number"
            min={1}
            max={RETAIN_DAYS_MAX}
            value={days}
            onChange={(e) => setDays(e.target.value)}
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="retain-count">Keep at least (count)</Label>
          <Input
            id="retain-count"
            type="number"
            min={1}
            max={RETAIN_COUNT_MAX}
            value={count}
            onChange={(e) => setCount(e.target.value)}
          />
        </div>
      </div>
      <div className="flex justify-end">
        <Button
          size="sm"
          variant="outline"
          onClick={handleSave}
          disabled={updateSettings.isPending}
        >
          {updateSettings.isPending ? "Saving…" : "Save"}
        </Button>
      </div>
      <p className="text-text-faint text-xs">
        The most recent backups (count) are always kept regardless of age; older
        ones are deleted once they pass the day limit. Applies to local archives
        and the remote bucket alike.
      </p>
    </div>
  );
}

// RemoteStorageSection configures the off-host (S3-compatible) destination
// for control-plane backups: saved to backup-remote.env on the server,
// separate from .env (kept out of the database so a total-loss restore can
// still find where its own backups live), read fresh on every backup — no
// restart needed.
function RemoteStorageSection({ remote }: { remote: BackupRemoteConfig | null }) {
  const update = useUpdateBackupRemote();
  const test = useTestBackupRemote();

  const [enabled, setEnabled] = useState(!!remote);
  const [endpoint, setEndpoint] = useState(remote?.endpoint ?? "");
  const [region, setRegion] = useState(remote?.region ?? "us-east-1");
  const [bucket, setBucket] = useState(remote?.bucket ?? "");
  const [prefix, setPrefix] = useState(remote?.prefix ?? "belune/");
  const [useSSL, setUseSSL] = useState(remote?.use_ssl ?? true);
  const [accessKey, setAccessKey] = useState("");
  const [secretKey, setSecretKey] = useState("");

  const buildData = (enabledOverride: boolean = enabled) => ({
    enabled: enabledOverride,
    endpoint: endpoint.trim(),
    region: region.trim() || "us-east-1",
    bucket: bucket.trim(),
    prefix: prefix.trim(),
    use_ssl: useSSL,
    // Blank preserves the stored secret — never redisplayed once saved.
    access_key: accessKey || undefined,
    secret_key: secretKey || undefined,
  });

  const handleSave = () => {
    if (!bucket.trim()) {
      toast.error("Bucket is required to enable remote storage");
      return;
    }
    toast.promise(
      update.mutateAsync(buildData()).then(() => {
        setAccessKey("");
        setSecretKey("");
      }),
      {
        loading: "Saving…",
        success: "Remote storage saved",
        error: (err) => err.message ?? "Failed to save",
      },
    );
  };

  const handleTest = () => {
    toast.promise(test.mutateAsync(), {
      loading: "Testing connection…",
      success: "Connection OK — bucket reachable",
      error: (err) => err.message ?? "Connection failed",
    });
  };

  // Turning it off is a complete, valid state on its own (no fields needed),
  // so it saves immediately. Turning it on just reveals the form — that
  // needs a bucket and credentials, so it stays a draft until Save.
  const handleToggle = (next: boolean) => {
    setEnabled(next);
    if (next) return;
    toast.promise(
      update.mutateAsync(buildData(false)).then(() => {
        setAccessKey("");
        setSecretKey("");
      }),
      {
        loading: "Disabling…",
        success: "Remote storage disabled",
        error: (err) => err.message ?? "Failed to save",
      },
    );
  };

  return (
    <div className="space-y-4">
      <SectionHeader
        icon={Cloud}
        title="Remote Storage"
        description="Off-host destination for control-plane backups."
        checked={enabled}
        disabled={update.isPending}
        onCheckedChange={handleToggle}
        switchLabel="Remote storage enabled"
      />

      {enabled && (
        <>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="remote-bucket">Bucket</Label>
              <Input
                id="remote-bucket"
                value={bucket}
                onChange={(e) => setBucket(e.target.value)}
                placeholder="my-belune-backups"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="remote-region">Region</Label>
              <Input
                id="remote-region"
                value={region}
                onChange={(e) => setRegion(e.target.value)}
                placeholder="us-east-1"
              />
            </div>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="remote-endpoint">Endpoint</Label>
            <Input
              id="remote-endpoint"
              value={endpoint}
              onChange={(e) => setEndpoint(e.target.value)}
              placeholder="Empty = AWS S3. Set for MinIO/R2/B2/Wasabi (host:port, no scheme)"
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="remote-prefix">Prefix</Label>
            <Input
              id="remote-prefix"
              value={prefix}
              onChange={(e) => setPrefix(e.target.value)}
              placeholder="belune/"
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="remote-access-key">Access key</Label>
              <Input
                id="remote-access-key"
                value={accessKey}
                onChange={(e) => setAccessKey(e.target.value)}
                placeholder={remote ? "•••• (unchanged)" : ""}
                className="font-mono"
                autoComplete="off"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="remote-secret-key">Secret key</Label>
              <Input
                id="remote-secret-key"
                type="password"
                value={secretKey}
                onChange={(e) => setSecretKey(e.target.value)}
                placeholder={remote ? "•••• (unchanged)" : ""}
                className="font-mono"
                autoComplete="off"
              />
            </div>
          </div>

          <label className="flex items-center gap-2 text-sm">
            <Checkbox checked={useSSL} onCheckedChange={setUseSSL} />
            Use SSL (HTTPS)
          </label>

          <div className="flex items-center justify-between gap-2 pt-1">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={handleTest}
              disabled={test.isPending}
            >
              {test.isPending ? "Testing…" : "Test connection"}
            </Button>
            <Button size="sm" onClick={handleSave} disabled={update.isPending}>
              {update.isPending ? "Saving…" : "Save"}
            </Button>
          </div>

          <p className="text-text-faint text-xs">
            Saved to <code className="font-mono">backup-remote.env</code> on
            the server, separate from{" "}
            <code className="font-mono">.env</code> — takes effect on the
            next backup, no restart needed. Kept out of the database so a
            full restore can still locate its backups. Per-database backups
            use project destinations instead.
          </p>
        </>
      )}
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

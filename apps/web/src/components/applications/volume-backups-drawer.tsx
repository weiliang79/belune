import { useState } from "react";
import { toast } from "sonner";
import {
  ClockIcon,
  CloudIcon,
  DatabaseBackupIcon,
  RotateCcwIcon,
  ScrollTextIcon,
} from "lucide-react";
import type { ApplicationVolume } from "@/lib/types";
import { useBackupDestinations } from "@/lib/hooks/use-backup-destinations";
import {
  useVolumeBackupConfigs,
  useVolumeBackups,
  useVolumeRestores,
  useCreateVolumeBackupConfig,
  useUpdateVolumeBackupConfig,
  useDeleteVolumeBackupConfig,
  useRunVolumeBackup,
  useRestoreVolumeBackup,
} from "@/lib/hooks/use-volume-backups";
import type { SaveVolumeBackupConfig } from "@/lib/api/volume-backups";
import { formatBytes, formatRelativeTime } from "@/lib/utils/format";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
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
  volume: ApplicationVolume;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

// "manual" is a sentinel for an empty schedule (back up on demand only); Radix
// Select cannot use an empty string as an item value.
const SCHEDULE_PRESETS: { value: string; label: string }[] = [
  { value: "manual", label: "Manual only (no schedule)" },
  { value: "0 * * * *", label: "Every hour" },
  { value: "0 0 * * *", label: "Every day at midnight" },
  { value: "0 0 * * 0", label: "Every week (Sunday midnight)" },
  { value: "0 0 1 * *", label: "Every month (1st, midnight)" },
];

function statusVariant(status: string): "default" | "secondary" | "destructive" {
  if (status === "succeeded") return "default";
  if (status === "failed") return "destructive";
  return "secondary";
}

export function VolumeBackupsDrawer({
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
  const { data: restores } = useVolumeRestores(
    projectId,
    applicationId,
    volume.id,
    open,
  );
  const config = configs?.[0];

  const createConfig = useCreateVolumeBackupConfig(
    projectId,
    applicationId,
    volume.id,
  );
  const updateConfig = useUpdateVolumeBackupConfig(
    projectId,
    applicationId,
    volume.id,
    config?.id ?? "",
  );
  const deleteConfig = useDeleteVolumeBackupConfig(
    projectId,
    applicationId,
    volume.id,
  );
  const runBackup = useRunVolumeBackup(projectId, applicationId, volume.id);
  const restore = useRestoreVolumeBackup(projectId, applicationId, volume.id);

  const [destinationId, setDestinationId] = useState("");
  const [prefix, setPrefix] = useState("");
  const [schedule, setSchedule] = useState("");
  // Sticky "Custom…" selection: the cron may equal a preset yet the user still
  // wants the free-form field, so track custom mode explicitly.
  const [customMode, setCustomMode] = useState(false);
  const [keepLatest, setKeepLatest] = useState("");
  const [quiesce, setQuiesce] = useState(false);
  const [enabled, setEnabled] = useState(true);
  const [logView, setLogView] = useState<{ title: string; log: string } | null>(
    null,
  );
  const [restoreTarget, setRestoreTarget] = useState<string | null>(null);

  // Seed the form from the loaded config once (React "adjust state during
  // render" pattern), keyed on config id so a background refetch doesn't clobber
  // the user's in-progress edits.
  const [seededId, setSeededId] = useState<string | null>(null);
  if (config && config.id !== seededId) {
    setSeededId(config.id);
    setDestinationId(config.destination_id);
    setPrefix(config.prefix ?? "");
    const sched = config.schedule ?? "";
    setSchedule(sched);
    setCustomMode(
      sched !== "" && !SCHEDULE_PRESETS.some((p) => p.value === sched),
    );
    setKeepLatest(config.keep_latest != null ? String(config.keep_latest) : "");
    setQuiesce(config.quiesce);
    setEnabled(config.enabled);
  }

  const presetValue = customMode
    ? "custom"
    : schedule === ""
      ? "manual"
      : (SCHEDULE_PRESETS.find((p) => p.value === schedule)?.value ?? "custom");

  const buildPayload = (): SaveVolumeBackupConfig => ({
    destination_id: destinationId,
    prefix: prefix.trim(),
    schedule: schedule.trim(),
    keep_latest: keepLatest.trim() === "" ? null : Number(keepLatest),
    quiesce,
    enabled,
  });

  const saveConfig = () => {
    if (!destinationId) return;
    const mutation = config
      ? updateConfig.mutateAsync(buildPayload())
      : createConfig.mutateAsync(buildPayload());
    toast.promise(mutation, {
      loading: config ? "Saving changes..." : "Saving backup config...",
      success: config ? "Backup config updated" : "Backup configured",
      error: (err) => err.message,
    });
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
      restore.mutateAsync(restoreTarget).then(() => setRestoreTarget(null)),
      {
        loading: "Starting restore...",
        success: "Restore started — the app will stop, restore, and restart",
        error: (err) => err.message,
      },
    );
  };

  const saving = createConfig.isPending || updateConfig.isPending;

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="sm:max-w-lg">
        <SheetHeader>
          <SheetTitle>Backups — {volume.name}</SheetTitle>
          <SheetDescription>
            Snapshot <span className="font-mono">{volume.mount_path}</span> to an
            S3-compatible destination, on a schedule or on demand, and restore it.
          </SheetDescription>
        </SheetHeader>

        <div className="space-y-4">
          {/* Config form */}
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

            <div className="space-y-1.5">
              <Label htmlFor="vol-prefix">Prefix</Label>
              <Input
                id="vol-prefix"
                value={prefix}
                onChange={(e) => setPrefix(e.target.value)}
                placeholder="/my-app"
                className="font-mono"
              />
              <p className="text-muted-foreground text-xs">
                Optional path inside the bucket to store backups under.
              </p>
            </div>

            <div className="space-y-1.5">
              <Label>Schedule</Label>
              <Select
                value={presetValue}
                onValueChange={(v) => {
                  if (v === "manual") {
                    setCustomMode(false);
                    setSchedule("");
                  } else if (v === "custom") {
                    setCustomMode(true);
                  } else if (v) {
                    setCustomMode(false);
                    setSchedule(v);
                  }
                }}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select a schedule" />
                </SelectTrigger>
                <SelectContent>
                  {SCHEDULE_PRESETS.map((p) => (
                    <SelectItem key={p.value} value={p.value} icon={<ClockIcon />}>
                      {p.label}
                    </SelectItem>
                  ))}
                  <SelectItem value="custom" icon={<ClockIcon />}>
                    Custom…
                  </SelectItem>
                </SelectContent>
              </Select>
              {presetValue === "custom" && (
                <Input
                  value={schedule}
                  onChange={(e) => setSchedule(e.target.value)}
                  placeholder="0 0 * * *"
                  className="font-mono"
                  aria-label="Cron expression"
                />
              )}
              <p className="text-muted-foreground text-xs">
                Standard 5-field cron expression. Manual only = back up on demand.
              </p>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="vol-keep">Keep latest</Label>
              <Input
                id="vol-keep"
                type="number"
                min={1}
                value={keepLatest}
                onChange={(e) => setKeepLatest(e.target.value)}
                placeholder="Keeps all if empty"
              />
              <p className="text-muted-foreground text-xs">
                Optional. Only keep the latest N backups in the destination.
              </p>
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

            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                className="size-4"
                checked={enabled}
                onChange={(e) => setEnabled(e.target.checked)}
              />
              Enabled (run on schedule)
            </label>

            <div className="flex flex-wrap items-center gap-2">
              <Button onClick={saveConfig} disabled={!destinationId || saving}>
                {config ? "Save changes" : "Save backup config"}
              </Button>
              {config && (
                <>
                  <Button
                    variant="outline"
                    onClick={backUpNow}
                    disabled={runBackup.isPending}
                  >
                    <DatabaseBackupIcon aria-hidden="true" className="size-4" />
                    Back up now
                  </Button>
                  <Button variant="outline" onClick={removeConfig}>
                    Remove
                  </Button>
                  {config.last_run_at && (
                    <span className="text-text-faint ml-auto text-xs">
                      Last run {formatRelativeTime(config.last_run_at)}
                    </span>
                  )}
                </>
              )}
            </div>
          </div>

          <Separator />

          {/* Backup runs */}
          <div className="space-y-2">
            <div className="text-sm font-medium">Recent backups</div>
            {!backups || backups.length === 0 ? (
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
          {restores && restores.length > 0 && (
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
                <span className="font-mono">{volume.mount_path}</span> with the
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

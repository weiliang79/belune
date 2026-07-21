import { useState } from "react";
import { toast } from "sonner";
import { ClockIcon, CloudIcon, HardDriveIcon } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useVolumes } from "@/lib/hooks/use-volumes";
import { useBackupDestinations } from "@/lib/hooks/use-backup-destinations";
import {
  useCreateVolumeBackupConfig,
  useUpdateVolumeBackupConfig,
} from "@/lib/hooks/use-volume-backups";
import type { AppVolumeBackupConfig } from "@/lib/types";

interface Props {
  projectId: string;
  applicationId: string;
  config?: AppVolumeBackupConfig | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

const SCHEDULE_PRESETS: { value: string; label: string }[] = [
  { value: "0 * * * *", label: "Every hour" },
  { value: "0 0 * * *", label: "Every day at midnight" },
  { value: "0 0 * * 0", label: "Every week (Sunday midnight)" },
  { value: "0 0 1 * *", label: "Every month (1st, midnight)" },
];

export function VolumeBackupConfigForm({
  projectId,
  applicationId,
  config,
  open,
  onOpenChange,
}: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        {/* Remount per open/target so fields initialise from props without an effect. */}
        {open && (
          <ConfigForm
            key={config?.id ?? "new"}
            projectId={projectId}
            applicationId={applicationId}
            config={config}
            onDone={() => onOpenChange(false)}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function ConfigForm({
  projectId,
  applicationId,
  config,
  onDone,
}: {
  projectId: string;
  applicationId: string;
  config?: AppVolumeBackupConfig | null;
  onDone: () => void;
}) {
  const editing = !!config;
  const { data: volumes } = useVolumes(projectId, applicationId);
  const { data: destinations } = useBackupDestinations(projectId);

  const [volumeId, setVolumeId] = useState(config?.application_volume_id ?? "");
  const [destinationId, setDestinationId] = useState(
    config?.destination_id ?? "",
  );
  const [prefix, setPrefix] = useState(config?.prefix ?? "");
  const [schedule, setSchedule] = useState(config?.schedule ?? "");
  const [customMode, setCustomMode] = useState(
    (config?.schedule ?? "") !== "" &&
      !SCHEDULE_PRESETS.some((p) => p.value === config?.schedule),
  );
  const [keepLatest, setKeepLatest] = useState(
    config?.keep_latest != null ? String(config.keep_latest) : "",
  );
  const [quiesce, setQuiesce] = useState(config?.quiesce ?? false);
  const [enabled, setEnabled] = useState(config?.enabled ?? true);

  const create = useCreateVolumeBackupConfig(projectId, applicationId, volumeId);
  const update = useUpdateVolumeBackupConfig(
    projectId,
    applicationId,
    volumeId,
    config?.id ?? "",
  );

  const presetValue = customMode
    ? "custom"
    : schedule === ""
      ? "manual"
      : (SCHEDULE_PRESETS.find((p) => p.value === schedule)?.value ?? "custom");

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!volumeId) {
      toast.error("Select a volume");
      return;
    }
    if (!destinationId) {
      toast.error("Select a destination");
      return;
    }
    const keep = keepLatest.trim() === "" ? null : Number(keepLatest);
    if (keep != null && (!Number.isInteger(keep) || keep < 1)) {
      toast.error("Keep latest must be a whole number ≥ 1");
      return;
    }
    const data = {
      destination_id: destinationId,
      prefix: prefix.trim(),
      schedule: schedule.trim(),
      keep_latest: keep,
      quiesce,
      enabled,
    };
    const action =
      editing && config
        ? update.mutateAsync(data)
        : create.mutateAsync(data);
    toast.promise(action, {
      loading: editing ? "Saving backup…" : "Creating backup…",
      success: () => {
        onDone();
        return editing ? "Backup saved" : "Backup created";
      },
      error: (err) => err.message,
    });
  };

  const pending = create.isPending || update.isPending;
  const noDestinations = destinations?.length === 0;
  const noVolumes = volumes?.length === 0;

  return (
    <>
      <DialogHeader>
        <DialogTitle>{editing ? "Edit Backup" : "Add Backup"}</DialogTitle>
        <DialogDescription>
          Back up a volume to a project destination, on a schedule or on demand.
        </DialogDescription>
      </DialogHeader>

      <form onSubmit={handleSubmit} className="space-y-4">
        <div className="space-y-1.5">
          <Label>Volume</Label>
          {editing ? (
            <div className="border-input flex items-center gap-2 rounded-md border px-3 py-2 text-sm">
              <HardDriveIcon aria-hidden="true" className="size-4" />
              <span className="font-medium">{config?.volume_name}</span>
              <span className="text-text-faint font-mono text-xs">
                {config?.mount_path}
              </span>
            </div>
          ) : (
            <Select value={volumeId} onValueChange={(v) => setVolumeId(v ?? "")}>
              <SelectTrigger>
                <SelectValue placeholder="Select a volume" />
              </SelectTrigger>
              <SelectContent>
                {(volumes ?? []).map((v) => (
                  <SelectItem key={v.id} value={v.id} icon={<HardDriveIcon />}>
                    {v.name}
                    <span className="text-text-faint ml-1 font-mono text-xs">
                      {v.mount_path}
                    </span>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
          {noVolumes && (
            <p className="text-muted-foreground text-xs">
              No volumes yet — add one in the Volumes list first.
            </p>
          )}
        </div>

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
          {noDestinations && (
            <p className="text-muted-foreground text-xs">
              No destinations yet — add one on the project Backups tab first.
            </p>
          )}
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="vbc-prefix">Prefix</Label>
          <Input
            id="vbc-prefix"
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
              <SelectItem value="manual" icon={<ClockIcon />}>
                Manual only (no schedule)
              </SelectItem>
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
          <Input
            value={schedule}
            onChange={(e) => {
              setCustomMode(false);
              setSchedule(e.target.value);
            }}
            placeholder="0 0 * * * (leave empty for manual only)"
            className="font-mono"
            aria-label="Cron expression"
          />
          <p className="text-muted-foreground text-xs">
            Standard 5-field cron expression. Empty = back up on demand only.
          </p>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="vbc-keep">Keep latest</Label>
          <Input
            id="vbc-keep"
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
          <Checkbox
            className="mt-0.5"
            checked={quiesce}
            onCheckedChange={setQuiesce}
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
          <Checkbox checked={enabled} onCheckedChange={setEnabled} />
          Enabled (run on schedule)
        </label>

        <DialogFooter>
          <Button type="submit" disabled={pending}>
            {pending ? "Saving…" : editing ? "Save" : "Create"}
          </Button>
        </DialogFooter>
      </form>
    </>
  );
}

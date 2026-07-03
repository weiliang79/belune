import { useState } from "react";
import { toast } from "sonner";
import { ClockIcon, CloudIcon, X } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  useCreateBackupConfig,
  useUpdateBackupConfig,
} from "@/lib/hooks/use-database-backup-configs";
import { useBackupDestinations } from "@/lib/hooks/use-backup-destinations";
import type { DatabaseBackupConfig } from "@/lib/types";

interface Props {
  projectId: string;
  databaseId: string;
  config?: DatabaseBackupConfig | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

const SCHEDULE_PRESETS: { value: string; label: string }[] = [
  { value: "0 * * * *", label: "Every hour" },
  { value: "0 0 * * *", label: "Every day at midnight" },
  { value: "0 13 * * *", label: "Every day at 1:00 PM" },
  { value: "0 0 * * 0", label: "Every week (Sunday midnight)" },
  { value: "0 0 1 * *", label: "Every month (1st, midnight)" },
];

export function BackupConfigFormDialog({
  projectId,
  databaseId,
  config,
  open,
  onOpenChange,
}: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        {/* Remount per open/target so fields initialise from props without an effect. */}
        {open && (
          <BackupConfigForm
            key={config?.id ?? "new"}
            projectId={projectId}
            databaseId={databaseId}
            config={config}
            onDone={() => onOpenChange(false)}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function BackupConfigForm({
  projectId,
  databaseId,
  config,
  onDone,
}: {
  projectId: string;
  databaseId: string;
  config?: DatabaseBackupConfig | null;
  onDone: () => void;
}) {
  const editing = !!config;
  const { data: destinations } = useBackupDestinations(projectId);
  const create = useCreateBackupConfig(projectId, databaseId);
  const update = useUpdateBackupConfig(projectId, databaseId);

  const [destinationId, setDestinationId] = useState(
    config?.destination_id ?? "",
  );
  const [schedule, setSchedule] = useState(config?.schedule ?? "0 0 * * *");
  const [prefix, setPrefix] = useState(config?.prefix ?? "");
  const [keepLatest, setKeepLatest] = useState(
    config?.keep_latest != null ? String(config.keep_latest) : "",
  );
  const [enabled, setEnabled] = useState(config?.enabled ?? true);
  const [databases, setDatabases] = useState<string[]>(config?.databases ?? []);
  const [dbInput, setDbInput] = useState("");

  const commitDbInput = () => {
    const v = dbInput.trim();
    if (v && !databases.includes(v)) setDatabases([...databases, v]);
    setDbInput("");
  };
  const handleDbKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter" || e.key === ",") {
      e.preventDefault();
      commitDbInput();
    } else if (e.key === "Backspace" && dbInput === "" && databases.length) {
      setDatabases(databases.slice(0, -1));
    }
  };

  const presetValue =
    SCHEDULE_PRESETS.find((p) => p.value === schedule)?.value ?? "custom";

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!destinationId) {
      toast.error("Select a destination");
      return;
    }
    const keep = keepLatest.trim() === "" ? null : Number(keepLatest);
    if (keep != null && (!Number.isInteger(keep) || keep < 1)) {
      toast.error("Keep latest must be a whole number ≥ 1");
      return;
    }
    // Include a pending typed-but-not-committed value so it isn't lost on submit.
    const pendingDb = dbInput.trim();
    const finalDatabases =
      pendingDb && !databases.includes(pendingDb)
        ? [...databases, pendingDb]
        : databases;
    const data = {
      destination_id: destinationId,
      schedule: schedule.trim(),
      prefix: prefix.trim(),
      keep_latest: keep,
      enabled,
      databases: finalDatabases,
    };
    const action =
      editing && config
        ? update.mutateAsync({ configId: config.id, data })
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

  return (
    <>
      <DialogHeader>
        <DialogTitle>{editing ? "Edit Backup" : "Add Backup"}</DialogTitle>
        <DialogDescription>
          Schedule recurring backups to a project destination.
        </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-3">
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
            <Label>Schedule</Label>
            <Select
              value={presetValue}
              onValueChange={(v) => {
                if (v && v !== "custom") setSchedule(v);
              }}
            >
              <SelectTrigger className="capitalize">
                <SelectValue placeholder="Select a predefined schedule" />
              </SelectTrigger>
              <SelectContent>
                {SCHEDULE_PRESETS.map((p) => (
                  <SelectItem
                    key={p.value}
                    value={p.value}
                    icon={<ClockIcon />}
                    className="capitalize"
                  >
                    {p.label}
                  </SelectItem>
                ))}
                <SelectItem value="custom" icon={<ClockIcon />} className="capitalize">
                  Custom…
                </SelectItem>
              </SelectContent>
            </Select>
            <Input
              value={schedule}
              onChange={(e) => setSchedule(e.target.value)}
              placeholder="Custom cron (e.g. 0 0 * * *)"
              required
            />
            <p className="text-muted-foreground text-xs">
              Standard 5-field cron expression.
            </p>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="cfg-database">Databases</Label>
            <div className="border-input focus-within:ring-ring flex flex-wrap items-center gap-1 rounded-md border px-2 py-1.5 focus-within:ring-1">
              {databases.map((d, i) => (
                <span
                  key={d}
                  className="bg-muted inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs"
                >
                  <code className="font-mono">{d}</code>
                  <button
                    type="button"
                    aria-label={`Remove ${d}`}
                    className="text-muted-foreground hover:text-foreground"
                    onClick={() =>
                      setDatabases(databases.filter((_, idx) => idx !== i))
                    }
                  >
                    <X className="h-3 w-3" />
                  </button>
                </span>
              ))}
              <input
                id="cfg-database"
                value={dbInput}
                onChange={(e) => setDbInput(e.target.value)}
                onKeyDown={handleDbKeyDown}
                onBlur={commitDbInput}
                placeholder={databases.length ? "" : "All databases"}
                className="placeholder:text-muted-foreground min-w-24 flex-1 bg-transparent text-sm outline-none"
              />
            </div>
            <p className="text-muted-foreground text-xs">
              Databases in this container to back up (type a name, press Enter).
              Leave empty to back up <span className="font-medium">all</span>{" "}
              databases.
            </p>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="cfg-prefix">Prefix</Label>
            <Input
              id="cfg-prefix"
              value={prefix}
              onChange={(e) => setPrefix(e.target.value)}
              placeholder="/my-project"
            />
            <p className="text-muted-foreground text-xs">
              Optional path inside the bucket.
            </p>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="cfg-keep">Keep latest</Label>
            <Input
              id="cfg-keep"
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

          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
              className="size-4"
            />
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

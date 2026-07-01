import { createFileRoute, useNavigate } from "@tanstack/react-router";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { useState } from "react";
import { BackupConfigFormDialog } from "@/components/databases/backup-config-form-dialog";
import { BackupConfigRunsSheet } from "@/components/databases/backup-config-runs-sheet";
import { useDatabaseBackupConfigs } from "@/lib/hooks/use-database-backup-configs";
import { useBackupDestinations } from "@/lib/hooks/use-backup-destinations";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
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
import { toast } from "sonner";
import {
  useDatabase,
  useDeleteDatabase,
  useUpdateDatabase,
  useSetDatabaseExternalAccess,
  useDatabaseBackups,
  useBackupDatabase,
  useUpgradeDatabase,
} from "@/lib/hooks/use-databases";
import { useProject } from "@/lib/hooks/use-projects";
import {
  Database as DatabaseIcon,
  Loader2,
  Trash2,
  TriangleAlert as AlertTriangleIcon,
  DatabaseBackup as DatabaseBackupIcon,
  Plus,
} from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import { useBreadcrumbLabel } from "@/lib/hooks/use-breadcrumb";
import { StatusBadge } from "@/lib/components/status-badge";
import { CopyButton } from "@/lib/components/copy-button";
import { formatBytes } from "@/lib/utils/format";
import { cn } from "@/lib/utils";
import type { Database, DatabaseBackupConfig } from "@/lib/types";

// Engines with an in-image logical-dump tool (pg_dump/mysqldump/mongodump).
// redis (cache) has no logical backup. "other" is backed up when a backup mode
// (volume snapshot or custom commands) was configured at creation.
const BACKUP_SUPPORTED_TYPES = ["postgres", "mysql", "mongo"];

function dbBackupEnabled(db: Database): boolean {
  if (db.type === "other") return db.backup_mode !== "none";
  return BACKUP_SUPPORTED_TYPES.includes(db.type);
}

// imageLabel shows the engine:tag for known engines and the full image for "other".
function imageLabel(db: Database): string {
  return db.type === "other" ? (db.image ?? "—") : `${db.type}:${db.version}`;
}

// Engines eligible for the guarded major-version upgrade (logical dump-and-reload).
function dbUpgradable(db: Database): boolean {
  return db.type === "postgres" || db.type === "mysql" || db.type === "mongo";
}

type DbTab = "overview" | "backups" | "advanced";

export const Route = createFileRoute(
  "/_app/projects/$projectId/databases/$databaseId",
)({
  component: DatabaseDetailPage,
  validateSearch: (search: Record<string, unknown>): { tab?: DbTab } =>
    search.tab === "backups" || search.tab === "advanced"
      ? { tab: search.tab as DbTab }
      : {},
});

function DatabaseDetailPage() {
  const { projectId, databaseId } = Route.useParams();
  const { tab } = Route.useSearch();
  const navigate = useNavigate();
  const { data: db, isLoading } = useDatabase(projectId, databaseId);
  const { data: project } = useProject(projectId);
  const deleteDb = useDeleteDatabase(projectId);

  const activeTab: DbTab = tab ?? "overview";
  const setTab = (next: DbTab) =>
    navigate({
      to: ".",
      search: () => ({ tab: next === "overview" ? undefined : next }),
      replace: true,
    });

  useBreadcrumbLabel(projectId, project?.name);
  useBreadcrumbLabel(databaseId, db?.name);

  if (isLoading || !db) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-4 w-64" />
        <div>
          <Skeleton className="h-7 w-48" />
          <Skeleton className="mt-1 h-4 w-24" />
        </div>
        <Card>
          <CardHeader>
            <Skeleton className="h-6 w-40" />
          </CardHeader>
          <CardContent className="space-y-3">
            {[1, 2, 3].map((i) => (
              <Skeleton key={i} className="h-5 w-full" />
            ))}
          </CardContent>
        </Card>
      </div>
    );
  }

  const handleDelete = () => {
    toast.promise(
      deleteDb.mutateAsync(databaseId).then(() => {
        navigate({
          to: "/projects/$projectId",
          params: { projectId },
        });
      }),
      {
        loading: "Deleting database...",
        success: "Database deleted",
        error: (err) => err.message,
      },
    );
  };

  return (
    <div className="space-y-6">

      <div className="flex items-start gap-3">
        <div className="bg-elev text-text-muted grid size-11 shrink-0 place-items-center rounded-xl">
          <DatabaseIcon aria-hidden="true" className="size-5" />
        </div>
        <div className="min-w-0">
          <h1 className="truncate text-2xl font-semibold tracking-tight">
            {db.name}
          </h1>
          <p className="text-text-faint truncate font-mono text-sm">
            {db.slug}
          </p>
          <div className="mt-2 flex items-center gap-2">
            <Badge variant="outline" className="font-mono">
              {imageLabel(db)}
            </Badge>
            <StatusBadge status={db.status} />
          </div>
        </div>
      </div>

      {(db.status === "creating" ||
        db.status === "upgrading" ||
        db.status === "backing_up") && (
        <Card>
          <CardContent className="flex items-center gap-3 py-6">
            <Loader2 className="h-5 w-5 animate-spin" />
            <div>
              <p className="font-medium">
                {db.status === "upgrading"
                  ? "Upgrading database…"
                  : db.status === "backing_up"
                    ? "Backing up database…"
                    : "Provisioning database..."}
              </p>
              <p className="text-muted-foreground text-sm">
                {db.status === "upgrading"
                  ? "Dumping, rebuilding at the new version, and restoring. The database is briefly offline."
                  : db.status === "backing_up"
                    ? "Briefly offline while a volume snapshot is taken."
                    : "This usually takes a few seconds."}
              </p>
            </div>
          </CardContent>
        </Card>
      )}

      {db.status === "failed" && (
        <Card>
          <CardContent className="space-y-1 py-6">
            <p className="text-destructive text-sm font-medium">
              This database is in a failed state.
            </p>
            <p className="text-muted-foreground text-sm">
              A failed provision or reconfigure usually leaves the data volume
              intact. If an upgrade was interrupted, recover from the most
              recent pre-upgrade backup. Otherwise remove the database from the
              Danger Zone (Advanced tab) if you no longer need it.
            </p>
          </CardContent>
        </Card>
      )}

      <nav className="flex gap-1 border-b">
        {(
          [
            { value: "overview", label: "Overview" },
            { value: "backups", label: "Backups" },
            { value: "advanced", label: "Advanced" },
          ] as const
        ).map((t) => (
          <button
            key={t.value}
            type="button"
            onClick={() => setTab(t.value)}
            className={cn(
              "border-b-2 px-4 py-2 text-sm font-medium transition-colors",
              activeTab === t.value
                ? "border-primary text-foreground"
                : "text-muted-foreground hover:text-foreground border-transparent",
            )}
          >
            {t.label}
          </button>
        ))}
      </nav>

      {activeTab === "overview" && (
        <div className="space-y-6">
          {db.status === "running" && db.credentials ? (
            <Card>
              <CardHeader>
                <CardTitle>Connection Details</CardTitle>
                <CardDescription>
                  Use these credentials to connect from your services on the
                  internal network.
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-3">
                  <div className="flex items-center justify-between text-sm">
                    <span className="text-muted-foreground">Host</span>
                    <div className="flex items-center gap-1">
                      <code className="text-xs">{db.internal_host}</code>
                      <CopyButton value={db.internal_host} />
                    </div>
                  </div>
                  <div className="flex items-center justify-between text-sm">
                    <span className="text-muted-foreground">Port</span>
                    <div className="flex items-center gap-1">
                      <code className="text-xs">{db.internal_port}</code>
                      <CopyButton value={String(db.internal_port)} />
                    </div>
                  </div>
                  <Separator />
                  {Object.entries(db.credentials).map(([key, value]) => (
                    <div
                      key={key}
                      className="flex items-center justify-between text-sm"
                    >
                      <span className="text-muted-foreground">{key}</span>
                      <div className="flex items-center gap-1">
                        <code className="text-xs">{value}</code>
                        <CopyButton value={value} />
                      </div>
                    </div>
                  ))}
                </div>

                {db.connection_string && (
                  <>
                    <Separator />
                    <div className="space-y-2">
                      <p className="text-muted-foreground text-sm">
                        Connection String
                      </p>
                      <div className="bg-muted flex items-center justify-between rounded-md px-3 py-2">
                        <code className="text-xs break-all">
                          {db.connection_string}
                        </code>
                        <CopyButton value={db.connection_string} />
                      </div>
                    </div>
                  </>
                )}
              </CardContent>
            </Card>
          ) : (
            db.status !== "failed" && (
              <Card>
                <CardContent className="text-muted-foreground py-6 text-sm">
                  Connection details appear once the database is running.
                </CardContent>
              </Card>
            )
          )}

          {db.status === "running" && <ExternalAccessCard db={db} />}
        </div>
      )}

      {activeTab === "backups" && (
        <div className="space-y-6">
          {db.status === "running" && dbBackupEnabled(db) ? (
            <BackupsTab db={db} />
          ) : (
            <Card>
              <CardContent className="text-muted-foreground py-6 text-sm">
                {dbBackupEnabled(db)
                  ? "Backups are available once the database is running."
                  : "Backups are not supported for this database type."}
              </CardContent>
            </Card>
          )}
        </div>
      )}

      {activeTab === "advanced" && (
        <div className="space-y-6">
          {db.status === "running" && <AdvancedCard db={db} />}

          <Card>
            <CardHeader>
              <CardTitle className="text-destructive">Danger Zone</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm font-medium">Delete this database</p>
                  <p className="text-muted-foreground text-xs">
                    This will permanently delete the database, its data, and
                    container.
                  </p>
                </div>
                <AlertDialog>
                  <AlertDialogTrigger
                    render={<Button variant="destructive" size="sm" />}
                  >
                    <Trash2 className="mr-1 size-4" />
                    Delete
                  </AlertDialogTrigger>
                  <AlertDialogContent>
                    <AlertDialogHeader>
                      <AlertDialogTitle>Delete database?</AlertDialogTitle>
                      <AlertDialogDescription>
                        This will permanently delete &quot;{db.name}&quot; and
                        all its data. This action cannot be undone.
                      </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                      <AlertDialogCancel>Cancel</AlertDialogCancel>
                      <AlertDialogAction
                        onClick={handleDelete}
                        className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                      >
                        {deleteDb.isPending ? (
                          <Loader2 className="mr-1 h-4 w-4 animate-spin" />
                        ) : null}
                        Delete
                      </AlertDialogAction>
                    </AlertDialogFooter>
                  </AlertDialogContent>
                </AlertDialog>
              </div>
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  );
}

/** Per-engine localhost connection string reached through the SSH tunnel. */
function localConnectionString(db: Database, localPort: number): string {
  const c = db.credentials ?? {};
  switch (db.type) {
    case "postgres":
      return `postgresql://${c.user}:${c.password}@localhost:${localPort}/${c.database}`;
    case "mysql":
      return `mysql://${c.user}:${c.password}@localhost:${localPort}/${c.database}`;
    case "redis":
      return `redis://:${c.password}@localhost:${localPort}`;
    case "mongo":
      return `mongodb://${c.username}:${c.password}@localhost:${localPort}`;
    default:
      return "";
  }
}

/**
 * External Access (SSH tunnel). Toggling binds/unbinds a loopback host port,
 * which recreates the container — surfaced via the warning so the brief
 * downtime isn't a surprise. The database is never exposed publicly.
 */
function ExternalAccessCard({ db }: { db: Database }) {
  const setAccess = useSetDatabaseExternalAccess(db.project_id, db.id);
  const ext = db.external_access;
  const enabled = ext?.enabled ?? false;

  const handleToggle = () => {
    toast.promise(setAccess.mutateAsync(!enabled), {
      loading: enabled
        ? "Disabling external access…"
        : "Enabling external access…",
      success: enabled
        ? "Disabling — recreating container…"
        : "Enabling — recreating container…",
      error: (err) => err.message,
    });
  };

  const localPort = db.internal_port;
  const sshHost = ext?.ssh_host || "<server-host>";
  const sshUser = ext?.ssh_user || "<user>";
  const sshCmd = `ssh -L ${localPort}:127.0.0.1:${ext?.host_port ?? ""} ${sshUser}@${sshHost}`;
  const connStr = localConnectionString(db, localPort);

  return (
    <Card>
      <CardHeader>
        <CardTitle>External Access</CardTitle>
        <CardDescription>
          Reach this database from your machine over an SSH tunnel. It is bound
          to the server&apos;s loopback only — never exposed publicly.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Toggle + recreate warning */}
        <div className="flex items-center justify-between gap-3">
          <div>
            <p className="text-sm font-medium">Enable external access</p>
            <p className="text-text-faint text-xs">
              Binds a loopback port for SSH tunneling.
            </p>
          </div>
          <button
            type="button"
            role="switch"
            aria-checked={enabled}
            aria-label="Enable external access"
            disabled={setAccess.isPending}
            onClick={handleToggle}
            className={cn(
              "focus-visible:ring-ring relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors focus-visible:ring-2 focus-visible:outline-none disabled:opacity-50",
              enabled ? "bg-primary" : "bg-input",
            )}
          >
            <span
              className={cn(
                "bg-background pointer-events-none inline-block h-4 w-4 rounded-full shadow-lg transition-transform",
                enabled ? "translate-x-4" : "translate-x-0",
              )}
            />
          </button>
        </div>

        <div className="bg-status-building-soft text-status-building ring-status-building-line flex items-start gap-2 rounded-md px-3 py-2 text-xs ring-1 ring-inset">
          <AlertTriangleIcon
            aria-hidden="true"
            className="mt-0.5 size-3.5 shrink-0"
          />
          <span>
            Toggling external access recreates the database container — expect a
            few seconds of downtime. Your data is preserved (the volume is
            reattached).
          </span>
        </div>

        {/* Tunnel details — only when enabled */}
        {enabled && ext?.host_port && (
          <div className="space-y-3 border-t pt-4">
            <div className="space-y-1.5">
              <p className="text-muted-foreground text-xs">
                SSH tunnel command
              </p>
              <div className="bg-muted flex items-center justify-between gap-2 rounded-md px-3 py-2">
                <code className="text-xs break-all">{sshCmd}</code>
                <CopyButton value={sshCmd} />
              </div>
            </div>

            {connStr && (
              <div className="space-y-1.5">
                <p className="text-muted-foreground text-xs">
                  Connection string (via tunnel)
                </p>
                <div className="bg-muted flex items-center justify-between gap-2 rounded-md px-3 py-2">
                  <code className="text-xs break-all">{connStr}</code>
                  <CopyButton value={connStr} />
                </div>
              </div>
            )}

            <p className="text-text-faint text-xs">
              GUI clients (TablePlus, DBeaver, DataGrip) can use their built-in
              SSH-tunnel option instead of running the command above. Once
              connected, point the client at{" "}
              <span className="font-mono">localhost:{localPort}</span>.
            </p>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

const MB = 1024 * 1024;

/**
 * Advanced settings: editable CPU/memory limits applied live, plus read-only
 * image and managed-volume info. Image upgrades are intentionally not editable
 * here — a major version bump is a separate guarded flow.
 */
function AdvancedCard({ db }: { db: Database }) {
  const update = useUpdateDatabase(db.project_id, db.id);
  const upgrade = useUpgradeDatabase(db.project_id, db.id);
  const [cpu, setCpu] = useState(String(db.cpu_limit ?? 0));
  const [memMb, setMemMb] = useState(
    String(db.memory_limit ? Math.round(db.memory_limit / MB) : 0),
  );
  const [targetVersion, setTargetVersion] = useState("");

  const handleUpgrade = () => {
    const target = targetVersion.trim();
    if (!target) return;
    toast.promise(upgrade.mutateAsync(target), {
      loading: "Starting upgrade…",
      success: "Upgrade started — the database will be briefly offline",
      error: (err) => err.message,
    });
    setTargetVersion("");
  };

  const handleSave = () => {
    const cpuVal = Number(cpu);
    const memVal = Number(memMb);
    if (
      Number.isNaN(cpuVal) ||
      cpuVal < 0 ||
      Number.isNaN(memVal) ||
      memVal < 0
    ) {
      toast.error("CPU and memory must be non-negative numbers");
      return;
    }
    toast.promise(
      update.mutateAsync({ cpu_limit: cpuVal, memory_limit: memVal * MB }),
      {
        loading: "Applying resource limits…",
        success: "Resource limits updated",
        error: (err) => err.message,
      },
    );
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Advanced</CardTitle>
        <CardDescription>
          Resource limits apply live. 0 means unlimited.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        {/* Resource limits — editable */}
        <div className="grid gap-4 sm:grid-cols-2">
          <div className="space-y-2">
            <Label htmlFor="db-cpu">CPU limit (cores)</Label>
            <Input
              id="db-cpu"
              type="number"
              min={0}
              step={0.1}
              value={cpu}
              onChange={(e) => setCpu(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="db-mem">Memory limit (MB)</Label>
            <Input
              id="db-mem"
              type="number"
              min={0}
              step={64}
              value={memMb}
              onChange={(e) => setMemMb(e.target.value)}
            />
          </div>
        </div>
        <Button size="sm" onClick={handleSave} disabled={update.isPending}>
          {update.isPending ? "Saving…" : "Save resource limits"}
        </Button>

        <Separator />

        {/* Image — read-only, with guarded upgrade for known engines */}
        <div className="flex items-start justify-between gap-3">
          <div>
            <p className="text-sm font-medium">Image</p>
            <p className="text-text-faint text-xs">
              {db.type === "other"
                ? "Change it by recreating the database."
                : "Major version upgrades dump, rebuild, and restore (brief downtime)."}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <Badge variant="outline" className="font-mono">
              {imageLabel(db)}
            </Badge>
            {dbUpgradable(db) && (
              <AlertDialog>
                <AlertDialogTrigger
                  render={<Button variant="outline" size="sm" />}
                >
                  Upgrade
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>
                      Upgrade {db.type} version
                    </AlertDialogTitle>
                    <AlertDialogDescription>
                      This takes a dump of{" "}
                      <span className="font-medium">{db.name}</span>, rebuilds
                      the container at the new version, and restores the data.
                      The database is briefly offline. If anything fails it
                      rolls back to {db.version}. A pre-upgrade backup is kept.
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <div className="space-y-2">
                    <Label htmlFor="upgrade-version">
                      Target version (current: {db.version})
                    </Label>
                    <Input
                      id="upgrade-version"
                      value={targetVersion}
                      onChange={(e) => setTargetVersion(e.target.value)}
                      placeholder="e.g. 17"
                    />
                  </div>
                  <AlertDialogFooter>
                    <AlertDialogCancel>Cancel</AlertDialogCancel>
                    <AlertDialogAction
                      onClick={handleUpgrade}
                      disabled={
                        !targetVersion.trim() ||
                        targetVersion.trim() === db.version
                      }
                    >
                      Upgrade
                    </AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            )}
          </div>
        </div>

        {/* Data directory — read-only ("other" only) */}
        {db.type === "other" && db.data_dir && (
          <div className="flex items-start justify-between gap-3">
            <div>
              <p className="text-sm font-medium">Data directory</p>
              <p className="text-text-faint text-xs">
                Set at creation; cannot be changed.
              </p>
            </div>
            <Badge variant="outline" className="font-mono">
              {db.data_dir}
            </Badge>
          </div>
        )}

        {/* Volume — read-only */}
        <div className="flex items-start justify-between gap-3">
          <div>
            <p className="text-sm font-medium">Volume</p>
            <p className="text-text-faint text-xs">Managed automatically.</p>
          </div>
          <div className="text-right">
            {db.volume ? (
              <>
                <p className="font-mono text-xs">{db.volume.name}</p>
                <p className="text-text-faint text-xs">
                  {formatBytes(db.volume.size_bytes)}
                </p>
              </>
            ) : (
              <p className="text-text-faint text-xs">—</p>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

function BackupsTab({ db }: { db: Database }) {
  const { data: configs, isLoading } = useDatabaseBackupConfigs(
    db.project_id,
    db.id,
  );
  const { data: destinations } = useBackupDestinations(db.project_id);
  const { data: backups } = useDatabaseBackups(db.project_id, db.id);
  const backup = useBackupDatabase(db.project_id, db.id);

  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<DatabaseBackupConfig | null>(null);
  const [sheetOpen, setSheetOpen] = useState(false);
  const [selected, setSelected] = useState<DatabaseBackupConfig | null>(null);

  const destName = (id: string) =>
    destinations?.find((d) => d.id === id)?.name ?? "Unknown destination";
  const noDestinations = destinations?.length === 0;
  const anyRunning = backups?.some((b) => b.status === "running") ?? false;

  const openAdd = () => {
    setEditing(null);
    setFormOpen(true);
  };
  const openManage = (cfg: DatabaseBackupConfig) => {
    setSelected(cfg);
    setSheetOpen(true);
  };
  const openEdit = (cfg: DatabaseBackupConfig) => {
    setSheetOpen(false);
    setEditing(cfg);
    setFormOpen(true);
  };

  const handleAdHocBackup = () => {
    toast.promise(backup.mutateAsync(), {
      loading: "Starting backup…",
      success: "Backup started",
      error: (err) => err.message,
    });
  };

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <CardTitle>Backups</CardTitle>
            <CardDescription>
              Schedule recurring backups to a project destination, or run an
              ad-hoc backup now.
            </CardDescription>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button
              size="sm"
              variant="outline"
              onClick={handleAdHocBackup}
              disabled={backup.isPending || anyRunning}
            >
              <DatabaseBackupIcon className="mr-1 h-4 w-4" />
              Back up now
            </Button>
            <Button size="sm" onClick={openAdd} disabled={noDestinations}>
              <Plus className="mr-1 h-4 w-4" />
              Add Backup
            </Button>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        {noDestinations && (
          <p className="text-text-faint mb-3 text-sm">
            Add a destination on the project{" "}
            <span className="font-medium">Backups</span> tab before scheduling
            backups.
          </p>
        )}
        {isLoading ? (
          <Skeleton className="h-16 w-full" />
        ) : !configs || configs.length === 0 ? (
          <p className="text-text-faint py-4 text-center text-sm">
            No scheduled backups yet.
          </p>
        ) : (
          <ul className="divide-border divide-y">
            {configs.map((c) => (
              <li
                key={c.id}
                className="flex items-center justify-between gap-3 py-3"
              >
                <div className="min-w-0 space-y-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-medium">{destName(c.destination_id)}</span>
                    {c.enabled ? (
                      <Badge variant="outline">Active</Badge>
                    ) : (
                      <Badge variant="secondary">Disabled</Badge>
                    )}
                  </div>
                  <p className="text-text-faint text-xs">
                    <code className="font-mono">{c.schedule}</code>
                    {c.all_databases
                      ? " · all databases"
                      : ` · ${c.databases.join(", ")}`}
                    {c.prefix ? ` · ${c.prefix}` : ""}
                    {c.keep_latest != null ? ` · keep ${c.keep_latest}` : ""}
                  </p>
                </div>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => openManage(c)}
                >
                  Manage
                </Button>
              </li>
            ))}
          </ul>
        )}
      </CardContent>

      <BackupConfigFormDialog
        projectId={db.project_id}
        databaseId={db.id}
        config={editing}
        open={formOpen}
        onOpenChange={setFormOpen}
      />
      <BackupConfigRunsSheet
        db={db}
        config={selected}
        destinationName={selected ? destName(selected.destination_id) : undefined}
        open={sheetOpen}
        onOpenChange={setSheetOpen}
        onEdit={openEdit}
      />
    </Card>
  );
}

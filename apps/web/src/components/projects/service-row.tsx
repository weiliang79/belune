import { useState } from "react";
import { Link, useNavigate } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import {
  AppWindowIcon,
  ArrowUpRightIcon,
  DatabaseIcon,
  Loader2Icon,
  MoreHorizontalIcon,
  PlayIcon,
  RocketIcon,
  RotateCcwIcon,
  ScrollTextIcon,
  SquareIcon,
  Trash2Icon,
} from "lucide-react";
import { toast } from "sonner";
import type { Application, Database, ServiceMetrics } from "@/lib/types";
import {
  useDeleteApplication,
  useDeployApplication,
  useRestartApplication,
  useStartApplication,
  useStopApplication,
} from "@/lib/hooks/use-applications";
import {
  useDeleteDatabase,
  useRestartDatabase,
  useStartDatabase,
  useStopDatabase,
} from "@/lib/hooks/use-databases";
import { queryKeys } from "@/lib/hooks/query-keys";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { StatusPill } from "@/components/ui/status-pill";
import { formatUptime } from "@/lib/utils/format";
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
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Tooltip,
  TooltipContent,
  TooltipPositioner,
  TooltipTrigger,
} from "@/components/ui/tooltip";

// Icon-only row action with a hover/focus tooltip label.
function IconAction({
  label,
  onClick,
  children,
}: {
  label: string;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            variant="ghost"
            size="icon"
            aria-label={label}
            onClick={onClick}
          />
        }
      >
        {children}
      </TooltipTrigger>
      <TooltipPositioner>
        <TooltipContent>{label}</TooltipContent>
      </TooltipPositioner>
    </Tooltip>
  );
}

const TRANSIENT = new Set([
  "building",
  "deploying",
  "pending",
  "queued",
  "creating",
  "restarting",
  "provisioning",
  "upgrading",
  "backing_up",
]);

const isTransient = (status: string) => TRANSIENT.has(status.toLowerCase());

const RUNNING = new Set(["running", "ready"]);

function formatBytes(bytes: number): string {
  const gb = bytes / (1024 * 1024 * 1024);
  if (gb >= 1) return `${gb.toFixed(1)} GB`;
  const mb = bytes / (1024 * 1024);
  if (mb >= 1) return `${mb.toFixed(0)} MB`;
  return `${(bytes / 1024).toFixed(0)} KB`;
}

/** Shared row chrome: icon, name link, type badge, runtime columns, and actions. */
function RowShell({
  to,
  params,
  icon,
  name,
  slug,
  typeLabel,
  status,
  metrics,
  port,
  actions,
}: {
  to: string;
  params: Record<string, string>;
  icon: React.ReactNode;
  name: string;
  slug: string;
  typeLabel: string;
  status: string;
  metrics?: ServiceMetrics;
  port?: number;
  actions: React.ReactNode;
}) {
  const running = RUNNING.has(status.toLowerCase());
  const domain = metrics?.domain;
  const effectivePort = port ?? metrics?.port;

  return (
    <div className="hover:bg-card-hover grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 px-4 py-3 transition-colors md:grid-cols-[minmax(0,2fr)_140px_minmax(0,1.4fr)_80px_104px_104px]">
      {/* Name / Type */}
      <div className="flex min-w-0 items-center gap-3">
        <div className="bg-elev text-text-muted grid size-9 shrink-0 place-items-center rounded-lg">
          {icon}
        </div>
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <Link
              to={to}
              params={params}
              className="hover:text-primary truncate text-sm font-medium transition-colors"
            >
              {name}
            </Link>
            <span className="text-text-faint hidden truncate font-mono text-xs lg:inline">
              {slug}
            </span>
          </div>
          <div className="mt-1 flex items-center gap-2">
            <Badge variant="outline" className="capitalize">
              {typeLabel}
            </Badge>
          </div>
        </div>
      </div>

      {/* Status (+ uptime) */}
      <div className="hidden flex-col items-start gap-0.5 md:flex">
        <StatusPill status={status} />
        {running && metrics?.uptime_seconds ? (
          <span className="text-text-faint text-xs">
            Up {formatUptime(metrics.uptime_seconds)}
          </span>
        ) : null}
      </div>

      {/* Port · Domain */}
      <div className="text-text-faint hidden min-w-0 font-mono text-xs md:block">
        {effectivePort ? <span>:{effectivePort}</span> : null}
        {domain ? (
          <div className="text-text-muted truncate">{domain}</div>
        ) : null}
        {!effectivePort && !domain ? "—" : null}
      </div>

      {/* CPU */}
      <div className="text-text-muted hidden font-mono text-xs md:block">
        {metrics && running ? `${metrics.cpu_percent.toFixed(0)}%` : "—"}
      </div>

      {/* Memory */}
      <div className="text-text-muted hidden font-mono text-xs md:block">
        {metrics && running && metrics.memory_used
          ? formatBytes(metrics.memory_used)
          : "—"}
      </div>

      {/* Actions */}
      <div
        className="flex items-center justify-start gap-1"
        onClick={(e) => e.stopPropagation()}
      >
        {actions}
      </div>
    </div>
  );
}

function ApplicationRow({
  projectId,
  app,
  metrics,
}: {
  projectId: string;
  app: Application;
  metrics?: ServiceMetrics;
}) {
  const qc = useQueryClient();
  const navigate = useNavigate();
  const deploy = useDeployApplication(projectId, app.id);
  const stop = useStopApplication(projectId, app.id);
  const start = useStartApplication(projectId, app.id);
  const restart = useRestartApplication(projectId, app.id);
  const del = useDeleteApplication(projectId);
  const [confirmOpen, setConfirmOpen] = useState(false);

  // Lifecycle hooks invalidate the app *detail* query; nudge the list too so the
  // row reflects the new status without waiting for the next poll.
  const refreshList = () =>
    qc.invalidateQueries({ queryKey: queryKeys.applications.all(projectId) });
  const onSuccess = { onSuccess: refreshList };

  const status = app.status.toLowerCase();
  const busy = isTransient(status) || stop.isPending || start.isPending || restart.isPending;

  let primary: React.ReactNode = null;
  if (busy) {
    primary = (
      <Button variant="ghost" size="icon" disabled aria-label="Working">
        <Loader2Icon aria-hidden="true" className="size-4 animate-spin" />
      </Button>
    );
  } else if (status === "running") {
    primary = (
      <>
        <IconAction
          label="Stop"
          onClick={() => stop.mutate(undefined, onSuccess)}
        >
          <SquareIcon aria-hidden="true" className="size-4" />
        </IconAction>
        <IconAction
          label="Restart"
          onClick={() => restart.mutate(undefined, onSuccess)}
        >
          <RotateCcwIcon aria-hidden="true" className="size-4" />
        </IconAction>
      </>
    );
  } else if (status === "failed" || status === "error" || status === "crashed") {
    primary = (
      <IconAction
        label="Restart"
        onClick={() => restart.mutate(undefined, onSuccess)}
      >
        <RotateCcwIcon aria-hidden="true" className="size-4" />
      </IconAction>
    );
  } else {
    primary = (
      <IconAction
        label="Start"
        onClick={() => start.mutate(undefined, onSuccess)}
      >
        <PlayIcon aria-hidden="true" className="size-4" />
      </IconAction>
    );
  }

  const open = () =>
    navigate({
      to: "/projects/$projectId/applications/$applicationId",
      params: { projectId, applicationId: app.id },
    });

  return (
    <RowShell
      to="/projects/$projectId/applications/$applicationId"
      params={{ projectId, applicationId: app.id }}
      icon={<AppWindowIcon aria-hidden="true" className="size-4.5" />}
      name={app.name}
      slug={app.slug}
      typeLabel={app.type}
      status={app.status}
      metrics={metrics}
      actions={
        <>
          {primary}
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label="More actions"
                  title="More"
                />
              }
            >
              <MoreHorizontalIcon aria-hidden="true" className="size-4" />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={open}>
                <ArrowUpRightIcon aria-hidden="true" />
                Open
              </DropdownMenuItem>
              <DropdownMenuItem
                onClick={() =>
                  navigate({
                    to: "/projects/$projectId/applications/$applicationId/logs",
                    params: { projectId, applicationId: app.id },
                  })
                }
              >
                <ScrollTextIcon aria-hidden="true" />
                View logs
              </DropdownMenuItem>
              <DropdownMenuItem
                disabled={busy}
                onClick={() => deploy.mutate(undefined, onSuccess)}
              >
                <RocketIcon aria-hidden="true" />
                Redeploy
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                variant="destructive"
                onClick={() => setConfirmOpen(true)}
              >
                <Trash2Icon aria-hidden="true" />
                Delete
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>

          <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>Delete {app.name}?</AlertDialogTitle>
                <AlertDialogDescription>
                  This permanently deletes the application and stops its
                  container. This action cannot be undone.
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>Cancel</AlertDialogCancel>
                <AlertDialogAction
                  onClick={() =>
                    toast.promise(del.mutateAsync(app.id), {
                      loading: "Deleting application…",
                      success: `${app.name} deleted`,
                      error: (err) => err.message,
                    })
                  }
                >
                  Delete
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </>
      }
    />
  );
}

function DatabaseRow({
  projectId,
  db,
}: {
  projectId: string;
  db: Database;
}) {
  const navigate = useNavigate();
  const stop = useStopDatabase(projectId, db.id);
  const start = useStartDatabase(projectId, db.id);
  const restart = useRestartDatabase(projectId, db.id);
  const del = useDeleteDatabase(projectId);
  const [confirmOpen, setConfirmOpen] = useState(false);

  const status = db.status.toLowerCase();
  const busy =
    isTransient(status) ||
    stop.isPending ||
    start.isPending ||
    restart.isPending;

  let primary: React.ReactNode = null;
  if (busy) {
    primary = (
      <Button variant="ghost" size="icon" disabled aria-label="Working">
        <Loader2Icon aria-hidden="true" className="size-4 animate-spin" />
      </Button>
    );
  } else if (status === "running") {
    primary = (
      <>
        <IconAction label="Stop" onClick={() => stop.mutate()}>
          <SquareIcon aria-hidden="true" className="size-4" />
        </IconAction>
        <IconAction label="Restart" onClick={() => restart.mutate()}>
          <RotateCcwIcon aria-hidden="true" className="size-4" />
        </IconAction>
      </>
    );
  } else if (status === "failed" || status === "error") {
    primary = (
      <IconAction label="Restart" onClick={() => restart.mutate()}>
        <RotateCcwIcon aria-hidden="true" className="size-4" />
      </IconAction>
    );
  } else {
    primary = (
      <IconAction label="Start" onClick={() => start.mutate()}>
        <PlayIcon aria-hidden="true" className="size-4" />
      </IconAction>
    );
  }

  const open = () =>
    navigate({
      to: "/projects/$projectId/databases/$databaseId",
      params: { projectId, databaseId: db.id },
    });

  return (
    <RowShell
      to="/projects/$projectId/databases/$databaseId"
      params={{ projectId, databaseId: db.id }}
      icon={<DatabaseIcon aria-hidden="true" className="size-4.5" />}
      name={db.name}
      slug={db.slug}
      typeLabel={db.type}
      status={db.status}
      port={db.internal_port ?? undefined}
      actions={
        <>
          {primary}
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label="More actions"
                  title="More"
                />
              }
            >
              <MoreHorizontalIcon aria-hidden="true" className="size-4" />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={open}>
                <ArrowUpRightIcon aria-hidden="true" />
                Open
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                variant="destructive"
                onClick={() => setConfirmOpen(true)}
              >
                <Trash2Icon aria-hidden="true" />
                Delete
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>

          <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>Delete {db.name}?</AlertDialogTitle>
                <AlertDialogDescription>
                  This permanently deletes the database and its volume. This
                  action cannot be undone.
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>Cancel</AlertDialogCancel>
                <AlertDialogAction
                  onClick={() =>
                    toast.promise(del.mutateAsync(db.id), {
                      loading: "Deleting database…",
                      success: `${db.name} deleted`,
                      error: (err) => err.message,
                    })
                  }
                >
                  Delete
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </>
      }
    />
  );
}

export type ServiceRowItem =
  | { kind: "application"; data: Application }
  | { kind: "database"; data: Database };

/** A single service row (application or database) with inline lifecycle actions. */
export function ServiceRow({
  projectId,
  item,
  metrics,
}: {
  projectId: string;
  item: ServiceRowItem;
  metrics?: ServiceMetrics;
}) {
  return item.kind === "application" ? (
    <ApplicationRow projectId={projectId} app={item.data} metrics={metrics} />
  ) : (
    <DatabaseRow projectId={projectId} db={item.data} />
  );
}

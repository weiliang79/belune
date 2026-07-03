import { useMemo, useState } from "react";
import { Link, useNavigate } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import type { ColumnDef } from "@tanstack/react-table";
import { DataTable } from "@/components/ui/data-table";
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
import type {
  Application,
  Database,
  ProjectMetrics,
  ServiceMetrics,
} from "@/lib/types";
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

export type ServiceRowItem =
  | { kind: "application"; data: Application }
  | { kind: "database"; data: Database };

// Icon-only row action with a hover/focus tooltip label.
function IconAction({
  label,
  onClick,
  children,
  className,
}: {
  label: string;
  onClick: () => void;
  children: React.ReactNode;
  className?: string;
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
            className={className}
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

function ApplicationActions({
  projectId,
  app,
}: {
  projectId: string;
  app: Application;
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
  const busy =
    isTransient(status) || stop.isPending || start.isPending || restart.isPending;

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
          className="text-destructive hover:bg-destructive/10 hover:text-destructive"
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
    <div className="flex items-center justify-end gap-1">
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
              This permanently deletes the application and stops its container.
              This action cannot be undone.
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
    </div>
  );
}

function DatabaseActions({
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
        <IconAction
          label="Stop"
          onClick={() => stop.mutate()}
          className="text-destructive hover:bg-destructive/10 hover:text-destructive"
        >
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
    <div className="flex items-center justify-end gap-1">
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
              This permanently deletes the database and its volume. This action
              cannot be undone.
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
    </div>
  );
}

/** Inline lifecycle controls (start/stop/restart + kebab menu) for a service. */
export function ServiceActions({
  projectId,
  item,
}: {
  projectId: string;
  item: ServiceRowItem;
}) {
  return item.kind === "application" ? (
    <ApplicationActions projectId={projectId} app={item.data} />
  ) : (
    <DatabaseActions projectId={projectId} db={item.data} />
  );
}

function NameCell({
  projectId,
  item,
}: {
  projectId: string;
  item: ServiceRowItem;
}) {
  const isApp = item.kind === "application";
  return (
    <div className="flex min-w-0 items-center gap-3">
      <div className="bg-elev text-text-muted grid size-9 shrink-0 place-items-center rounded-lg">
        {isApp ? (
          <AppWindowIcon aria-hidden="true" className="size-4.5" />
        ) : (
          <DatabaseIcon aria-hidden="true" className="size-4.5" />
        )}
      </div>
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          {isApp ? (
            <Link
              to="/projects/$projectId/applications/$applicationId"
              params={{ projectId, applicationId: item.data.id }}
              className="hover:text-primary truncate text-sm font-medium transition-colors"
            >
              {item.data.name}
            </Link>
          ) : (
            <Link
              to="/projects/$projectId/databases/$databaseId"
              params={{ projectId, databaseId: item.data.id }}
              className="hover:text-primary truncate text-sm font-medium transition-colors"
            >
              {item.data.name}
            </Link>
          )}
          <span className="text-text-faint hidden truncate font-mono text-xs lg:inline">
            {item.data.slug}
          </span>
        </div>
        <Badge variant="outline" className="mt-1 capitalize">
          {item.data.type}
        </Badge>
      </div>
    </div>
  );
}

function StatusCell({
  item,
  metrics,
}: {
  item: ServiceRowItem;
  metrics?: ServiceMetrics;
}) {
  const running = RUNNING.has(item.data.status.toLowerCase());
  return (
    <div className="flex flex-col items-start gap-0.5">
      <StatusPill status={item.data.status} />
      {running && metrics?.uptime_seconds ? (
        <span className="text-text-faint text-xs">
          Up {formatUptime(metrics.uptime_seconds)}
        </span>
      ) : null}
    </div>
  );
}

function EndpointCell({
  item,
  metrics,
}: {
  item: ServiceRowItem;
  metrics?: ServiceMetrics;
}) {
  const port =
    item.kind === "database"
      ? (item.data.internal_port ?? undefined)
      : undefined;
  const effectivePort = port ?? metrics?.port;
  const domain = metrics?.domain;
  return (
    <div className="text-text-faint min-w-0 font-mono text-xs">
      {effectivePort ? <span>:{effectivePort}</span> : null}
      {domain ? <div className="text-text-muted truncate">{domain}</div> : null}
      {!effectivePort && !domain ? "—" : null}
    </div>
  );
}

function usageText(item: ServiceRowItem, metrics: ServiceMetrics | undefined) {
  return metrics && RUNNING.has(item.data.status.toLowerCase()) ? metrics : null;
}

/**
 * Standard column definitions for the project services table. `metricsFor`
 * resolves live metrics per row (applications only); databases have none.
 */
function buildColumns(
  projectId: string,
  metricsFor: (item: ServiceRowItem) => ServiceMetrics | undefined,
): ColumnDef<ServiceRowItem>[] {
  return [
    {
      id: "name",
      header: "Name / Type",
      cell: ({ row }) => <NameCell projectId={projectId} item={row.original} />,
    },
    {
      id: "status",
      header: "Status",
      cell: ({ row }) => (
        <StatusCell item={row.original} metrics={metricsFor(row.original)} />
      ),
    },
    {
      id: "endpoint",
      header: "Port · Domain",
      meta: { className: "hidden md:table-cell", headerClassName: "hidden md:table-cell" },
      cell: ({ row }) => (
        <EndpointCell item={row.original} metrics={metricsFor(row.original)} />
      ),
    },
    {
      id: "cpu",
      header: "CPU",
      meta: {
        className: "text-text-muted hidden font-mono text-xs md:table-cell",
        headerClassName: "hidden md:table-cell",
      },
      cell: ({ row }) => {
        const m = usageText(row.original, metricsFor(row.original));
        return m ? `${m.cpu_percent.toFixed(0)}%` : "—";
      },
    },
    {
      id: "memory",
      header: "Memory",
      meta: {
        className: "text-text-muted hidden font-mono text-xs md:table-cell",
        headerClassName: "hidden md:table-cell",
      },
      cell: ({ row }) => {
        const m = usageText(row.original, metricsFor(row.original));
        return m && m.memory_used ? formatBytes(m.memory_used) : "—";
      },
    },
    {
      id: "actions",
      header: "",
      meta: { headerClassName: "text-right", className: "text-right" },
      cell: ({ row }) => (
        <ServiceActions projectId={projectId} item={row.original} />
      ),
    },
  ];
}

/** Standard column table of a project's services (applications + databases). */
export function ServicesTable({
  projectId,
  items,
  metrics,
  isLoading,
}: {
  projectId: string;
  items: ServiceRowItem[];
  metrics?: ProjectMetrics;
  isLoading?: boolean;
}) {
  const columns = useMemo(
    () =>
      buildColumns(projectId, (item) =>
        item.kind === "application" ? metrics?.[item.data.id] : undefined,
      ),
    [projectId, metrics],
  );

  return (
    <DataTable
      columns={columns}
      data={items}
      getRowId={(it) => it.data.id}
      isLoading={isLoading}
      emptyMessage="No services match the current filters."
    />
  );
}

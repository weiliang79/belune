import { useState } from "react";
import { Link, useNavigate } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import {
  AppWindowIcon,
  ArrowUpRightIcon,
  DatabaseBackupIcon,
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
import type { Application, Database } from "@/lib/types";
import {
  useDeleteApplication,
  useDeployApplication,
  useRestartApplication,
  useStartApplication,
  useStopApplication,
} from "@/lib/hooks/use-applications";
import {
  useBackupDatabase,
  useDeleteDatabase,
} from "@/lib/hooks/use-databases";
import { queryKeys } from "@/lib/hooks/query-keys";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { StatusPill } from "@/components/ui/status-pill";
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

/** Shared row chrome: icon, name link, type badge, detail line, and the right cluster. */
function RowShell({
  to,
  params,
  icon,
  name,
  slug,
  typeLabel,
  detail,
  status,
  actions,
}: {
  to: string;
  params: Record<string, string>;
  icon: React.ReactNode;
  name: string;
  slug: string;
  typeLabel: string;
  detail: React.ReactNode;
  status: string;
  actions: React.ReactNode;
}) {
  return (
    <div className="hover:bg-card-hover grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 px-4 py-3 transition-colors md:grid-cols-[minmax(0,2.2fr)_minmax(0,1.4fr)_140px_120px]">
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

      {/* Detail (source / version · port) */}
      <div className="text-text-faint hidden min-w-0 truncate font-mono text-xs md:block">
        {detail}
      </div>

      {/* Status */}
      <div className="hidden md:flex">
        <StatusPill status={status} />
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
  const busy = isTransient(status) || stop.isPending || start.isPending || restart.isPending;

  const detail =
    app.type === "image"
      ? app.source_image
      : `${app.branch ? app.branch + " · " : ""}${app.build_type}`;

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
        <Button
          variant="ghost"
          size="icon"
          title="Stop"
          aria-label="Stop"
          onClick={() => stop.mutate(undefined, onSuccess)}
        >
          <SquareIcon aria-hidden="true" className="size-4" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          title="Restart"
          aria-label="Restart"
          onClick={() => restart.mutate(undefined, onSuccess)}
        >
          <RotateCcwIcon aria-hidden="true" className="size-4" />
        </Button>
      </>
    );
  } else if (status === "failed" || status === "error" || status === "crashed") {
    primary = (
      <Button
        variant="ghost"
        size="icon"
        title="Restart"
        aria-label="Restart"
        onClick={() => restart.mutate(undefined, onSuccess)}
      >
        <RotateCcwIcon aria-hidden="true" className="size-4" />
      </Button>
    );
  } else {
    primary = (
      <Button
        variant="ghost"
        size="icon"
        title="Start"
        aria-label="Start"
        onClick={() => start.mutate(undefined, onSuccess)}
      >
        <PlayIcon aria-hidden="true" className="size-4" />
      </Button>
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
      detail={detail}
      status={app.status}
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
  const backup = useBackupDatabase(projectId, db.id);
  const del = useDeleteDatabase(projectId);
  const [confirmOpen, setConfirmOpen] = useState(false);

  const busy = isTransient(db.status) || backup.isPending;
  const detail = `${db.type}:${db.version}${db.internal_port ? " · :" + db.internal_port : ""}`;

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
      detail={detail}
      status={db.status}
      actions={
        <>
          <Button
            variant="ghost"
            size="icon"
            title="Back up now"
            aria-label="Back up now"
            disabled={busy}
            onClick={() =>
              toast.promise(backup.mutateAsync(), {
                loading: "Starting backup…",
                success: "Backup started",
                error: (err) => err.message,
              })
            }
          >
            {backup.isPending ? (
              <Loader2Icon aria-hidden="true" className="size-4 animate-spin" />
            ) : (
              <DatabaseBackupIcon aria-hidden="true" className="size-4" />
            )}
          </Button>
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
}: {
  projectId: string;
  item: ServiceRowItem;
}) {
  return item.kind === "application" ? (
    <ApplicationRow projectId={projectId} app={item.data} />
  ) : (
    <DatabaseRow projectId={projectId} db={item.data} />
  );
}

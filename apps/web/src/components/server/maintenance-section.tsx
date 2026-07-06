import { useState } from "react";
import { toast } from "sonner";
import {
  ChevronDownIcon,
  HardDriveIcon,
  HelpCircleIcon,
  RouteIcon,
  ListChecksIcon,
  RefreshCwIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
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
import { Separator } from "@/components/ui/separator";
import { useDockerOverview } from "@/lib/hooks/use-docker";
import {
  useClearQueue,
  useQueueStatus,
  useReconcileProxy,
  useReconcilerStatus,
  useRunCleanup,
} from "@/lib/hooks/use-maintenance";
import { useSettings, useUpdateSettings } from "@/lib/hooks/use-settings";
import type { CleanupAction } from "@/lib/api/maintenance";
import { formatBytes, formatRelativeTime } from "@/lib/utils/format";
import { cn } from "@/lib/utils";

type CleanTarget = {
  actions?: CleanupAction[]; // undefined = full cleanup
  label: string;
  description: string;
};

export function MaintenanceSection() {
  const { data: overview } = useDockerOverview();
  const runCleanup = useRunCleanup();
  const [pendingClean, setPendingClean] = useState<CleanTarget | null>(null);

  const du = overview?.disk_usage;
  const reclaimable =
    (du?.images.reclaimable ?? 0) +
    (du?.volumes.reclaimable ?? 0) +
    (du?.build_cache.reclaimable ?? 0);

  const doClean = (t: CleanTarget) => {
    setPendingClean(null);
    toast.promise(runCleanup.mutateAsync(t.actions), {
      loading: `Cleaning ${t.label}…`,
      success: "Cleanup queued",
      error: (err) => err.message,
    });
  };

  return (
    <>
      <div className="space-y-6">
        {/* ---- Disk cleanup ---- */}
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <HardDriveIcon className="text-text-muted size-4" />
              <p className="text-sm font-medium">Disk cleanup</p>
              <span className="text-status-ready text-sm">
                {formatBytes(reclaimable)} reclaimable
              </span>
            </div>
            <p className="text-muted-foreground mt-1 text-xs">
              Images {formatBytes(du?.images.reclaimable ?? 0)} · Volumes{" "}
              {formatBytes(du?.volumes.reclaimable ?? 0)} · Build cache{" "}
              {formatBytes(du?.build_cache.reclaimable ?? 0)}. Protects app data,
              caches in use, and running containers.
            </p>
          </div>
          <div className="flex items-center gap-2">
            <DropdownMenu>
              <DropdownMenuTrigger
                render={<Button size="sm" variant="outline" />}
              >
                Clean
                <ChevronDownIcon aria-hidden="true" className="size-4" />
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem
                  onClick={() =>
                    setPendingClean({
                      actions: ["images"],
                      label: "dangling images",
                      description:
                        "Removes unreferenced (dangling) images. In-use images are kept.",
                    })
                  }
                >
                  Dangling images
                </DropdownMenuItem>
                <DropdownMenuItem
                  onClick={() =>
                    setPendingClean({
                      actions: ["volumes"],
                      label: "unused volumes",
                      description:
                        "Removes dangling volumes not owned by the platform. App data (paas-data) and in-use volumes are preserved.",
                    })
                  }
                >
                  Unused volumes
                </DropdownMenuItem>
                <DropdownMenuItem
                  onClick={() =>
                    setPendingClean({
                      actions: ["build_cache"],
                      label: "build caches",
                      description:
                        "Removes CNB cache volumes and the BuildKit builder cache. The next build repopulates them.",
                    })
                  }
                >
                  Build caches
                </DropdownMenuItem>
                <DropdownMenuItem
                  onClick={() =>
                    setPendingClean({
                      actions: ["containers"],
                      label: "orphaned containers",
                      description:
                        "Removes platform containers that no longer have a matching application. Running and stopped apps are kept.",
                    })
                  }
                >
                  Orphaned containers
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
            <Button
              size="sm"
              variant="outline"
              disabled={runCleanup.isPending}
              onClick={() =>
                setPendingClean({
                  actions: undefined,
                  label: "everything",
                  description:
                    "Runs all cleanup steps: prune old deployments, dangling images, unused volumes, build caches, and orphaned containers.",
                })
              }
            >
              Run full cleanup
            </Button>
          </div>
        </div>

        <Separator />

        {/* ---- Proxy (Caddy) ---- */}
        <ProxyBlock />

        <Separator />

        {/* ---- Job queue ---- */}
        <QueueBlock />

        <Separator />

        {/* ---- Daily automatic cleanup ---- */}
        <DailyCleanupToggle />
      </div>

      <AlertDialog
        open={pendingClean !== null}
        onOpenChange={(open) => !open && setPendingClean(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Clean {pendingClean?.label}?</AlertDialogTitle>
            <AlertDialogDescription>
              {pendingClean?.description}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => pendingClean && doClean(pendingClean)}
            >
              Clean
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

function ProxyBlock() {
  const { data: status } = useReconcilerStatus();
  const reconcile = useReconcileProxy();

  const lastRun =
    status && status.run_count > 0 && status.last_run_at
      ? formatRelativeTime(status.last_run_at)
      : null;

  const handleReconcile = () => {
    toast.promise(reconcile.mutateAsync(), {
      loading: "Reconciling routes…",
      success: (s) =>
        s.last_error
          ? `Reconciled with warnings: ${s.last_error}`
          : `Routes reconciled (+${s.last_added} / −${s.last_removed})`,
      error: (err) => err.message,
    });
  };

  return (
    <div className="flex flex-wrap items-center justify-between gap-3">
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <RouteIcon className="text-text-muted size-4" />
          <p className="text-sm font-medium">Proxy (Caddy)</p>
          {lastRun && (
            <span
              className={cn(
                "text-xs",
                status?.last_error ? "text-status-error" : "text-text-faint",
              )}
            >
              reconciled {lastRun}
              {status?.last_error ? " · error" : ""}
            </span>
          )}
        </div>
        <p className="text-muted-foreground mt-1 text-xs">
          Re-push all app routes to Caddy to fix any drift between the database
          and the live proxy.
        </p>
      </div>
      <Button
        size="sm"
        variant="outline"
        disabled={reconcile.isPending}
        onClick={handleReconcile}
      >
        <RefreshCwIcon aria-hidden="true" className="size-4" />
        Reconcile routes
      </Button>
    </div>
  );
}

function QueueBlock() {
  const { data: status } = useQueueStatus();
  const clear = useClearQueue();
  const [confirm, setConfirm] = useState(false);

  const stuck = status?.total_stuck ?? 0;

  const handleClear = () => {
    setConfirm(false);
    toast.promise(clear.mutateAsync(), {
      loading: "Clearing stuck jobs…",
      success: (r) => `Cleared ${r.cleared} stuck job${r.cleared === 1 ? "" : "s"}`,
      error: (err) => err.message,
    });
  };

  return (
    <>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <ListChecksIcon className="text-text-muted size-4" />
            <p className="text-sm font-medium">Job queue</p>
            <span
              className={cn(
                "text-xs",
                stuck > 0 ? "text-status-building" : "text-text-faint",
              )}
            >
              {stuck} stuck
            </span>
          </div>
          <p className="text-muted-foreground mt-1 text-xs">
            Clear failed (dead-letter) and retrying jobs. Pending and in-flight
            jobs are never touched.
          </p>
        </div>
        <Button
          size="sm"
          variant="outline"
          disabled={clear.isPending || stuck === 0}
          onClick={() => setConfirm(true)}
        >
          Clear stuck jobs
        </Button>
      </div>

      <AlertDialog open={confirm} onOpenChange={setConfirm}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Clear {stuck} stuck jobs?</AlertDialogTitle>
            <AlertDialogDescription>
              Removes failed and retrying tasks from all queues. Pending and
              running tasks are left untouched.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={handleClear}>Clear</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

function DailyCleanupToggle() {
  const { data: settings } = useSettings();
  const updateSettings = useUpdateSettings();

  // Absent or any value other than "false" = enabled (matches the backend default).
  const enabled =
    settings?.find((s) => s.key === "daily_cleanup_enabled")?.value !== "false";

  const handleToggle = () => {
    const next = !enabled;
    toast.promise(
      updateSettings.mutateAsync([
        { key: "daily_cleanup_enabled", value: next ? "true" : "false" },
      ]),
      {
        loading: "Saving…",
        success: `Daily cleanup ${next ? "enabled" : "disabled"}`,
        error: (err) => err.message,
      },
    );
  };

  return (
    <div className="flex items-center justify-between gap-3">
      <div className="flex items-center gap-1.5">
        <p className="text-sm font-medium">Daily automatic cleanup</p>
        <HelpCircleIcon
          className="text-text-faint size-3.5"
          aria-hidden="true"
          {...({
            title:
              "Runs the full cleanup once a day. When off, disk is only reclaimed when you run cleanup manually.",
          } as object)}
        />
      </div>
      <button
        type="button"
        role="switch"
        aria-checked={enabled}
        aria-label="Daily automatic cleanup"
        disabled={updateSettings.isPending}
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
  );
}

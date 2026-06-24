import { useMemo, useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { Database, DatabaseIcon, Globe, LayersIcon, PlusIcon } from "lucide-react";
import { useApplications } from "@/lib/hooks/use-applications";
import { useDatabases } from "@/lib/hooks/use-databases";
import { useProjectMetrics } from "@/lib/hooks/use-project-metrics";
import type { ColumnDef } from "@tanstack/react-table";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { DataTable } from "@/components/ui/data-table";
import { ApplicationFormDialog } from "@/components/applications/application-form-dialog";
import { DatabaseFormDialog } from "@/components/databases/database-form-dialog";
import {
  ServiceRow,
  type ServiceRowItem,
} from "@/components/projects/service-row";

const serviceColumns: ColumnDef<ServiceRowItem>[] = [];

export const Route = createFileRoute("/_app/projects/$projectId/")({
  component: ProjectOverview,
});

type TypeFilter = "all" | "application" | "database";
type StatusFilter = "all" | "running" | "stopped" | "error";

const STATUS_GROUPS: Record<Exclude<StatusFilter, "all">, Set<string>> = {
  running: new Set(["running", "ready"]),
  stopped: new Set(["stopped", "inactive", "paused", "exited"]),
  error: new Set(["failed", "error", "crashed", "unhealthy"]),
};

function ProjectOverview() {
  const { projectId } = Route.useParams();
  const { data: applications, isLoading: applicationsLoading } =
    useApplications(projectId);
  const { data: databases, isLoading: databasesLoading } =
    useDatabases(projectId);
  const { data: serviceMetrics } = useProjectMetrics(projectId);

  const [appDialogOpen, setAppDialogOpen] = useState(false);
  const [dbDialogOpen, setDbDialogOpen] = useState(false);
  const [typeFilter, setTypeFilter] = useState<TypeFilter>("all");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");

  const loading = applicationsLoading || databasesLoading;

  const items = useMemo<ServiceRowItem[]>(() => {
    const all: ServiceRowItem[] = [
      ...(applications ?? []).map(
        (data) => ({ kind: "application", data }) as ServiceRowItem,
      ),
      ...(databases ?? []).map(
        (data) => ({ kind: "database", data }) as ServiceRowItem,
      ),
    ];
    return all.filter((item) => {
      if (typeFilter !== "all" && item.kind !== typeFilter) return false;
      if (statusFilter !== "all") {
        const group = STATUS_GROUPS[statusFilter];
        if (!group.has(item.data.status.toLowerCase())) return false;
      }
      return true;
    });
  }, [applications, databases, typeFilter, statusFilter]);

  const isEmpty =
    !loading &&
    (applications?.length ?? 0) === 0 &&
    (databases?.length ?? 0) === 0;

  return (
    <div className="space-y-4">
      <ApplicationFormDialog
        projectId={projectId}
        open={appDialogOpen}
        onOpenChange={setAppDialogOpen}
      />
      <DatabaseFormDialog
        projectId={projectId}
        open={dbDialogOpen}
        onOpenChange={setDbDialogOpen}
      />

      {/* Filter bar */}
      <div className="flex flex-wrap items-center gap-2">
        <ToggleGroup
          variant="outline"
          size="sm"
          value={[typeFilter]}
          onValueChange={(v) => v.length > 0 && setTypeFilter(v[0] as TypeFilter)}
        >
          <ToggleGroupItem value="all">All types</ToggleGroupItem>
          <ToggleGroupItem value="application">
            <Globe />
            App
          </ToggleGroupItem>
          <ToggleGroupItem value="database">
            <Database />
            Database
          </ToggleGroupItem>
        </ToggleGroup>
        <ToggleGroup
          variant="outline"
          size="sm"
          value={[statusFilter]}
          onValueChange={(v) =>
            v.length > 0 && setStatusFilter(v[0] as StatusFilter)
          }
        >
          <ToggleGroupItem value="all">All</ToggleGroupItem>
          <ToggleGroupItem value="running">Running</ToggleGroupItem>
          <ToggleGroupItem value="stopped">Stopped</ToggleGroupItem>
          <ToggleGroupItem value="error">Error</ToggleGroupItem>
        </ToggleGroup>

        <div className="ml-auto flex items-center gap-2">
          <span className="text-text-faint text-xs">
            {items.length} service{items.length === 1 ? "" : "s"}
          </span>
          <Button variant="outline" size="sm" onClick={() => setDbDialogOpen(true)}>
            <DatabaseIcon aria-hidden="true" className="size-4" />
            New Database
          </Button>
          <Button size="sm" onClick={() => setAppDialogOpen(true)}>
            <PlusIcon aria-hidden="true" className="size-4" />
            New Application
          </Button>
        </div>
      </div>

      {/* Service table */}
      {loading ? (
        <div className="divide-border divide-y overflow-hidden rounded-xl border">
          {[1, 2, 3].map((i) => (
            <div key={i} className="flex items-center gap-3 px-4 py-3">
              <Skeleton className="size-9 rounded-lg" />
              <div className="flex-1 space-y-1.5">
                <Skeleton className="h-4 w-40" />
                <Skeleton className="h-3 w-24" />
              </div>
              <Skeleton className="h-5 w-16 rounded-full" />
            </div>
          ))}
        </div>
      ) : isEmpty ? (
        <div className="rounded-xl border border-dashed py-16 text-center">
          <LayersIcon
            aria-hidden="true"
            className="text-text-faint mx-auto size-8"
          />
          <p className="text-muted-foreground mt-3 text-sm">
            No applications or databases yet.
          </p>
          <p className="text-text-faint mt-1 text-xs">
            Use{" "}
            <span className="text-foreground font-medium">New Application</span>{" "}
            or <span className="text-foreground font-medium">New Database</span>{" "}
            above to get started.
          </p>
        </div>
      ) : (
        <div className="overflow-hidden rounded-xl border">
          {/* Column header */}
          <div className="text-text-faint bg-elev/40 hidden grid-cols-[minmax(0,2fr)_140px_minmax(0,1.4fr)_80px_104px_104px] gap-3 px-4 py-2 text-xs font-medium md:grid">
            <span>Name / Type</span>
            <span>Status</span>
            <span>Port · Domain</span>
            <span>CPU</span>
            <span>Memory</span>
            <span>Actions</span>
          </div>
          <DataTable
            columns={serviceColumns}
            data={items}
            getRowId={(it) => it.data.id}
            customView={{
              wrapperProps: { className: "divide-border divide-y" },
              item: ({ row }) => (
                <ServiceRow
                  projectId={projectId}
                  item={row.original}
                  metrics={
                    row.original.kind === "application"
                      ? serviceMetrics?.[row.original.data.id]
                      : undefined
                  }
                />
              ),
              empty: (
                <div className="text-muted-foreground px-4 py-10 text-center text-sm">
                  No services match the current filters.
                </div>
              ),
            }}
          />
        </div>
      )}
    </div>
  );
}

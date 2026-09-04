import { useMemo, useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import {
  Database,
  DatabaseIcon,
  Globe,
  LayersIcon,
  PlusIcon,
} from "lucide-react";
import { useApplications } from "@/lib/hooks/use-applications";
import { useDatabases } from "@/lib/hooks/use-databases";
import { useProject } from "@/lib/hooks/use-projects";
import { useProjectMetrics } from "@/lib/hooks/use-project-metrics";
import { useAuthStore } from "@/lib/stores/auth";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
  SegmentedControl,
  SegmentedControlItem,
} from "@/components/ui/segmented-control";
import { ApplicationFormDialog } from "@/components/applications/application-form-dialog";
import { DatabaseFormDialog } from "@/components/databases/database-form-dialog";
import {
  ServicesTable,
  type ServiceRowItem,
} from "@/components/projects/service-row";

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
  const { data: project } = useProject(projectId);
  const currentUser = useAuthStore((s) => s.user);
  const canDelete =
    currentUser?.role === "admin" || currentUser?.id === project?.user_id;

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

      <Card>
        <CardContent className="space-y-4">
          {/* Filter + actions toolbar */}
          <div className="flex flex-wrap items-center gap-2">
            <SegmentedControl
              size="sm"
              value={typeFilter}
              onValueChange={(v) => setTypeFilter(v as TypeFilter)}
            >
              <SegmentedControlItem value="all">All types</SegmentedControlItem>
              <SegmentedControlItem value="application">
                <Globe />
                App
              </SegmentedControlItem>
              <SegmentedControlItem value="database">
                <Database />
                Database
              </SegmentedControlItem>
            </SegmentedControl>
            <SegmentedControl
              size="sm"
              value={statusFilter}
              onValueChange={(v) => setStatusFilter(v as StatusFilter)}
            >
              <SegmentedControlItem value="all">All</SegmentedControlItem>
              <SegmentedControlItem value="running">
                Running
              </SegmentedControlItem>
              <SegmentedControlItem value="stopped">
                Stopped
              </SegmentedControlItem>
              <SegmentedControlItem value="error">Error</SegmentedControlItem>
            </SegmentedControl>

            <div className="ml-auto flex items-center gap-2">
              <span className="text-text-faint text-xs">
                {items.length} service{items.length === 1 ? "" : "s"}
              </span>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setDbDialogOpen(true)}
              >
                <DatabaseIcon aria-hidden="true" className="size-4" />
                New Database
              </Button>
              <Button size="sm" onClick={() => setAppDialogOpen(true)}>
                <PlusIcon aria-hidden="true" className="size-4" />
                New Application
              </Button>
            </div>
          </div>

          {isEmpty ? (
            <div className="py-12 text-center">
              <LayersIcon
                aria-hidden="true"
                className="text-text-faint mx-auto size-8"
              />
              <p className="text-muted-foreground mt-3 text-sm">
                No applications or databases yet.
              </p>
              <p className="text-text-faint mt-1 text-xs">
                Use{" "}
                <span className="text-foreground font-medium">
                  New Application
                </span>{" "}
                or{" "}
                <span className="text-foreground font-medium">
                  New Database
                </span>{" "}
                above to get started.
              </p>
            </div>
          ) : (
            <ServicesTable
              projectId={projectId}
              canDelete={canDelete}
              items={items}
              metrics={serviceMetrics}
              isLoading={loading}
            />
          )}
        </CardContent>
      </Card>
    </div>
  );
}

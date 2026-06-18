import { createFileRoute, Link } from "@tanstack/react-router";
import { useCallback, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useGlobalDeployments } from "@/lib/hooks/use-global-deployments";
import { useProjects } from "@/lib/hooks/use-projects";
import { StatusPill } from "@/components/ui/status-pill";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import { DateRangePicker } from "@/components/ui/date-range-picker";
import { AppBreadcrumb } from "@/lib/components/app-breadcrumb";
import { formatDate, formatDuration } from "@/lib/utils/format";
import type { GlobalDeployment } from "@/lib/types";
import * as applicationsApi from "@/lib/api/applications";

export const Route = createFileRoute("/_app/deployments/")({
  component: GlobalDeploymentsPage,
});

const STATUSES = [
  { label: "All statuses", value: "" },
  { label: "Success", value: "success" },
  { label: "Failed", value: "failed" },
  { label: "Building", value: "building" },
  { label: "Deploying", value: "deploying" },
  { label: "Pending", value: "pending" },
] as const;

const PAGE_SIZE = 50;

interface Filters {
  dateFrom?: string;
  dateTo?: string;
  status: string;
  projectId: string;
  applicationId: string;
}

function DeploymentFilters({
  filters,
  onChange,
}: {
  filters: Filters;
  onChange: (f: Filters) => void;
}) {
  const { data: projects } = useProjects();
  const { data: applications } = useQuery({
    queryKey: ["applications", filters.projectId],
    queryFn: () => applicationsApi.listApplications(filters.projectId),
    enabled: !!filters.projectId,
  });

  return (
    <div className="flex flex-wrap gap-2">
      <DateRangePicker
        value={{ from: filters.dateFrom, to: filters.dateTo }}
        onChange={(range) =>
          onChange({ ...filters, dateFrom: range.from, dateTo: range.to })
        }
        placeholder="All time"
        className="w-64"
      />

      <Select
        value={filters.status}
        onValueChange={(v) => onChange({ ...filters, status: v ?? "" })}
      >
        <SelectTrigger className="w-36">
          <SelectValue placeholder="All statuses" />
        </SelectTrigger>
        <SelectContent>
          {STATUSES.map((s) => (
            <SelectItem key={s.value} value={s.value}>
              {s.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <Select
        items={projects?.map((p) => ({
          label: p.name,
          value: p.id,
        }))}
        value={filters.projectId}
        onValueChange={(v) =>
          onChange({ ...filters, projectId: v ?? "", applicationId: "" })
        }
      >
        <SelectTrigger className="w-40">
          <SelectValue placeholder="All projects" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="">All projects</SelectItem>
          {projects?.map((p) => (
            <SelectItem key={p.id} value={p.id}>
              {p.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <Select
        items={applications?.map((a) => ({
          label: a.name,
          value: a.id,
        }))}
        value={filters.applicationId}
        onValueChange={(v) => onChange({ ...filters, applicationId: v ?? "" })}
        disabled={!filters.projectId}
      >
        <SelectTrigger className="w-44">
          <SelectValue placeholder="All applications" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="">All applications</SelectItem>
          {applications?.map((a) => (
            <SelectItem key={a.id} value={a.id}>
              {a.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

function DeploymentRow({ d }: { d: GlobalDeployment }) {
  const duration =
    d.finished_at && d.started_at
      ? formatDuration(
          new Date(d.finished_at).getTime() - new Date(d.started_at).getTime(),
        )
      : null;

  return (
    <div className="flex items-center justify-between">
      <div className="flex min-w-0 flex-1 items-center gap-3">
        <StatusPill status={d.status} className="shrink-0" />
        <div className="min-w-0">
          <div className="flex items-center gap-1.5 text-sm font-medium">
            <span className="text-muted-foreground">{d.project_name}</span>
            <span className="text-muted-foreground">/</span>
            <span>{d.application_name}</span>
          </div>
          <div className="text-muted-foreground flex items-center gap-2 text-xs">
            <span className="capitalize">{d.triggered_by}</span>
            {d.commit_sha && (
              <span className="font-mono">{d.commit_sha.slice(0, 7)}</span>
            )}
          </div>
        </div>
      </div>
      <div className="text-muted-foreground flex shrink-0 items-center gap-3 text-xs">
        {duration && <span>{duration}</span>}
        <span>{formatDate(d.started_at)}</span>
      </div>
    </div>
  );
}

function GlobalDeploymentsPage() {
  const [offset, setOffset] = useState(0);
  const [filters, setFilters] = useState<Filters>({
    status: "",
    projectId: "",
    applicationId: "",
  });

  const handleFilterChange = useCallback((f: Filters) => {
    setFilters(f);
    setOffset(0);
  }, []);

  const queryParams = useMemo(
    () => ({
      limit: PAGE_SIZE,
      offset,
      project_id: filters.projectId || undefined,
      application_id: filters.applicationId || undefined,
      status: filters.status || undefined,
      from: filters.dateFrom,
      to: filters.dateTo,
    }),
    [filters, offset],
  );

  const { data: deployments, isLoading } = useGlobalDeployments(queryParams);

  return (
    <div className="space-y-6">
      <AppBreadcrumb items={[{ label: "Deployments" }]} />
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Deployments</h1>
        <p className="text-muted-foreground text-sm">
          All deployments across your applications.
        </p>
      </div>

      <DeploymentFilters filters={filters} onChange={handleFilterChange} />

      {isLoading ? (
        <div className="space-y-2">
          {[1, 2, 3, 4, 5].map((i) => (
            <div
              key={i}
              className="flex items-center justify-between rounded-lg border px-4 py-3"
            >
              <div className="flex items-center gap-3">
                <Skeleton className="h-5 w-16 rounded-full" />
                <div className="space-y-1">
                  <Skeleton className="h-4 w-40" />
                  <Skeleton className="h-3 w-24" />
                </div>
              </div>
              <Skeleton className="h-3 w-32" />
            </div>
          ))}
        </div>
      ) : !deployments || deployments.length === 0 ? (
        <div className="text-muted-foreground rounded-lg border py-12 text-center text-sm">
          {offset > 0 ? "No more deployments." : "No deployments found."}
        </div>
      ) : (
        <div className="space-y-2">
          {deployments.map((d) => (
            <Link
              key={d.id}
              to="/projects/$projectId/applications/$applicationId/deployments"
              params={{
                projectId: d.project_id,
                applicationId: d.application_id,
              }}
              className="hover:bg-card-hover block rounded-lg border px-4 py-3 transition-colors"
            >
              <DeploymentRow d={d} />
            </Link>
          ))}
        </div>
      )}

      {/* Pagination */}
      <div className="flex items-center justify-between">
        <Button
          variant="outline"
          size="sm"
          disabled={offset === 0}
          onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
        >
          Previous
        </Button>
        <span className="text-muted-foreground text-sm">
          Showing {offset + 1}–{offset + (deployments?.length ?? 0)}
        </span>
        <Button
          variant="outline"
          size="sm"
          disabled={!deployments || deployments.length < PAGE_SIZE}
          onClick={() => setOffset(offset + PAGE_SIZE)}
        >
          Next
        </Button>
      </div>
    </div>
  );
}

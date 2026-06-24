import { createFileRoute, Link } from "@tanstack/react-router";
import { useCallback, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { SearchIcon } from "lucide-react";
import type { ColumnDef } from "@tanstack/react-table";
import { useGlobalDeployments } from "@/lib/hooks/use-global-deployments";
import { useProjects } from "@/lib/hooks/use-projects";
import { StatusPill } from "@/components/ui/status-pill";
import { StatCard } from "@/lib/components/stats/stat-card";
import { useStats } from "@/lib/hooks/use-stats";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import { DataTable } from "@/components/ui/data-table";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { TimeRangeTabs } from "@/components/ui/time-range-tabs";
import { timeRangeToDates, type TimeRange } from "@/lib/utils/time-range";
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
  range: TimeRange;
  search: string;
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
    <div className="flex flex-wrap items-center gap-2">
      <TimeRangeTabs
        value={filters.range}
        onChange={(range) => onChange({ ...filters, range })}
      />

      <div className="relative min-w-0 flex-1 sm:max-w-xs">
        <SearchIcon
          aria-hidden="true"
          className="text-text-faint pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2"
        />
        <Input
          value={filters.search}
          onChange={(e) => onChange({ ...filters, search: e.target.value })}
          placeholder="Commit SHA or app name…"
          aria-label="Search deployments"
          className="pl-9"
        />
      </div>

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

const deploymentColumns: ColumnDef<GlobalDeployment>[] = [
  {
    id: "app",
    header: "App / Project",
    cell: ({ row: { original: d } }) => (
      <Link
        to="/projects/$projectId/applications/$applicationId/deployments"
        params={{ projectId: d.project_id, applicationId: d.application_id }}
        className="hover:text-primary block"
      >
        <span className="font-medium">{d.application_name}</span>
        <span className="text-text-faint block text-xs">{d.project_name}</span>
      </Link>
    ),
  },
  {
    id: "status",
    header: "Status",
    cell: ({ row: { original: d } }) => <StatusPill status={d.status} />,
  },
  {
    id: "commit",
    header: "Commit",
    cell: ({ row: { original: d } }) =>
      d.commit_sha ? (
        <span className="font-mono text-xs">{d.commit_sha.slice(0, 7)}</span>
      ) : (
        <span className="text-text-faint">—</span>
      ),
  },
  {
    id: "image",
    header: "Image",
    meta: { className: "max-w-[14rem] truncate font-mono text-xs" },
    cell: ({ row: { original: d } }) =>
      d.image_tag ? d.image_tag : <span className="text-text-faint">—</span>,
  },
  {
    id: "triggered_by",
    header: "Triggered by",
    cell: ({ row: { original: d } }) => (
      <span className="capitalize">{d.triggered_by}</span>
    ),
  },
  {
    id: "started",
    header: "Started",
    meta: { className: "text-text-muted whitespace-nowrap" },
    cell: ({ row: { original: d } }) => formatDate(d.started_at),
  },
  {
    id: "duration",
    header: "Duration",
    meta: { className: "text-text-muted whitespace-nowrap" },
    cell: ({ row: { original: d } }) => {
      const ms =
        d.finished_at && d.started_at
          ? new Date(d.finished_at).getTime() -
            new Date(d.started_at).getTime()
          : null;
      return ms != null ? (
        formatDuration(ms)
      ) : (
        <span className="text-text-faint">—</span>
      );
    },
  },
];

/** 7-day deploy outcome summary from the stats endpoint. */
function Deploy7dStrip() {
  const { data: stats } = useStats();
  if (!stats) return null;
  const { succeeded, failed, total, median_build_ms } = stats.deploy_7d;
  const rate = total > 0 ? Math.round((succeeded / total) * 100) : 0;
  return (
    <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
      <StatCard
        label="Success rate · 7d"
        tone={total === 0 ? "default" : rate === 100 ? "ready" : "attention"}
        value={total === 0 ? "—" : `${rate}%`}
        hint={
          total === 0
            ? "No deploys in 7 days"
            : `${succeeded}/${total} succeeded`
        }
      />
      <StatCard
        label="Deploys · 7d"
        value={total}
        hint="started in the last 7 days"
      />
      <StatCard
        label="Failed · 7d"
        tone={failed === 0 ? "ready" : "error"}
        value={failed}
        hint={failed === 0 ? "All clear" : "needs attention"}
      />
      <StatCard
        label="Median build · 7d"
        value={median_build_ms > 0 ? formatDuration(median_build_ms) : "—"}
        hint={median_build_ms > 0 ? "median build time" : "no completed builds"}
      />
    </div>
  );
}

function GlobalDeploymentsPage() {
  const [offset, setOffset] = useState(0);
  const [filters, setFilters] = useState<Filters>({
    range: "7d",
    search: "",
    status: "",
    projectId: "",
    applicationId: "",
  });

  const handleFilterChange = useCallback((f: Filters) => {
    setFilters(f);
    setOffset(0);
  }, []);

  const queryParams = useMemo(() => {
    const { from, to } = timeRangeToDates(filters.range);
    return {
      limit: PAGE_SIZE,
      offset,
      project_id: filters.projectId || undefined,
      application_id: filters.applicationId || undefined,
      status: filters.status || undefined,
      search: filters.search.trim() || undefined,
      from,
      to,
    };
  }, [filters, offset]);

  const { data: deployments, isLoading } = useGlobalDeployments(queryParams);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Deployments</h1>
        <p className="text-muted-foreground text-sm">
          All deployments across your applications.
        </p>
      </div>

      <Deploy7dStrip />

      <Card>
        <CardHeader>
          <CardTitle>Recent deployments</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <DeploymentFilters filters={filters} onChange={handleFilterChange} />
          <DataTable
            columns={deploymentColumns}
            data={deployments ?? []}
            isLoading={isLoading}
            getRowId={(d) => d.id}
            emptyMessage={
              offset > 0 ? "No more deployments." : "No deployments found."
            }
            pagination={{
              mode: "manual",
              offset,
              pageSize: PAGE_SIZE,
              hasMore: (deployments?.length ?? 0) === PAGE_SIZE,
              onOffsetChange: setOffset,
            }}
          />
        </CardContent>
      </Card>
    </div>
  );
}

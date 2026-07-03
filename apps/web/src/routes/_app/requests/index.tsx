import { createFileRoute } from "@tanstack/react-router";
import { useCallback, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  ActivityIcon,
  AppWindowIcon,
  ArrowRightIcon,
  BarChart3Icon,
  CircleCheckIcon,
  CircleXIcon,
  FolderIcon,
  GaugeIcon,
  ListFilterIcon,
  SearchIcon,
  TriangleAlertIcon,
} from "lucide-react";
import type { ColumnDef } from "@tanstack/react-table";
import { useRequestLogs, useRequestSummary } from "@/lib/hooks/use-request-logs";
import { useChannel } from "@/lib/hooks/use-websocket";
import { useProjects } from "@/lib/hooks/use-projects";
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
import { StatusBar, type StatusSegment } from "@/components/ui/status-bar";
import { Sparkline } from "@/components/ui/sparkline";
import { StatCard, MetricCard } from "@/lib/components/stats/stat-card";
import { PageHeader } from "@/components/ui/page-header";
import { LiveIndicator } from "@/components/ui/live-indicator";
import { TimeRangeTabs } from "@/components/ui/time-range-tabs";
import { timeRangeToDates, type TimeRange } from "@/lib/utils/time-range";
import { formatDateTime } from "@/lib/utils/format";
import { cn } from "@/lib/utils";
import type { RequestLog } from "@/lib/types";
import type { RequestSummary } from "@/lib/api/request-logs";
import * as applicationsApi from "@/lib/api/applications";

export const Route = createFileRoute("/_app/requests/")({
  component: GlobalRequestsPage,
});

const PAGE_SIZE = 100;

const STATUS_RANGES = [
  { label: "All statuses", value: "", Icon: ListFilterIcon },
  { label: "2xx", value: "2", Icon: CircleCheckIcon },
  { label: "3xx", value: "3", Icon: ArrowRightIcon },
  { label: "4xx", value: "4", Icon: TriangleAlertIcon },
  { label: "5xx", value: "5", Icon: CircleXIcon },
] as const;

interface Filters {
  range: TimeRange;
  search: string;
  statusRange: string;
  projectId: string;
  applicationId: string;
}

function statusRangeToMinMax(range: string): { min?: number; max?: number } {
  if (!range) return {};
  const base = parseInt(range) * 100;
  return { min: base, max: base + 100 };
}

/** Color a status code by its class (2xx ready · 3xx neutral · 4xx amber · 5xx red). */
function statusCodeClass(code: number) {
  if (code >= 500) return "bg-status-error-soft text-status-error";
  if (code >= 400) return "bg-status-building-soft text-status-building";
  if (code >= 300) return "bg-elev text-text-muted";
  return "bg-status-ready-soft text-status-ready";
}

/** Color an HTTP method by verb to match the design's method badges. */
function methodClass(method: string) {
  switch (method.toUpperCase()) {
    case "GET":
      return "bg-status-ready-soft text-status-ready";
    case "POST":
      return "bg-brand-soft text-brand";
    case "PUT":
    case "PATCH":
      return "bg-status-building-soft text-status-building";
    case "DELETE":
      return "bg-status-error-soft text-status-error";
    default:
      return "bg-elev text-text-muted";
  }
}

function RequestSummaryCards({ summary }: { summary: RequestSummary }) {
  const { class_counts, p50_ms, p95_ms, error_rate, per_minute, total } =
    summary;

  const segments: StatusSegment[] = [
    { label: "2xx", count: class_counts.c2xx, className: "bg-status-ready" },
    { label: "3xx", count: class_counts.c3xx, className: "bg-text-faint" },
    { label: "4xx", count: class_counts.c4xx, className: "bg-status-building" },
    { label: "5xx", count: class_counts.c5xx, className: "bg-status-error" },
  ];

  return (
    <div className="grid grid-cols-1 items-stretch gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <MetricCard
        label="Status breakdown"
        icon={<BarChart3Icon className="size-3.5" />}
      >
        <StatusBar segments={segments} className="mt-3" />
      </MetricCard>

      <StatCard
        label="Latency"
        icon={<GaugeIcon className="size-3.5" />}
        value={
          <span className="flex flex-wrap items-baseline gap-x-4 gap-y-1 font-mono">
            <span>
              {Math.round(p50_ms)}ms
              <span className="text-text-faint ml-1 text-xs font-normal">
                P50
              </span>
            </span>
            <span>
              {Math.round(p95_ms)}ms
              <span className="text-text-faint ml-1 text-xs font-normal">
                P95
              </span>
            </span>
          </span>
        }
        hint="median · 95th percentile"
      />

      <MetricCard label="Requests" icon={<ActivityIcon className="size-3.5" />}>
        <p className="mt-1 font-mono text-2xl font-semibold tracking-tight">
          {total.toLocaleString()}
        </p>
        <Sparkline
          className="mt-2"
          height={32}
          values={per_minute.map((p) => p.count)}
        />
      </MetricCard>

      <StatCard
        label="Error rate (5xx)"
        icon={<TriangleAlertIcon className="size-3.5" />}
        tone={error_rate > 0 ? "error" : "ready"}
        value={`${error_rate.toFixed(1)}%`}
        hint={error_rate > 0 ? "of responses are 5xx" : "no server errors"}
      />
    </div>
  );
}

function RequestFilters({
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
          placeholder="Path or client IP…"
          aria-label="Search requests"
          className="pl-9"
        />
      </div>

      <Select
        value={filters.statusRange}
        onValueChange={(v) => onChange({ ...filters, statusRange: v ?? "" })}
      >
        <SelectTrigger className="w-40 capitalize">
          <SelectValue placeholder="All statuses" />
        </SelectTrigger>
        <SelectContent>
          {STATUS_RANGES.map((r) => (
            <SelectItem
              key={r.value}
              value={r.value}
              icon={<r.Icon />}
              className="capitalize"
            >
              {r.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <Select
        items={projects?.map((p) => ({ label: p.name, value: p.id }))}
        value={filters.projectId}
        onValueChange={(v) =>
          onChange({ ...filters, projectId: v ?? "", applicationId: "" })
        }
      >
        <SelectTrigger className="w-40">
          <SelectValue placeholder="All Projects" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="" icon={<FolderIcon />}>
            All Projects
          </SelectItem>
          {projects?.map((p) => (
            <SelectItem key={p.id} value={p.id} icon={<FolderIcon />}>
              {p.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <Select
        items={applications?.map((a) => ({ label: a.name, value: a.id }))}
        value={filters.applicationId}
        onValueChange={(v) => onChange({ ...filters, applicationId: v ?? "" })}
        disabled={!filters.projectId}
      >
        <SelectTrigger className="w-44">
          <SelectValue placeholder="All Applications" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="" icon={<AppWindowIcon />}>
            All Applications
          </SelectItem>
          {applications?.map((a) => (
            <SelectItem key={a.id} value={a.id} icon={<AppWindowIcon />}>
              {a.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

const requestColumns: ColumnDef<RequestLog>[] = [
  {
    id: "status",
    header: "Status",
    cell: ({ row: { original: log } }) => (
      <span
        className={cn(
          "inline-block w-11 rounded-md py-0.5 text-center font-mono text-xs font-medium",
          statusCodeClass(log.status_code),
        )}
      >
        {log.status_code}
      </span>
    ),
  },
  {
    id: "method",
    header: "Method",
    cell: ({ row: { original: log } }) => (
      <span
        className={cn(
          "inline-block rounded-md px-1.5 py-0.5 font-mono text-xs font-medium",
          methodClass(log.method),
        )}
      >
        {log.method}
      </span>
    ),
  },
  {
    id: "path",
    header: "Path",
    meta: { className: "max-w-[20rem] truncate font-mono text-xs" },
    cell: ({ row: { original: log } }) => log.path,
  },
  {
    id: "host",
    header: "Host",
    meta: { className: "font-mono text-xs" },
    cell: ({ row: { original: log } }) => log.hostname,
  },
  {
    id: "client_ip",
    header: "Client IP",
    meta: { className: "font-mono text-xs" },
    cell: ({ row: { original: log } }) =>
      log.client_ip ?? <span className="text-text-faint">—</span>,
  },
  {
    id: "latency",
    header: "Latency",
    meta: { className: "text-text-muted whitespace-nowrap font-mono text-xs" },
    cell: ({ row: { original: log } }) => `${log.latency_ms}ms`,
  },
  {
    id: "time",
    header: "Time",
    meta: { className: "text-text-faint whitespace-nowrap font-mono text-xs" },
    cell: ({ row: { original: log } }) => formatDateTime(log.recorded_at),
  },
];

function GlobalRequestsPage() {
  const [liveEntries, setLiveEntries] = useState<RequestLog[]>([]);
  const [offset, setOffset] = useState(0);
  const [filters, setFilters] = useState<Filters>({
    range: "7d",
    search: "",
    statusRange: "",
    projectId: "",
    applicationId: "",
  });

  const queryParams = useMemo(() => {
    const { min, max } = statusRangeToMinMax(filters.statusRange);
    const { from, to } = timeRangeToDates(filters.range);
    return {
      limit: PAGE_SIZE,
      offset,
      application_id: filters.applicationId || undefined,
      status_min: min,
      status_max: max,
      search: filters.search.trim() || undefined,
      from,
      to,
    };
  }, [filters, offset]);

  const { data: history, isLoading } = useRequestLogs(queryParams);
  // Summary is window-scoped (not paginated); drop the offset so paging the
  // table doesn't refetch the aggregates.
  const { data: summary } = useRequestSummary({ ...queryParams, offset: 0 });

  const handleMessage = useCallback(
    (_event: string, data: unknown) => {
      try {
        const parsed = (
          typeof data === "string" ? JSON.parse(data) : data
        ) as RequestLog;
        const { min, max } = statusRangeToMinMax(filters.statusRange);
        if (min != null && parsed.status_code < min) return;
        if (max != null && parsed.status_code >= max) return;
        if (
          filters.applicationId &&
          parsed.application_id !== filters.applicationId
        )
          return;
        setLiveEntries((prev) => [parsed, ...prev].slice(0, 500));
      } catch {
        // ignore parse errors
      }
    },
    [filters.statusRange, filters.applicationId],
  );

  const { connected } = useChannel("requests:all", handleMessage);

  const handleFilterChange = useCallback((f: Filters) => {
    setFilters(f);
    setOffset(0);
  }, []);

  const allLogs = [...liveEntries, ...(history ?? [])];

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<ActivityIcon className="size-5" />}
        title="Requests"
        description="HTTP access logs across all applications."
        actions={<LiveIndicator active={connected} />}
      />

      {summary && <RequestSummaryCards summary={summary} />}

      <Card>
        <CardHeader>
          <CardTitle>Request logs</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <RequestFilters filters={filters} onChange={handleFilterChange} />
          <DataTable
            columns={requestColumns}
            data={allLogs}
            isLoading={isLoading}
            getRowId={(log) => log.id}
            emptyMessage={
              offset > 0 ? "No more request logs." : "No request logs found."
            }
            pagination={{
              mode: "manual",
              offset,
              pageSize: PAGE_SIZE,
              hasMore: (history?.length ?? 0) === PAGE_SIZE,
              onOffsetChange: setOffset,
            }}
          />
        </CardContent>
      </Card>
    </div>
  );
}

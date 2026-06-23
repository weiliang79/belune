import { createFileRoute } from "@tanstack/react-router";
import { useCallback, useMemo, useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { SearchIcon } from "lucide-react";
import { useRequestLogs, useRequestSummary } from "@/lib/hooks/use-request-logs";
import { useChannel } from "@/lib/hooks/use-websocket";
import { useProjects } from "@/lib/hooks/use-projects";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from "@/components/ui/table";
import { StatusBar, type StatusSegment } from "@/components/ui/status-bar";
import { Sparkline } from "@/components/ui/sparkline";
import { TimeRangeTabs } from "@/components/ui/time-range-tabs";
import { timeRangeToDates, type TimeRange } from "@/lib/utils/time-range";
import { cn } from "@/lib/utils";
import type { RequestLog } from "@/lib/types";
import type { RequestSummary } from "@/lib/api/request-logs";
import * as applicationsApi from "@/lib/api/applications";

export const Route = createFileRoute("/_app/requests/")({
  component: GlobalRequestsPage,
});

const PAGE_SIZE = 100;

const STATUS_RANGES = [
  { label: "All statuses", value: "" },
  { label: "2xx", value: "2" },
  { label: "3xx", value: "3" },
  { label: "4xx", value: "4" },
  { label: "5xx", value: "5" },
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
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
      <div className="bg-card rounded-xl border p-4">
        <p className="text-text-faint text-xs">Status breakdown</p>
        <StatusBar segments={segments} className="mt-3" />
      </div>

      <div className="bg-card rounded-xl border p-4">
        <p className="text-text-faint text-xs">Latency</p>
        <div className="mt-1 flex items-baseline gap-5">
          <span className="font-mono text-lg font-semibold">
            {Math.round(p50_ms)}ms
            <span className="text-text-faint ml-1 text-xs font-normal">P50</span>
          </span>
          <span className="font-mono text-lg font-semibold">
            {Math.round(p95_ms)}ms
            <span className="text-text-faint ml-1 text-xs font-normal">P95</span>
          </span>
        </div>
      </div>

      <div className="bg-card rounded-xl border p-4">
        <p className="text-text-faint text-xs">Requests</p>
        <p className="mt-1 font-mono text-lg font-semibold">
          {total.toLocaleString()}
        </p>
        <Sparkline
          className="mt-2"
          height={32}
          values={per_minute.map((p) => p.count)}
        />
      </div>

      <div className="bg-card rounded-xl border p-4">
        <p className="text-text-faint text-xs">Error rate (5xx)</p>
        <p
          className={cn(
            "mt-1 font-mono text-lg font-semibold",
            error_rate > 0 && "text-status-error",
          )}
        >
          {error_rate.toFixed(1)}%
        </p>
      </div>
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
        <SelectTrigger className="w-36">
          <SelectValue placeholder="All statuses" />
        </SelectTrigger>
        <SelectContent>
          {STATUS_RANGES.map((r) => (
            <SelectItem key={r.value} value={r.value}>
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
        items={applications?.map((a) => ({ label: a.name, value: a.id }))}
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

interface Column<T> {
  key: string;
  header: string;
  cell: (row: T) => ReactNode;
  className?: string;
}

const requestColumns: Column<RequestLog>[] = [
  {
    key: "status",
    header: "Status",
    cell: (log) => (
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
    key: "method",
    header: "Method",
    cell: (log) => (
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
    key: "path",
    header: "Path",
    className: "max-w-[20rem] truncate font-mono text-xs",
    cell: (log) => log.path,
  },
  {
    key: "host",
    header: "Host",
    className: "font-mono text-xs",
    cell: (log) => log.hostname,
  },
  {
    key: "client_ip",
    header: "Client IP",
    className: "font-mono text-xs",
    cell: (log) =>
      log.client_ip ?? <span className="text-text-faint">—</span>,
  },
  {
    key: "latency",
    header: "Latency",
    className: "text-text-muted whitespace-nowrap font-mono text-xs",
    cell: (log) => `${log.latency_ms}ms`,
  },
  {
    key: "time",
    header: "Time",
    className: "text-text-faint whitespace-nowrap font-mono text-xs",
    cell: (log) => new Date(log.recorded_at).toLocaleTimeString(),
  },
];

function RequestsTable({ logs }: { logs: RequestLog[] }) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          {requestColumns.map((c) => (
            <TableHead key={c.key}>{c.header}</TableHead>
          ))}
        </TableRow>
      </TableHeader>
      <TableBody>
        {logs.map((log) => (
          <TableRow key={log.id}>
            {requestColumns.map((c) => (
              <TableCell key={c.key} className={c.className}>
                {c.cell(log)}
              </TableCell>
            ))}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}

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
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Requests</h1>
          <p className="text-muted-foreground text-sm">
            HTTP access logs across all applications.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <span className="relative flex size-2">
            {connected && (
              <span className="bg-status-ready absolute inline-flex size-full animate-ping rounded-full opacity-75" />
            )}
            <span
              className={cn(
                "relative inline-flex size-2 rounded-full",
                connected ? "bg-status-ready" : "bg-text-faint",
              )}
            />
          </span>
          <span className="text-muted-foreground text-sm">
            {connected ? "Live" : "Disconnected"}
          </span>
        </div>
      </div>

      {summary && <RequestSummaryCards summary={summary} />}

      <RequestFilters filters={filters} onChange={handleFilterChange} />

      <div className="space-y-3">
        {isLoading ? (
          <div className="space-y-2">
            {[1, 2, 3, 4, 5].map((i) => (
              <div
                key={i}
                className="flex items-center gap-3 rounded-lg border px-4 py-3"
              >
                <Skeleton className="h-3 w-28" />
                <Skeleton className="h-5 w-12 rounded-full" />
                <Skeleton className="h-3 w-12" />
                <Skeleton className="h-3 flex-1" />
              </div>
            ))}
          </div>
        ) : allLogs.length === 0 ? (
          <div className="text-muted-foreground rounded-lg border py-12 text-center text-sm">
            {offset > 0 ? "No more request logs." : "No request logs found."}
          </div>
        ) : (
          <RequestsTable logs={allLogs} />
        )}

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
            {offset + 1}–{offset + (history?.length ?? 0)}
          </span>
          <Button
            variant="outline"
            size="sm"
            disabled={!history || history.length < PAGE_SIZE}
            onClick={() => setOffset(offset + PAGE_SIZE)}
          >
            Next
          </Button>
        </div>
      </div>
    </div>
  );
}

import { createFileRoute } from "@tanstack/react-router";
import { useCallback, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useRequestLogs } from "@/lib/hooks/use-request-logs";
import { useChannel } from "@/lib/hooks/use-websocket";
import { useProjects } from "@/lib/hooks/use-projects";
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
import { cn } from "@/lib/utils";
import type { RequestLog } from "@/lib/types";
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
  dateFrom?: string;
  dateTo?: string;
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

/** Percentile of an unsorted numeric array (0–100). */
function percentile(values: number[], p: number): number {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const idx = Math.min(sorted.length - 1, Math.floor((p / 100) * sorted.length));
  return sorted[idx];
}

function RequestSummary({ logs }: { logs: RequestLog[] }) {
  const stats = useMemo(() => {
    const total = logs.length;
    const errors = logs.filter((l) => l.status_code >= 500).length;
    const latencies = logs.map((l) => l.latency_ms);
    return {
      total,
      errorRate: total ? (errors / total) * 100 : 0,
      p50: percentile(latencies, 50),
      p95: percentile(latencies, 95),
    };
  }, [logs]);

  const cells = [
    { label: "Requests", value: stats.total.toLocaleString(), mono: true },
    {
      label: "Error rate (5xx)",
      value: `${stats.errorRate.toFixed(1)}%`,
      mono: true,
      tone: stats.errorRate > 0 ? "text-status-error" : undefined,
    },
    { label: "p50 latency", value: `${stats.p50}ms`, mono: true },
    { label: "p95 latency", value: `${stats.p95}ms`, mono: true },
  ];

  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
      {cells.map((c) => (
        <div key={c.label} className="bg-card rounded-xl border p-4">
          <p className="text-text-faint text-xs">{c.label}</p>
          <p
            className={cn(
              "mt-1 text-lg font-semibold",
              c.mono && "font-mono",
              c.tone,
            )}
          >
            {c.value}
          </p>
        </div>
      ))}
    </div>
  );
}

function RequestFilters({ filters, onChange }: {
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
        onChange={(range) => onChange({ ...filters, dateFrom: range.from, dateTo: range.to })}
        placeholder="All time"
        className="w-64"
      />

      <Select
        value={filters.statusRange}
        onValueChange={(v) => onChange({ ...filters, statusRange: v ?? "" })}
      >
        <SelectTrigger className="w-36">
          <SelectValue placeholder="All statuses" />
        </SelectTrigger>
        <SelectContent>
          {STATUS_RANGES.map((r) => (
            <SelectItem key={r.value} value={r.value}>{r.label}</SelectItem>
          ))}
        </SelectContent>
      </Select>

      <Select
        items={projects?.map((p) => ({
          label: p.name,
          value: p.id,
        }))}
        value={filters.projectId}
        onValueChange={(v) => onChange({ ...filters, projectId: v ?? "", applicationId: "" })}
      >
        <SelectTrigger className="w-40">
          <SelectValue placeholder="All projects" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="">All projects</SelectItem>
          {projects?.map((p) => (
            <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>
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
            <SelectItem key={a.id} value={a.id}>{a.name}</SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

function RequestRow({ log }: { log: RequestLog }) {
  return (
    <div className="hover:bg-card-hover flex items-center gap-3 rounded-lg border px-4 py-2.5 text-sm transition-colors">
      <span className="text-text-faint w-32 shrink-0 font-mono text-xs">
        {new Date(log.recorded_at).toLocaleTimeString()}
      </span>
      <span
        className={cn(
          "w-11 shrink-0 rounded-md py-0.5 text-center font-mono text-xs font-medium",
          statusCodeClass(log.status_code),
        )}
      >
        {log.status_code}
      </span>
      <span className="text-text-muted w-14 shrink-0 font-mono text-xs">
        {log.method}
      </span>
      <span className="min-w-0 flex-1 truncate font-mono text-xs">
        {log.hostname}{log.path}
      </span>
      <span className="text-text-faint shrink-0 font-mono text-xs">
        {log.latency_ms}ms
      </span>
    </div>
  );
}

function GlobalRequestsPage() {
  const [liveEntries, setLiveEntries] = useState<RequestLog[]>([]);
  const [offset, setOffset] = useState(0);
  const [filters, setFilters] = useState<Filters>({
    statusRange: "",
    projectId: "",
    applicationId: "",
  });

  const queryParams = useMemo(() => {
    const { min, max } = statusRangeToMinMax(filters.statusRange);
    return {
      limit: PAGE_SIZE,
      offset,
      application_id: filters.applicationId || undefined,
      status_min: min,
      status_max: max,
      from: filters.dateFrom,
      to: filters.dateTo,
    };
  }, [filters, offset]);

  const { data: history, isLoading } = useRequestLogs(queryParams);

  const handleMessage = useCallback((_event: string, data: unknown) => {
    try {
      const parsed = (typeof data === "string" ? JSON.parse(data) : data) as RequestLog;
      const { min, max } = statusRangeToMinMax(filters.statusRange);
      if (min != null && parsed.status_code < min) return;
      if (max != null && parsed.status_code >= max) return;
      if (filters.applicationId && parsed.application_id !== filters.applicationId) return;
      setLiveEntries((prev) => [parsed, ...prev].slice(0, 500));
    } catch {
      // ignore parse errors
    }
  }, [filters.statusRange, filters.applicationId]);

  const { connected } = useChannel("requests:all", handleMessage);

  const handleFilterChange = useCallback((f: Filters) => {
    setFilters(f);
    setOffset(0);
  }, []);

  const allLogs = [...liveEntries, ...(history ?? [])];

  return (
    <div className="space-y-6">
      <AppBreadcrumb items={[{ label: "Requests" }]} />
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

      {allLogs.length > 0 && <RequestSummary logs={allLogs} />}

      <RequestFilters filters={filters} onChange={handleFilterChange} />

      <div className="space-y-3">
        {isLoading ? (
          <div className="space-y-2">
            {[1, 2, 3, 4, 5].map((i) => (
              <div key={i} className="border rounded-lg flex items-center gap-3 px-4 py-3">
                <Skeleton className="h-3 w-28" />
                <Skeleton className="h-5 w-12 rounded-full" />
                <Skeleton className="h-3 w-12" />
                <Skeleton className="h-3 flex-1" />
              </div>
            ))}
          </div>
        ) : allLogs.length === 0 ? (
          <div className="border rounded-lg text-muted-foreground py-12 text-center text-sm">
            {offset > 0 ? "No more request logs." : "No request logs found."}
          </div>
        ) : (
          <div className="space-y-2">
            {allLogs.map((log) => <RequestRow key={log.id} log={log} />)}
          </div>
        )}

        <div className="flex items-center justify-between">
          <Button variant="outline" size="sm" disabled={offset === 0}
            onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}>
            Previous
          </Button>
          <span className="text-muted-foreground text-sm">
            {offset + 1}–{offset + (history?.length ?? 0)}
          </span>
          <Button variant="outline" size="sm"
            disabled={!history || history.length < PAGE_SIZE}
            onClick={() => setOffset(offset + PAGE_SIZE)}>
            Next
          </Button>
        </div>
      </div>
    </div>
  );
}

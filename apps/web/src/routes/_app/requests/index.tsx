import { createFileRoute } from "@tanstack/react-router";
import { useCallback, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useRequestLogs } from "@/lib/hooks/use-request-logs";
import { useSSEWithReconnect } from "@/lib/hooks/use-sse";
import { useProjects } from "@/lib/hooks/use-projects";
import { Badge } from "@/components/ui/badge";
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

function statusColor(code: number) {
  if (code >= 500) return "destructive" as const;
  if (code >= 400) return "outline" as const;
  if (code >= 300) return "secondary" as const;
  return "default" as const;
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
    <div className="border rounded-lg flex items-center gap-3 px-4 py-3 text-sm">
      <span className="text-muted-foreground w-36 shrink-0 font-mono text-xs">
        {new Date(log.recorded_at).toLocaleTimeString()}
      </span>
      <Badge variant={statusColor(log.status_code)} className="w-12 justify-center shrink-0">
        {log.status_code}
      </Badge>
      <span className="text-muted-foreground w-16 shrink-0 font-mono text-xs">
        {log.method}
      </span>
      <span className="min-w-0 flex-1 truncate font-mono text-xs">
        {log.hostname}{log.path}
      </span>
      <span className="text-muted-foreground shrink-0 text-xs">
        {log.latency_ms}ms
      </span>
    </div>
  );
}

function LiveRequestsView({ filters }: { filters: Filters }) {
  const [logs, setLogs] = useState<RequestLog[]>([]);

  const handleMessage = useCallback((data: string) => {
    try {
      const parsed = JSON.parse(data) as RequestLog;
      // Apply client-side status filter for live view
      const { min, max } = statusRangeToMinMax(filters.statusRange);
      if (min != null && parsed.status_code < min) return;
      if (max != null && parsed.status_code >= max) return;
      if (filters.applicationId && parsed.application_id !== filters.applicationId) return;
      setLogs((prev) => [parsed, ...prev].slice(0, 500));
    } catch {
      // ignore parse errors
    }
  }, [filters.statusRange, filters.applicationId]);

  const { connected } = useSSEWithReconnect("/api/requests/stream", handleMessage);

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <span className={`size-2 rounded-full ${connected ? "bg-green-500" : "bg-gray-400"}`} />
        <span className="text-muted-foreground text-sm">
          {connected ? "Live" : "Disconnected"} — showing up to 500 entries
        </span>
        <Button size="sm" variant="outline" onClick={() => setLogs([])}>
          Clear
        </Button>
      </div>
      {logs.length === 0 ? (
        <div className="border rounded-lg text-muted-foreground py-12 text-center text-sm">
          Waiting for requests...
        </div>
      ) : (
        <div className="space-y-2">
          {logs.map((log) => <RequestRow key={log.id} log={log} />)}
        </div>
      )}
    </div>
  );
}

function HistoryRequestsView({ filters }: { filters: Filters }) {
  const [offset, setOffset] = useState(0);

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

  const { data: logs, isLoading } = useRequestLogs(queryParams);

  return (
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
      ) : !logs || logs.length === 0 ? (
        <div className="border rounded-lg text-muted-foreground py-12 text-center text-sm">
          {offset > 0 ? "No more request logs." : "No request logs found."}
        </div>
      ) : (
        <div className="space-y-2">
          {logs.map((log) => <RequestRow key={log.id} log={log} />)}
        </div>
      )}

      <div className="flex items-center justify-between">
        <Button variant="outline" size="sm" disabled={offset === 0}
          onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}>
          Previous
        </Button>
        <span className="text-muted-foreground text-sm">
          {offset + 1}–{offset + (logs?.length ?? 0)}
        </span>
        <Button variant="outline" size="sm"
          disabled={!logs || logs.length < PAGE_SIZE}
          onClick={() => setOffset(offset + PAGE_SIZE)}>
          Next
        </Button>
      </div>
    </div>
  );
}

function GlobalRequestsPage() {
  const [mode, setMode] = useState<"history" | "live">("history");
  const [filters, setFilters] = useState<Filters>({
    statusRange: "",
    projectId: "",
    applicationId: "",
  });

  const handleFilterChange = useCallback((f: Filters) => {
    setFilters(f);
  }, []);

  return (
    <div className="space-y-6">
      <AppBreadcrumb items={[{ label: "Requests" }]} />
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Requests</h1>
          <p className="text-muted-foreground">HTTP access logs across all applications.</p>
        </div>
        <div className="flex gap-2">
          <Button size="sm" variant={mode === "history" ? "default" : "outline"}
            onClick={() => setMode("history")}>
            History
          </Button>
          <Button size="sm" variant={mode === "live" ? "default" : "outline"}
            onClick={() => setMode("live")}>
            Live
          </Button>
        </div>
      </div>

      <RequestFilters filters={filters} onChange={handleFilterChange} />

      {mode === "live"
        ? <LiveRequestsView filters={filters} />
        : <HistoryRequestsView filters={filters} />
      }
    </div>
  );
}

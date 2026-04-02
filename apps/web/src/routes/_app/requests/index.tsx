import { createFileRoute } from "@tanstack/react-router";
import { useCallback, useState } from "react";
import { useRequestLogs } from "@/lib/hooks/use-request-logs";
import { useSSEWithReconnect } from "@/lib/hooks/use-sse";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { AppBreadcrumb } from "@/lib/components/app-breadcrumb";
import { formatDate } from "@/lib/utils/format";
import type { RequestLog } from "@/lib/types";

export const Route = createFileRoute("/_app/requests/")({
  component: GlobalRequestsPage,
});

const PAGE_SIZE = 100;

function statusColor(code: number) {
  if (code >= 500) return "destructive" as const;
  if (code >= 400) return "outline" as const;
  if (code >= 300) return "secondary" as const;
  return "default" as const;
}

function RequestRow({ log }: { log: RequestLog }) {
  return (
    <div className="flex items-center gap-3 py-2 text-sm">
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

function LiveRequestsView() {
  const [logs, setLogs] = useState<RequestLog[]>([]);

  const handleMessage = useCallback((data: string) => {
    try {
      const parsed = JSON.parse(data) as RequestLog;
      setLogs((prev) => [parsed, ...prev].slice(0, 500));
    } catch {
      // ignore parse errors
    }
  }, []);

  const { connected } = useSSEWithReconnect(
    "/api/requests/stream",
    handleMessage,
  );

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <span
          className={`size-2 rounded-full ${connected ? "bg-green-500" : "bg-gray-400"}`}
        />
        <span className="text-muted-foreground text-sm">
          {connected ? "Live" : "Disconnected"} — showing up to 500 entries
        </span>
        <Button size="sm" variant="outline" onClick={() => setLogs([])}>
          Clear
        </Button>
      </div>
      <Card>
        <CardContent className="divide-y p-0 px-4">
          {logs.length === 0 ? (
            <p className="text-muted-foreground py-8 text-center text-sm">
              Waiting for requests...
            </p>
          ) : (
            logs.map((log) => <RequestRow key={log.id} log={log} />)
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function HistoryRequestsView() {
  const [offset, setOffset] = useState(0);
  const { data: logs, isLoading } = useRequestLogs({ limit: PAGE_SIZE, offset });

  return (
    <div className="space-y-3">
      {isLoading ? (
        <Card>
          <CardContent className="divide-y p-0 px-4">
            {[1, 2, 3, 4, 5].map((i) => (
              <div key={i} className="flex items-center gap-3 py-2">
                <Skeleton className="h-3 w-28" />
                <Skeleton className="h-5 w-12 rounded-full" />
                <Skeleton className="h-3 w-12" />
                <Skeleton className="h-3 flex-1" />
              </div>
            ))}
          </CardContent>
        </Card>
      ) : !logs || logs.length === 0 ? (
        <Card>
          <CardContent className="text-muted-foreground py-12 text-center">
            {offset > 0 ? "No more request logs." : "No request logs yet."}
          </CardContent>
        </Card>
      ) : (
        <Card>
          <CardContent className="divide-y p-0 px-4">
            {logs.map((log) => <RequestRow key={log.id} log={log} />)}
          </CardContent>
        </Card>
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
          {offset + 1}–{offset + (logs?.length ?? 0)}
        </span>
        <Button
          variant="outline"
          size="sm"
          disabled={!logs || logs.length < PAGE_SIZE}
          onClick={() => setOffset(offset + PAGE_SIZE)}
        >
          Next
        </Button>
      </div>
    </div>
  );
}

function GlobalRequestsPage() {
  const [mode, setMode] = useState<"history" | "live">("history");

  return (
    <div className="space-y-6">
      <AppBreadcrumb items={[{ label: "Requests" }]} />
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Requests</h1>
          <p className="text-muted-foreground">
            HTTP access logs across all applications.
          </p>
        </div>
        <div className="flex gap-2">
          <Button
            size="sm"
            variant={mode === "history" ? "default" : "outline"}
            onClick={() => setMode("history")}
          >
            History
          </Button>
          <Button
            size="sm"
            variant={mode === "live" ? "default" : "outline"}
            onClick={() => setMode("live")}
          >
            Live
          </Button>
        </div>
      </div>

      {mode === "live" ? <LiveRequestsView /> : <HistoryRequestsView />}
    </div>
  );
}

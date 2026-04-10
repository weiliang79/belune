import { createFileRoute } from "@tanstack/react-router";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { useChannel } from "@/lib/hooks/use-websocket";
import { useApplicationLogs } from "@/lib/hooks/use-application-logs";

export const Route = createFileRoute(
  "/_app/projects/$projectId/applications/$applicationId/logs",
)({
  component: LogsPage,
});

type LogEntry = {
  id: string;
  stream: "stdout" | "stderr";
  message: string;
  recorded_at: string | null;
};

function LogsPage() {
  const { projectId, applicationId } = Route.useParams();
  const [liveLogs, setLiveLogs] = useState<LogEntry[]>([]);
  const [follow, setFollow] = useState(true);
  const scrollRef = useRef<HTMLPreElement>(null);
  const liveIdRef = useRef(0);

  const { data: history, isLoading } = useApplicationLogs(
    projectId,
    applicationId,
    { limit: 500 },
  );

  const handleMessage = useCallback((_event: string, data: unknown) => {
    if (!data || typeof data !== "object") return;
    const obj = data as { stream?: string; message?: string };
    if (typeof obj.message !== "string") return;
    const stream = obj.stream === "stderr" ? "stderr" : "stdout";
    liveIdRef.current += 1;
    setLiveLogs((prev) =>
      [
        ...prev,
        {
          id: `live-${liveIdRef.current}`,
          stream,
          message: obj.message as string,
          recorded_at: new Date().toISOString(),
        },
      ].slice(-5000),
    );
  }, []);

  const { connected } = useChannel(`app-logs:${applicationId}`, handleMessage);

  // History is returned most-recent first; reverse to chronological, then append live.
  const entries = useMemo<LogEntry[]>(() => {
    const historical: LogEntry[] = history
      ? [...history].reverse().map((e) => ({
          id: e.id,
          stream: e.stream,
          message: e.message,
          recorded_at: e.recorded_at,
        }))
      : [];
    return [...historical, ...liveLogs];
  }, [history, liveLogs]);

  useEffect(() => {
    if (follow && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [entries, follow]);

  return (
    <div className="space-y-3 pt-3">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span
            className={`size-2 rounded-full ${connected ? "bg-green-500" : "bg-gray-400"}`}
          />
          <span className="text-muted-foreground text-sm">
            {connected ? "Connected" : "Disconnected"} · {entries.length} entries
          </span>
        </div>
        <div className="flex gap-2">
          <Button
            size="sm"
            variant={follow ? "default" : "outline"}
            onClick={() => setFollow(!follow)}
          >
            {follow ? "Following" : "Follow"}
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() => setLiveLogs([])}
          >
            Clear live
          </Button>
        </div>
      </div>

      <Card>
        <CardContent className="p-0">
          <pre
            ref={scrollRef}
            className="h-[600px] overflow-auto bg-zinc-950 p-4 font-mono text-xs text-zinc-200"
          >
            {isLoading ? (
              <span className="text-zinc-500">Loading logs...</span>
            ) : entries.length === 0 ? (
              <span className="text-zinc-500">Waiting for logs...</span>
            ) : (
              entries.map((entry) => (
                <div
                  key={entry.id}
                  className={`hover:bg-zinc-900 ${entry.stream === "stderr" ? "text-red-400" : ""}`}
                >
                  {entry.recorded_at && (
                    <span className="text-zinc-500">
                      {new Date(entry.recorded_at).toLocaleTimeString()}{" "}
                    </span>
                  )}
                  <span className="text-zinc-400">[{entry.stream}] </span>
                  {entry.message}
                </div>
              ))
            )}
          </pre>
        </CardContent>
      </Card>
    </div>
  );
}

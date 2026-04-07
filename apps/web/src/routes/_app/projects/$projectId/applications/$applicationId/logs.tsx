import { createFileRoute } from "@tanstack/react-router";
import { useCallback, useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useChannel } from "@/lib/hooks/use-websocket";
import { useApplicationLogs } from "@/lib/hooks/use-application-logs";

export const Route = createFileRoute(
  "/_app/projects/$projectId/applications/$applicationId/logs",
)({
  component: LogsPage,
});

function LogsPage() {
  const { projectId, applicationId } = Route.useParams();

  return (
    <Tabs defaultValue="live">
      <TabsList>
        <TabsTrigger value="live">Live</TabsTrigger>
        <TabsTrigger value="history">History</TabsTrigger>
      </TabsList>
      <TabsContent value="live">
        <LiveLogs projectId={projectId} applicationId={applicationId} />
      </TabsContent>
      <TabsContent value="history">
        <HistoryLogs projectId={projectId} applicationId={applicationId} />
      </TabsContent>
    </Tabs>
  );
}

function LiveLogs({
  projectId,
  applicationId,
}: {
  projectId: string;
  applicationId: string;
}) {
  const [logs, setLogs] = useState<string[]>([]);
  const [follow, setFollow] = useState(true);
  const scrollRef = useRef<HTMLPreElement>(null);

  const handleMessage = useCallback((_event: string, data: unknown) => {
    if (typeof data === "string") {
      setLogs((prev) => [...prev, data].slice(-5000));
    }
  }, []);

  const { connected } = useChannel(`app-logs:${applicationId}`, handleMessage);

  useEffect(() => {
    if (follow && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [logs, follow]);

  return (
    <div className="space-y-3 pt-3">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span
            className={`size-2 rounded-full ${connected ? "bg-green-500" : "bg-gray-400"}`}
          />
          <span className="text-muted-foreground text-sm">
            {connected ? "Connected" : "Disconnected"}
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
          <Button size="sm" variant="outline" onClick={() => setLogs([])}>
            Clear
          </Button>
        </div>
      </div>

      <Card>
        <CardContent className="p-0">
          <pre
            ref={scrollRef}
            className="h-[500px] overflow-auto bg-zinc-950 p-4 font-mono text-xs text-zinc-200"
          >
            {logs.length === 0 ? (
              <span className="text-zinc-500">Waiting for logs...</span>
            ) : (
              logs.map((line, i) => (
                <div key={i} className="hover:bg-zinc-900">
                  {line}
                </div>
              ))
            )}
          </pre>
        </CardContent>
      </Card>
    </div>
  );
}

function HistoryLogs({
  projectId,
  applicationId,
}: {
  projectId: string;
  applicationId: string;
}) {
  const { data: logs, isLoading, refetch } = useApplicationLogs(projectId, applicationId, { limit: 200 });

  return (
    <div className="space-y-3 pt-3">
      <div className="flex items-center justify-between">
        <span className="text-muted-foreground text-sm">
          {isLoading ? "Loading..." : `${logs?.length ?? 0} entries (most recent first)`}
        </span>
        <Button size="sm" variant="outline" onClick={() => refetch()}>
          Refresh
        </Button>
      </div>

      <Card>
        <CardContent className="p-0">
          <pre className="h-[500px] overflow-auto bg-zinc-950 p-4 font-mono text-xs text-zinc-200">
            {isLoading ? (
              <span className="text-zinc-500">Loading logs...</span>
            ) : !logs || logs.length === 0 ? (
              <span className="text-zinc-500">No historical logs found.</span>
            ) : (
              logs.map((entry) => (
                <div
                  key={entry.id}
                  className={`hover:bg-zinc-900 ${entry.stream === "stderr" ? "text-red-400" : ""}`}
                >
                  <span className="text-zinc-500">
                    {new Date(entry.recorded_at).toLocaleTimeString()}{" "}
                  </span>
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

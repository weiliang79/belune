import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";

export const Route = createFileRoute(
  "/_app/projects/$projectId/applications/$applicationId/logs",
)({
  component: LogsPage,
});

function LogsPage() {
  const { projectId, applicationId } = Route.useParams();
  const [logs, setLogs] = useState<string[]>([]);
  const [connected, setConnected] = useState(false);
  const [follow, setFollow] = useState(true);
  const scrollRef = useRef<HTMLPreElement>(null);
  const sourceRef = useRef<EventSource | null>(null);

  useEffect(() => {
    const url = `/api/projects/${projectId}/applications/${applicationId}/logs?follow=true`;
    const source = new EventSource(url);
    sourceRef.current = source;

    source.onopen = () => setConnected(true);
    source.onmessage = (event) => {
      setLogs((prev) => [...prev, event.data]);
    };
    source.onerror = () => {
      setConnected(false);
    };

    return () => {
      source.close();
      sourceRef.current = null;
    };
  }, [projectId, applicationId]);

  useEffect(() => {
    if (follow && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [logs, follow]);

  return (
    <div className="space-y-3">
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

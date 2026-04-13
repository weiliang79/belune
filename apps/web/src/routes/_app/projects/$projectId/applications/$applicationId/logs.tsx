import { createFileRoute } from "@tanstack/react-router";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { useChannel } from "@/lib/hooks/use-websocket";
import { useApplicationLogs } from "@/lib/hooks/use-application-logs";

type StreamFilter = "" | "stdout" | "stderr";

interface LogsSearch {
  q?: string;
  stream?: StreamFilter;
}

export const Route = createFileRoute(
  "/_app/projects/$projectId/applications/$applicationId/logs",
)({
  validateSearch: (search: Record<string, unknown>): LogsSearch => ({
    q: typeof search.q === "string" ? search.q : undefined,
    stream:
      search.stream === "stdout" || search.stream === "stderr"
        ? search.stream
        : undefined,
  }),
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
  const search = Route.useSearch();
  const navigate = Route.useNavigate();

  const [liveLogs, setLiveLogs] = useState<LogEntry[]>([]);
  const [follow, setFollow] = useState(true);
  const [inputValue, setInputValue] = useState(search.q ?? "");
  const scrollRef = useRef<HTMLPreElement>(null);
  const liveIdRef = useRef(0);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const streamFilter: StreamFilter = search.stream ?? "";

  // Debounce keyword search → URL param
  function handleSearchChange(value: string) {
    setInputValue(value);
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      navigate({
        search: (prev) => ({
          ...prev,
          q: value || undefined,
        }),
      });
    }, 400);
  }

  function handleStreamChange(value: StreamFilter) {
    navigate({
      search: (prev) => ({
        ...prev,
        stream: value || undefined,
      }),
    });
  }

  // Keep input in sync if URL changes externally (e.g. back/forward)
  useEffect(() => {
    setInputValue(search.q ?? "");
  }, [search.q]);

  const { data: history, isLoading, error } = useApplicationLogs(
    projectId,
    applicationId,
    {
      limit: 500,
      q: search.q,
      stream: search.stream,
    },
  );

  const handleMessage = useCallback((_event: string, data: unknown) => {
    if (!data || typeof data !== "object") return;
    const obj = data as { stream?: string; message?: string };
    if (typeof obj.message !== "string") return;
    const stream = obj.stream === "stderr" ? "stderr" : "stdout";

    // Apply live-side stream filter
    if (streamFilter && stream !== streamFilter) return;
    // Apply live-side keyword filter
    if (search.q && !obj.message.toLowerCase().includes(search.q.toLowerCase())) return;

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
  }, [streamFilter, search.q]);

  const { connected } = useChannel(`app-logs:${applicationId}`, handleMessage);

  // History is returned most-recent first; reverse to chronological, then append live.
  const entries = useMemo<LogEntry[]>(() => {
    const historical: LogEntry[] = history
      ? [...history].reverse().map((e) => ({
          id: e.id,
          stream: e.stream as "stdout" | "stderr",
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

  const streamTabs: { label: string; value: StreamFilter }[] = [
    { label: "All", value: "" },
    { label: "stdout", value: "stdout" },
    { label: "stderr", value: "stderr" },
  ];

  return (
    <div className="space-y-3 pt-3">
      {/* Toolbar */}
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-2">
          {/* Stream filter tabs */}
          <div className="flex rounded-md border text-sm">
            {streamTabs.map((tab) => (
              <button
                key={tab.value}
                onClick={() => handleStreamChange(tab.value)}
                className={`px-3 py-1 first:rounded-l-md last:rounded-r-md transition-colors ${
                  streamFilter === tab.value
                    ? "bg-primary text-primary-foreground"
                    : "text-muted-foreground hover:bg-muted"
                }`}
              >
                {tab.label}
              </button>
            ))}
          </div>

          {/* Keyword search */}
          <Input
            className="h-8 w-48 text-sm"
            placeholder="Search logs..."
            value={inputValue}
            onChange={(e) => handleSearchChange(e.target.value)}
          />
        </div>

        <div className="flex items-center gap-2">
          <span
            aria-hidden="true"
            className={`size-2 rounded-full ${connected ? "bg-green-500" : "bg-gray-400"}`}
          />
          <span className="text-muted-foreground text-sm">
            {connected ? "Connected" : "Disconnected"} · {entries.length}{" "}
            {entries.length === 1 ? "entry" : "entries"}
          </span>
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
            ) : error ? (
              <span className="text-red-400">
                Failed to load log history: {error.message}
              </span>
            ) : entries.length === 0 ? (
              <span className="text-zinc-500">
                {search.q || search.stream
                  ? "No logs match the current filter."
                  : "Waiting for logs..."}
              </span>
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

import { useEffect, useRef } from "react";
import {
  LEVEL_BG_CLASS,
  LEVEL_LABELS,
  LEVEL_TEXT_CLASS,
} from "@/lib/logs/level";
import { cn } from "@/lib/utils";
import { formatDateTime } from "@/lib/utils/format";
import type { LogEntry } from "./parse";

function LogLine({
  entry,
  showTimestamp,
  showLevel,
  wrap,
}: {
  entry: LogEntry;
  showTimestamp?: boolean;
  showLevel?: boolean;
  wrap?: boolean;
}) {
  // A session divider: a full-width labeled rule separating one deployment's
  // logs from the next in the merged ("all sessions") view.
  if (entry.divider !== undefined) {
    return (
      <div className="flex items-center gap-2 py-1 text-zinc-500 select-none">
        <span className="h-px flex-1 bg-zinc-700" />
        <span className="shrink-0 tracking-wide uppercase">{entry.divider}</span>
        <span className="h-px flex-1 bg-zinc-700" />
      </div>
    );
  }

  // When wrapping, hang-indent continuation lines so they align under the
  // message column (timestamp = 20 chars incl. trailing space, level = 10).
  const indentCh = (showTimestamp ? 20 : 0) + (showLevel ? 10 : 0);
  const hangingIndent =
    wrap && indentCh > 0
      ? { paddingLeft: `${indentCh}ch`, textIndent: `-${indentCh}ch` }
      : undefined;

  return (
    <div
      style={hangingIndent}
      className={cn(
        LEVEL_TEXT_CLASS[entry.level],
        LEVEL_BG_CLASS[entry.level],
        "hover:bg-white/5",
      )}
    >
      {showTimestamp && entry.recordedAt && (
        <span className="text-zinc-500">
          {formatDateTime(entry.recordedAt)}{" "}
        </span>
      )}
      {showLevel &&
        // Pad to a fixed width so the message column aligns across levels
        // ("[Warning]" is the widest label at 9 chars). The parent <pre>
        // preserves the padding spaces.
        `[${LEVEL_LABELS[entry.level]}]`.padEnd(10, " ")}
      {entry.message}
    </div>
  );
}

/**
 * Shared log surface: a scrollable dark terminal `<pre>` that renders leveled
 * entries with per-level coloring. Used by the container log viewer and by the
 * build / backup / restore blob viewers so every log reads consistently.
 *
 * When `follow` is true, it auto-scrolls to the bottom as entries change.
 */
export function LogView({
  entries,
  follow = true,
  showTimestamp = false,
  showLevel = false,
  wrap = false,
  isLoading = false,
  error,
  emptyMessage = "No logs.",
  className,
}: {
  entries: LogEntry[];
  follow?: boolean;
  showTimestamp?: boolean;
  showLevel?: boolean;
  wrap?: boolean;
  isLoading?: boolean;
  error?: string | null;
  emptyMessage?: string;
  className?: string;
}) {
  const scrollRef = useRef<HTMLPreElement>(null);

  useEffect(() => {
    if (follow && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [entries, follow]);

  return (
    <pre
      ref={scrollRef}
      className={cn(
        "bg-terminal-bg overflow-auto p-4 font-mono text-xs text-zinc-200",
        wrap ? "whitespace-pre-wrap [overflow-wrap:anywhere]" : "whitespace-pre",
        className,
      )}
    >
      {isLoading ? (
        <span className="text-zinc-500">Loading logs...</span>
      ) : error ? (
        <span className="text-terminal-err">{error}</span>
      ) : entries.length === 0 ? (
        <span className="text-zinc-500">{emptyMessage}</span>
      ) : (
        entries.map((entry) => (
          <LogLine
            key={entry.id}
            entry={entry}
            showTimestamp={showTimestamp}
            showLevel={showLevel}
            wrap={wrap}
          />
        ))
      )}
    </pre>
  );
}

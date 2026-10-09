import type { LogEntry } from "@/components/logs/parse";

// Merges the two sources the container log viewer reads from — the persisted
// history query and the live WebSocket buffer — into one chronological list
// with no line shown twice.
//
// Why this is not a concatenation: the collector publishes a line to Redis only
// AFTER its row is durable (`logcollector/collector.go`, "so a live viewer never
// sees a line that was not stored"). Every live line therefore already exists in
// the database, so any refetch of history — and history inherits the global 60s
// `staleTime` from `main.tsx`, so a tab-focus is enough — returns a second copy
// of lines the live buffer is still holding.
//
// The two copies cannot be matched on identity:
//
//   - not by `id`: history carries the row's UUID, while the live buffer mints
//     synthetic `live-N` ids that never equal one.
//   - not by `recordedAt` as a string: live publishes RFC3339Nano taken from
//     Docker's own nanosecond clock, while the stored value has been truncated
//     to Postgres's microsecond resolution and comes back re-formatted. The same
//     line reads `…123456789Z` live and `…123457Z` from history.
//
// So a line is identified by its content at millisecond resolution, which is
// the precision both representations survive down to. Two genuinely distinct
// lines that share a millisecond are kept; only an exact repeat of the same
// stream and message within one millisecond collapses, and that is a pair a
// reader could not have told apart on screen anyway. The opposite trade — a
// high-water mark on the newest historical timestamp — would instead discard
// *different* text that happened to land on the boundary instant, which is a
// silent loss rather than a cosmetic one.
function dedupeKey(entry: LogEntry): string | null {
  // A divider is positional furniture rather than a log line; it is inserted
  // after merging and must never be collapsed against anything.
  if (entry.divider !== undefined) return null;
  if (!entry.recordedAt) return null;
  const ms = Date.parse(entry.recordedAt);
  if (Number.isNaN(ms)) return null;
  return `${ms}|${entry.stream ?? ""}|${entry.message}`;
}

export function mergeLogEntries(
  historical: LogEntry[],
  live: LogEntry[],
): LogEntry[] {
  // History first, so when a line is present in both the surviving copy is the
  // persisted one — it carries the real row id, which is what React keys on.
  const combined = [...historical, ...live];

  const seen = new Set<string>();
  const unique: LogEntry[] = [];
  for (const entry of combined) {
    const key = dedupeKey(entry);
    if (key !== null) {
      if (seen.has(key)) continue;
      seen.add(key);
    }
    unique.push(entry);
  }

  // Dedupe alone usually restores order, because a live line that history does
  // not contain must be newer than everything history returned. That invariant
  // is not worth depending on: it breaks under clock skew between the collector
  // and the database, and under any filter mismatch between the server-side and
  // client-side passes. Sorting explicitly costs one pass and removes the
  // question.
  //
  // An entry with no parseable timestamp inherits the last one seen rather than
  // sorting to an edge, so it stays next to the line it arrived with instead of
  // being hoisted to the top of the view.
  let lastKnown = Number.NEGATIVE_INFINITY;
  const keyed = unique.map((entry, index) => {
    const parsed = entry.recordedAt ? Date.parse(entry.recordedAt) : Number.NaN;
    if (!Number.isNaN(parsed)) lastKnown = parsed;
    return { entry, index, at: lastKnown };
  });

  // Index breaks ties so the result is deterministic and history keeps its
  // place ahead of a live line recorded in the same millisecond.
  keyed.sort((a, b) => (a.at === b.at ? a.index - b.index : a.at - b.at));

  return keyed.map((k) => k.entry);
}

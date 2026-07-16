import { normalizeLevel, type LogLevel } from "@/lib/logs/level";

// A single renderable log line, normalized across every source (container logs
// from the DB, or NDJSON entries from a build/backup/restore run).
export interface LogEntry {
  id: string;
  level: LogLevel;
  message: string;
  stream?: string;
  recordedAt?: string | null;
}

// Shape of one NDJSON log entry. joblog (build/backup/restore) writes `ts` as an
// ISO string; Caddy writes it as a numeric epoch-seconds float (e.g. 1699564800.1).
interface RawEntry {
  ts?: string | number;
  level?: string;
  msg?: string;
}

// Normalize a raw `ts` to an ISO string. A bare number is epoch seconds, which
// `new Date()` would otherwise read as milliseconds and render as 1970. Anything
// past ~2001 in real milliseconds is ≥ 1e12, so treat smaller numbers as seconds.
function normalizeTs(ts: string | number | undefined): string | null {
  if (ts == null) return null;
  if (typeof ts === "number") {
    const ms = ts < 1e12 ? ts * 1000 : ts;
    const d = new Date(ms);
    return Number.isNaN(d.getTime()) ? null : d.toISOString();
  }
  return ts;
}

// Parses an NDJSON log blob (build / backup / restore logs) — one JSON object
// per line: {"ts","level","msg"}. A line that isn't valid JSON falls back to a
// plain info entry so stray/legacy content still renders.
export function parseLogBlob(blob: string, idPrefix = "blob"): LogEntry[] {
  if (!blob) return [];
  const out: LogEntry[] = [];
  const lines = blob.replace(/\r\n/g, "\n").split("\n");
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i].replace(/\r$/, "");
    if (!line) continue;
    out.push(parseLine(line, `${idPrefix}-${i}`));
  }
  return out;
}

function parseLine(line: string, id: string): LogEntry {
  try {
    const obj = JSON.parse(line) as RawEntry;
    if (obj && typeof obj.msg === "string") {
      return {
        id,
        level: normalizeLevel(obj.level ?? "info"),
        message: obj.msg,
        recordedAt: normalizeTs(obj.ts),
      };
    }
  } catch {
    // not JSON — fall through
  }
  return { id, level: "info", message: line };
}

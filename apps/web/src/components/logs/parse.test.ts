import { describe, expect, it } from "vitest";
import { parseLogBlob, stripAnsi } from "./parse";

describe("stripAnsi", () => {
  it("removes SGR colour sequences, keeping the visible text", () => {
    // A NestJS/winston-style line: green tag, reset, grey query marker.
    const line =
      "\x1b[32mNestWinston\x1b[39m \x1b[90mquery\x1b[39m listening on port 3000";
    expect(stripAnsi(line)).toBe("NestWinston query listening on port 3000");
  });

  it("leaves a line with no escape sequences untouched", () => {
    expect(stripAnsi("plain log line")).toBe("plain log line");
  });
});

// Belune's console handler (apps/api/internal/pkg/logger/console_handler.go)
// emits "<date> <time> <LEVEL> [module] <message>", with attributes on an
// indented continuation line. The platform log viewer reads that stream back
// through parseLogBlob, so these two must stay in step.
describe("parseLogBlob — console format", () => {
  it("reads the level off the line instead of defaulting to info", () => {
    const blob = [
      "2026-07-22 10:00:04 INFO  [worker.deploy] deploy finished",
      "2026-07-22 10:00:09 ERROR [proxy.caddy] reconcile failed",
      "2026-07-22 10:00:10 WARN  [eventwatcher] container restarting",
      "2026-07-22 10:00:11 DEBUG [store] query complete",
    ].join("\n");

    expect(parseLogBlob(blob).map((e) => e.level)).toEqual([
      "info",
      "error",
      "warning",
      "debug",
    ]);
  });

  // The reason the format needs an explicit level at all: keyword inference
  // would promote this line to Error on the word "failed".
  it("does not promote an info line that mentions a failure", () => {
    const [entry] = parseLogBlob(
      "2026-07-22 10:00:04 INFO  [worker.deploy] health check failed, retrying",
    );
    expect(entry.level).toBe("info");
  });

  it("keeps the module in the message", () => {
    const [entry] = parseLogBlob(
      "2026-07-22 10:00:04 INFO  [worker.deploy] deploy finished",
    );
    expect(entry.message).toBe("[worker.deploy] deploy finished");
  });

  it("folds an indented attribute line into the entry above", () => {
    const blob = [
      "2026-07-22 10:00:09 ERROR [proxy.caddy] reconcile failed",
      '                          app_id=10d7ec5f error="connection refused"',
    ].join("\n");

    const entries = parseLogBlob(blob);
    expect(entries).toHaveLength(1);
    expect(entries[0].level).toBe("error");
    expect(entries[0].message).toContain("reconcile failed");
    expect(entries[0].message).toContain("connection refused");
  });

  it("still folds continuation lines when Docker prefixes timestamps", () => {
    const blob = [
      "2026-07-22T10:00:09.100Z 2026-07-22 10:00:09 ERROR [proxy.caddy] reconcile failed",
      "2026-07-22T10:00:09.100Z                           error=refused",
      "2026-07-22T10:00:12.000Z 2026-07-22 10:00:12 INFO  [app] recovered",
    ].join("\n");

    const entries = parseLogBlob(blob);
    expect(entries).toHaveLength(2);
    expect(entries[0].level).toBe("error");
    expect(entries[0].message).toContain("error=refused");
    expect(entries[1].level).toBe("info");
    // Docker's timestamp wins over the one printed in the line.
    expect(entries[0].recordedAt).toBe("2026-07-22T10:00:09.100Z");
  });
});

// The console format is opt-out: LOG_FORMAT=json still exists, and stored blob
// logs (build/backup) have always been NDJSON.
describe("parseLogBlob — JSON format still works", () => {
  it("reads level and message from a JSON line", () => {
    const [entry] = parseLogBlob(
      '{"time":"2026-07-22T10:00:04Z","level":"ERROR","msg":"boom"}',
    );
    expect(entry.level).toBe("error");
    expect(entry.message).toBe("boom");
  });

  it("falls back to info for anything unrecognised", () => {
    const [entry] = parseLogBlob("some plain line from another container");
    expect(entry.level).toBe("info");
    expect(entry.message).toBe("some plain line from another container");
  });
});

// Colour is disabled whenever output is captured, but build tools colour
// theirs regardless — and a forced LOG_COLOR=always would too. Escape codes
// must not reach the rendered text or hide the level.
describe("parseLogBlob — ANSI colour", () => {
  it("strips colour and still reads the level", () => {
    const line =
      "\x1b[37m2026-07-22 10:00:09\x1b[0m \x1b[31mERROR\x1b[0m " +
      "\x1b[95m[proxy.caddy]\x1b[0m \x1b[31mreconcile failed\x1b[0m";
    const [entry] = parseLogBlob(line);
    expect(entry.level).toBe("error");
    expect(entry.message).toBe("[proxy.caddy] reconcile failed");
    expect(entry.message).not.toContain("\x1b");
  });

  it("strips colour from unrecognised lines too", () => {
    const [entry] = parseLogBlob("\x1b[32m=> building image\x1b[0m");
    expect(entry.message).toBe("=> building image");
  });
});

// The console handler pads continuation lines out to the width of the timestamp
// and level so they align in a terminal. The viewer renders timestamp and level
// as separate columns with whitespace-pre-wrap, so carrying that padding across
// threw the attributes ~26 characters into the message column.
describe("parseLogBlob — continuation indent", () => {
  it("re-indents a folded line instead of keeping the terminal padding", () => {
    const blob = [
      "2026-07-22 14:12:44 INFO  [store.db] database migrations applied",
      "                          version=56 dirty=false",
    ].join("\n");

    const [entry] = parseLogBlob(blob);
    const [, continuation] = entry.message.split("\n");
    expect(continuation).toBe("  version=56 dirty=false");
    expect(continuation.length).toBeLessThan(30);
  });

  it("normalises indent regardless of what the producer used", () => {
    // postgres and friends indent with a tab, not 26 spaces.
    const [entry] = parseLogBlob(["ERROR:  syntax error", "\t\tat character 8"].join("\n"));
    expect(entry.message.split("\n")[1]).toBe("  at character 8");
  });

  it("still keeps the continuation visually subordinate", () => {
    const [entry] = parseLogBlob(
      ["2026-07-22 14:12:44 ERROR [app] boom", "        error=nope"].join("\n"),
    );
    expect(entry.message.split("\n")[1].startsWith("  ")).toBe(true);
  });
});

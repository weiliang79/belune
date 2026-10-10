import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import type { ContainerLog } from "@/lib/types";
import { useContainerLogs } from "./use-container-logs";

vi.mock("@/lib/api/container-logs", () => ({
  listContainerLogs: vi.fn(),
}));
const { listContainerLogs } = await import("@/lib/api/container-logs");
const listMock = vi.mocked(listContainerLogs);

function row(message: string, recordedAt: string): ContainerLog {
  return {
    id: `row-${message}`,
    source_type: "application",
    source_id: "app-1",
    level: "info",
    stream: "stdout",
    message,
    recorded_at: recordedAt,
    deployment_id: null,
    container_id: "container-1",
  } as ContainerLog;
}

const DYING = [row("shutting down", "2026-10-09T06:00:00.000Z")];
const DYING_AND_STARTING = [
  row("listening on :3000", "2026-10-09T06:00:05.000Z"),
  row("shutting down", "2026-10-09T06:00:00.000Z"),
];

/** The viewer's history query, rendered so its newest line is readable. The
 *  endpoint answers newest-first, which is what the viewer reverses. */
function Probe() {
  const { data } = useContainerLogs("application", "proj-1", "app-1", {
    limit: 500,
  });
  return <div data-testid="newest">{data?.[0]?.message ?? "—"}</div>;
}

let client: QueryClient;

function wrap(children: ReactNode) {
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

beforeEach(() => {
  listMock.mockReset();
  // ⚠️ The same defaults main.tsx sets. The 60s staleTime is the whole point of
  // this file: a test with React Query's own defaults (staleTime 0) cannot
  // reproduce the bug, because every mount would refetch.
  client = new QueryClient({
    defaultOptions: { queries: { staleTime: 1000 * 60, retry: false } },
  });
});

afterEach(() => {
  client.clear();
});

describe("useContainerLogs freshness", () => {
  it("re-reads history when the viewer is remounted, even seconds later", async () => {
    // ⚠️ The bug this covers, found on a real stack: restart an application,
    // watch the logs go from dying to starting, navigate to another tab on the
    // application page and come back — and the starting logs are gone, until a
    // full page refresh brings them back.
    //
    // Two things combine. The live buffer is component state, so leaving the
    // route empties it; and the history query inherited main.tsx's 60s
    // staleTime, so React Query served the PREVIOUS fetch — taken before the
    // restart — and did not refetch. The viewer then had neither source for the
    // new container's lines: history was stale and live was empty. A refresh
    // worked because it bypasses the cache entirely.
    listMock.mockResolvedValue(DYING);
    const first = render(wrap(<Probe />));
    expect(await screen.findByText("shutting down")).toBeTruthy();
    expect(listMock).toHaveBeenCalledTimes(1);

    // The application restarts: the API would now answer with more.
    listMock.mockResolvedValue(DYING_AND_STARTING);

    // Leaving the Logs tab unmounts the viewer; coming back remounts it.
    first.unmount();
    render(wrap(<Probe />));

    expect(await screen.findByText("listening on :3000")).toBeTruthy();
    expect(
      listMock,
      "history was served from cache on remount, so the viewer showed logs from before the restart",
    ).toHaveBeenCalledTimes(2);
  });

  it("keeps showing the cached lines while the re-read is in flight", async () => {
    // The refetch must not blank the view: an empty log pane that fills in a
    // moment later reads as "the logs are gone".
    listMock.mockResolvedValue(DYING);
    const first = render(wrap(<Probe />));
    expect(await screen.findByText("shutting down")).toBeTruthy();

    let release: ((v: ContainerLog[]) => void) | undefined;
    listMock.mockReturnValue(
      new Promise<ContainerLog[]>((r) => {
        release = r;
      }),
    );

    first.unmount();
    render(wrap(<Probe />));

    // Still the old content, not a placeholder, while the request is pending.
    expect(screen.getByTestId("newest").textContent).toBe("shutting down");
    release?.(DYING_AND_STARTING);
    expect(await screen.findByText("listening on :3000")).toBeTruthy();
  });
});

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { VersionInfo } from "@/lib/api/version";
import { useAuthStore } from "@/lib/stores/auth";
import { resetUpdateProgress } from "@/lib/update-progress";
import { useSelfUpdateStatus } from "./use-maintenance";
import { useReloadOnVersionChange } from "./use-reload-on-version-change";

// Only the two edges are mocked. The progress state machine and the auth store
// are the real ones, driven through their own APIs, because the behaviour under
// test is the transition that machine reports.
vi.mock("@/lib/api/version", () => ({ getVersion: vi.fn() }));
vi.mock("@/lib/api/maintenance", () => ({
  getSelfUpdateStatus: vi.fn(),
  triggerSelfUpdate: vi.fn(),
}));
vi.mock("./use-websocket", () => ({
  useWebSocketStatus: () => "disconnected",
  useChannel: () => ({ connected: false }),
}));

const versionApi = await import("@/lib/api/version");
const maintenanceApi = await import("@/lib/api/maintenance");
const getVersion = vi.mocked(versionApi.getVersion);
const getSelfUpdateStatus = vi.mocked(maintenanceApi.getSelfUpdateStatus);

// The poll rate the hook uses while it believes an update is underway.
const LATCHED_POLL_MS = 2_000;

function wrapper(client: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    );
  };
}

function signIn(role: "admin" | "member") {
  useAuthStore.setState({
    isAuthenticated: true,
    user: {
      id: "u1",
      email: "a@b.c",
      role,
      username: "u",
      first_name: "A",
      last_name: "B",
    },
  });
}

// The hook under test plus the query whose refetch is the observable effect —
// which is how the Server page mounts them.
function useServerPage() {
  useReloadOnVersionChange();
  return useSelfUpdateStatus(true);
}

function version(updating: boolean): VersionInfo {
  return { version: "0.1.16", updating } as VersionInfo;
}

describe("useReloadOnVersionChange — reading the outcome when an update ends", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    resetUpdateProgress();
    useAuthStore.setState({ isAuthenticated: false, user: null });
    getSelfUpdateStatus.mockResolvedValue({ state: "idle" });
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("re-reads the update status the moment the update stops, with no quiet gap", async () => {
    // The defect: useSelfUpdateStatus polls at 5s and only while it ALREADY
    // reads "running", so when the server stopped saying "updating" the clock
    // vanished and the outcome arrived up to 5s later — the operator was told
    // "updating", then told nothing at all.
    signIn("admin");
    getVersion
      .mockResolvedValueOnce(version(true)) // latches: an update is underway
      .mockResolvedValue(version(false)); // the update has ended

    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    renderHook(useServerPage, { wrapper: wrapper(client) });

    // The mount read latches the tab.
    await vi.waitFor(() => expect(getVersion).toHaveBeenCalledTimes(1));
    const readsWhileLatched = getSelfUpdateStatus.mock.calls.length;

    // The latched poll notices the update has ended.
    await vi.advanceTimersByTimeAsync(LATCHED_POLL_MS + 100);
    await vi.waitFor(() => expect(getVersion).toHaveBeenCalledTimes(2));

    // The transition itself re-reads the attempt, rather than waiting for the
    // status query's own 5s interval.
    await vi.waitFor(() =>
      expect(getSelfUpdateStatus.mock.calls.length).toBeGreaterThan(
        readsWhileLatched,
      ),
    );
  });

  it("does not send a member at the admin-only status endpoint", async () => {
    // The endpoint is admin + session and this hook runs for every signed-in
    // user, so the refetch is gated on the role.
    //
    // The status query is mounted deliberately, even though a member's page
    // would not mount it: invalidateQueries only refetches ACTIVE queries, so
    // without an active one the call would be absent whether the gate exists or
    // not and this test would pass against a hook with no gate at all. With it
    // active, the gate is the only thing that can prevent the read. (Confirmed
    // by mutation: dropping `&& isAdmin` fails this test and nothing else.)
    signIn("member");
    getVersion
      .mockResolvedValueOnce(version(true))
      .mockResolvedValue(version(false));

    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    renderHook(useServerPage, { wrapper: wrapper(client) });

    await vi.waitFor(() => expect(getVersion).toHaveBeenCalledTimes(1));
    const readsWhileLatched = getSelfUpdateStatus.mock.calls.length;

    await vi.advanceTimersByTimeAsync(LATCHED_POLL_MS + 100);
    await vi.waitFor(() => expect(getVersion).toHaveBeenCalledTimes(2));
    // Give the transition the same chance to fire that the admin case takes.
    await vi.advanceTimersByTimeAsync(100);

    expect(getSelfUpdateStatus.mock.calls.length).toBe(readsWhileLatched);
  });

  it("does not re-read the status when no update was ever underway", async () => {
    // Guards against firing on every poll: the refetch is tied to a
    // latched → idle transition, not to merely reading "not updating".
    signIn("admin");
    getVersion.mockResolvedValue(version(false));

    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    renderHook(useServerPage, { wrapper: wrapper(client) });

    await vi.waitFor(() => expect(getVersion).toHaveBeenCalledTimes(1));
    const baseline = getSelfUpdateStatus.mock.calls.length;

    await vi.advanceTimersByTimeAsync(LATCHED_POLL_MS * 3);

    expect(getSelfUpdateStatus.mock.calls.length).toBe(baseline);
  });
});

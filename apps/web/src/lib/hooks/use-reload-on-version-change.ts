import { useEffect, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";
import * as versionApi from "@/lib/api/version";
import { useAuthStore } from "@/lib/stores/auth";
import {
  dispatchUpdateEvent,
  getUpdateProgress,
  isUpdateLatched,
} from "@/lib/update-progress";
import { queryKeys } from "./query-keys";
import { useChannel, useWebSocketStatus } from "./use-websocket";

/** How often to re-read /api/version while the tab is visible. It is a static
 *  string behind a public GET, so this is the cheapest request the app makes;
 *  30s bounds how long a stale tab can outlive an update. */
const POLL_MS = 30_000;

/** The poll rate while this tab believes an update is underway. Stage 3 (the
 *  API answering again) is detected by THIS, not by the socket reconnecting:
 *  the socket's backoff is 16-30s by the time a restart has lasted a minute,
 *  so it is slowest exactly when it is needed. A tiny public endpoint, and
 *  only for the length of an update. */
const LATCHED_POLL_MS = 2_000;

/** How long the "reloading…" screen is visible before the page reloads. Long
 *  enough to read, short enough that nobody submits a form into a stale API
 *  in the meantime. It is shown as a countdown, which is honest here because
 *  we control it. */
const RELOAD_DELAY_MS = 3_000;

/**
 * Reloads the page when the backend comes back on a different version.
 *
 * Self-update (v0.1.8) broke an assumption the rest of the app was built on:
 * that the running version cannot change while a page is open. It now can —
 * the dashboard replaces its own container — and without this the old bundle
 * keeps running against the new API until someone presses refresh. In that
 * window the sidebar reports the old version, the Updates card keeps offering
 * an update that already landed (and the API answers "already up to date"),
 * and any endpoint the new release changed can misbehave in ways that look
 * like the update failed.
 *
 * Three triggers, all funnelling into one idempotent check:
 *  - a slow poll while the tab is visible — the one that is guaranteed to
 *    exist on every page;
 *  - the tab becoming visible again — the operator tabbed away during the
 *    update and came back;
 *  - the WebSocket reporting "connected" — a fast path on the pages that
 *    have a socket (logs, metrics, deployments, requests).
 *
 * ⚠️ The socket is NOT a reliable signal on its own, and an earlier version of
 * this hook relied on it alone. It connects lazily, only when a component
 * subscribes to a channel, so on the landing page and on Server →
 * Configuration — where updates are triggered — there is no socket at all and
 * therefore no reconnect to observe. Found live: the hook never fired.
 *
 * It also drives the update-in-progress experience (see update-progress.ts):
 * /api/version reports an `updating` flag, the tab latches on it, and the API
 * going dark WHILE latched is what promotes "updating" to "restarting". The
 * `platform` channel only makes the first transition prompt, and is subscribed
 * here at the root for the reason above — a page-level subscription would leave
 * Server → Configuration, where the button is, with no socket. /api/version
 * stays the mechanism because it alone covers a tab opened mid-update, a
 * host-run update.sh and a client whose socket never came up. While latched the
 * poll quickens to LATCHED_POLL_MS.
 *
 * Only a DIFFERENT /api/version answer reloads. Mount once at the app root,
 * like useAccentSync. Covers a manual `update.sh` with the dashboard open
 * too, and every open tab reloads independently.
 *
 * ⚠️ The bundle doing the reloading is the OLD one, so the release that ships
 * this still shows a stale tab when updating INTO it. It pays off from the
 * release after.
 */
export function useReloadOnVersionChange() {
  const status = useWebSocketStatus();
  const qc = useQueryClient();
  const loadedVersion = useRef<string>("");
  const reloading = useRef(false);
  const inFlight = useRef(false);
  const lastCheck = useRef(0);
  const wasLatched = useRef(false);
  // Gates the update-status refetch below. Read here rather than inside the
  // check so it follows the same render as the closure that uses it.
  const isAdmin = useAuthStore((s) => s.user?.role === "admin");

  // One idempotent check shared by every trigger. Safe to call any number of
  // times: it reads, compares against the page's baseline, and either does
  // nothing or reloads exactly once.
  const checkRef = useRef<() => Promise<void>>(async () => {});
  checkRef.current = async () => {
    if (reloading.current || inFlight.current) return;
    inFlight.current = true;
    lastCheck.current = Date.now();
    let fresh: versionApi.VersionInfo;
    try {
      fresh = await versionApi.getVersion();
    } catch {
      // Mid-restart, or just a blip: only a tab already latched on "updating"
      // reads this as the restart, and only after several in a row.
      dispatchUpdateEvent({ type: "failure" });
      return;
    } finally {
      inFlight.current = false;
    }
    if (!fresh.version) return;

    // The first successful read is this page's baseline.
    if (!loadedVersion.current) {
      loadedVersion.current = fresh.version;
    } else if (
      fresh.version !== loadedVersion.current &&
      // A local Air rebuild reports "dev" before and after; never an update.
      fresh.version !== "dev" &&
      loadedVersion.current !== "dev"
    ) {
      reloading.current = true;
      dispatchUpdateEvent({
        type: "reload",
        version: fresh.version,
        reloadAt: Date.now() + RELOAD_DELAY_MS,
      });
      // Keep the sidebar and the Updates card honest for the seconds before
      // the reload — both read this key, and useVersion never refetches on its
      // own (staleTime: Infinity, which is correct WITHIN one page lifetime).
      qc.setQueryData(queryKeys.version, fresh);
      window.setTimeout(() => window.location.reload(), RELOAD_DELAY_MS);
      return;
    }

    dispatchUpdateEvent({
      type: "read",
      updating: fresh.updating,
      updatingFor: fresh.updating_for,
    });

    // Latched → idle means the update just ended. What became of it lives in the
    // admin-only update status, which polls at 5s and only while it already reads
    // "running" — so without this the clock vanishes the instant the server says
    // "not updating" and the outcome arrives up to 5s later, leaving a window
    // where the operator has been told "updating" and then told nothing at all.
    //
    // Safe to read it the moment we hear this: pullAndSpawnUpdateHelper's fail()
    // writes the reason BEFORE it broadcasts, so it is already persisted.
    //
    // ⚠️ Admin-gated. The endpoint is admin + session, and this hook runs for
    // every signed-in user. invalidateQueries only refetches ACTIVE queries, so a
    // Member who never mounted it would be a no-op anyway — but the gate is a
    // guarantee rather than an inference about library behaviour.
    //
    // Hooked to the transition, not to the socket event, so the 2s fallback poll
    // covers a client whose socket never came up.
    const latched = isUpdateLatched(getUpdateProgress());
    if (wasLatched.current && !latched && isAdmin) {
      void qc.invalidateQueries({
        queryKey: queryKeys.maintenanceUpdateStatus,
      });
    }
    wasLatched.current = latched;
  };

  // Baseline, poll, and tab-focus — the triggers that exist on every page.
  useEffect(() => {
    void checkRef.current();
    // Ticks at the fast rate always but only acts at the slow one unless
    // latched, so entering and leaving the latch needs no timer juggling.
    const tick = () => {
      if (document.hidden) return;
      const due = isUpdateLatched(getUpdateProgress())
        ? LATCHED_POLL_MS
        : POLL_MS;
      if (Date.now() - lastCheck.current >= due - 100) void checkRef.current();
    };
    const timer = window.setInterval(tick, LATCHED_POLL_MS);
    const onVisible = () => {
      if (!document.hidden) void checkRef.current();
    };
    document.addEventListener("visibilitychange", onVisible);
    return () => {
      window.clearInterval(timer);
      document.removeEventListener("visibilitychange", onVisible);
    };
  }, []);

  // The prompt half. An announce latches at once — including for the admin who
  // just clicked, on a page with no other socket — and the read that follows
  // confirms it and fetches the server's elapsed time.
  //
  // Only when signed in: the socket needs a session, so on /login it would be
  // refused and burn its retries. The HTTP poll still covers that page.
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  useChannel<{ updating?: boolean }>(
    isAuthenticated ? "platform" : null,
    (event, data) => {
      if (event !== "update") return;
      if (data?.updating) dispatchUpdateEvent({ type: "announce" });
      void checkRef.current();
    },
  );

  // Fast path: the socket coming (back) up means the API is serving. Harmless
  // on the first connection — the baseline is set by then and matches. The
  // other direction is only a prompt reason to ask: a close is NOT proof the
  // API is down (proxy hiccup, sleeping laptop, dropped wifi), so while latched
  // it probes, and the failure debounce decides whether that counts.
  useEffect(() => {
    if (status === "connected") void checkRef.current();
    else if (isUpdateLatched(getUpdateProgress())) void checkRef.current();
  }, [status]);
}

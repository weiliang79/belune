import { useEffect, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import * as versionApi from "@/lib/api/version";
import { queryKeys } from "./query-keys";
import { useWebSocketStatus } from "./use-websocket";

/** How often to re-read /api/version while the tab is visible. It is a static
 *  string behind a public GET, so this is the cheapest request the app makes;
 *  30s bounds how long a stale tab can outlive an update. */
const POLL_MS = 30_000;

/** How long the "reloading…" toast is visible before the page reloads. Long
 *  enough to read, short enough that nobody submits a form into a stale API
 *  in the meantime. */
const RELOAD_DELAY_MS = 2_500;

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

  // One idempotent check shared by every trigger. Safe to call any number of
  // times: it reads, compares against the page's baseline, and either does
  // nothing or reloads exactly once.
  const checkRef = useRef<() => Promise<void>>(async () => {});
  checkRef.current = async () => {
    if (reloading.current || inFlight.current) return;
    inFlight.current = true;
    let fresh: versionApi.VersionInfo;
    try {
      fresh = await versionApi.getVersion();
    } catch {
      return; // mid-restart; the next trigger will try again
    } finally {
      inFlight.current = false;
    }
    if (!fresh.version) return;

    // The first successful read is this page's baseline.
    if (!loadedVersion.current) {
      loadedVersion.current = fresh.version;
      return;
    }
    if (fresh.version === loadedVersion.current) return;
    // A local Air rebuild reports "dev" before and after; never an update.
    if (fresh.version === "dev" || loadedVersion.current === "dev") return;

    reloading.current = true;
    // Keep the sidebar and the Updates card honest for the ~2s before the
    // reload — both read this key, and useVersion never refetches on its own
    // (staleTime: Infinity, which is correct WITHIN one page lifetime).
    qc.setQueryData(queryKeys.version, fresh);
    toast.info(`Belune updated to ${fresh.version} — reloading…`, {
      duration: RELOAD_DELAY_MS,
    });
    window.setTimeout(() => window.location.reload(), RELOAD_DELAY_MS);
  };

  // Baseline, poll, and tab-focus — the triggers that exist on every page.
  useEffect(() => {
    void checkRef.current();
    const tick = () => {
      if (!document.hidden) void checkRef.current();
    };
    const timer = window.setInterval(tick, POLL_MS);
    document.addEventListener("visibilitychange", tick);
    return () => {
      window.clearInterval(timer);
      document.removeEventListener("visibilitychange", tick);
    };
  }, []);

  // Fast path: the socket coming (back) up means the API is serving. Harmless
  // on the first connection — the baseline is set by then and matches.
  useEffect(() => {
    if (status === "connected") void checkRef.current();
  }, [status]);
}

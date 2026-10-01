import { useSyncExternalStore } from "react";

/**
 * What this tab knows about a platform update in progress.
 *
 *  updating   — the server says an update is running and still answers. For
 *               most of an update that is the case (the pull and the backup
 *               happen with the API up), so this stage must not block anything.
 *  restarting — we were latched on "updating" and the API has stopped
 *               answering. Connection loss IS the phase transition, which is
 *               why nothing needs to report phases or predict a duration.
 *  reloading  — the API is back on a different version; the page reloads at
 *               `reloadAt`. The reload path itself predates this module.
 *
 * Kept as a module-level store, not React state, because the root hook that
 * drives it and the route error boundary that must react to it share no
 * component ancestry beyond the router.
 */
export type UpdateStage = "idle" | "updating" | "restarting" | "reloading";

export interface UpdateProgress {
  stage: UpdateStage;
  /** Wall-clock ms the update began, the earliest anchor seen. Null when idle. */
  since: number | null;
  /** Consecutive failed reads while latched; resets on any success. */
  failures: number;
  /** New version and reload deadline, only while reloading. */
  version: string | null;
  reloadAt: number | null;
}

/** How many failed reads in a row before "updating" becomes "restarting". At
 *  the 2s latched poll that is ~6s — long enough that a proxy hiccup, a
 *  sleeping laptop or a dropped wifi does not flash a blocking screen. */
export const FAILURES_BEFORE_RESTARTING = 3;

export const IDLE: UpdateProgress = {
  stage: "idle",
  since: null,
  failures: 0,
  version: null,
  reloadAt: null,
};

export type UpdateEvent =
  /** /api/version answered. */
  | { type: "read"; updating: boolean; updatingFor: number }
  /** /api/version failed (network error, 502 from the proxy, anything). */
  | { type: "failure" }
  /** The `platform` socket said an update started. */
  | { type: "announce" }
  | { type: "reload"; version: string; reloadAt: number };

/**
 * Pure transition function, so the stage rules are unit-tested rather than
 * verified by staging a real outage.
 */
export function nextProgress(
  state: UpdateProgress,
  event: UpdateEvent,
  now: number,
): UpdateProgress {
  if (state.stage === "reloading") return state; // terminal: the page is leaving

  switch (event.type) {
    case "reload":
      return {
        stage: "reloading",
        since: state.since,
        failures: 0,
        version: event.version,
        reloadAt: event.reloadAt,
      };

    case "announce":
      return state.stage === "idle"
        ? { ...IDLE, stage: "updating", since: now }
        : state;

    case "read": {
      // Not updating: either nothing is happening, or the update died and its
      // helper exited. Both unlatch — a failed update must not leave a toast on
      // every client forever.
      if (!event.updating) return IDLE;
      // Keep the EARLIEST anchor. The server's elapsed restarts when the pull
      // window hands over to the helper, and a counter that jumps backwards
      // reads as a bug.
      const anchor = now - event.updatingFor * 1000;
      return {
        ...IDLE,
        stage: "updating",
        since: state.since === null ? anchor : Math.min(state.since, anchor),
      };
    }

    case "failure": {
      // A failure with nothing latched is just a failure; only a latched tab
      // may read it as a restart.
      if (state.stage === "idle") return state;
      const failures = state.failures + 1;
      return {
        ...state,
        failures,
        stage:
          failures >= FAILURES_BEFORE_RESTARTING ? "restarting" : state.stage,
      };
    }
  }
}

/** "45s", "1m 20s", "1h 02m". Elapsed time only — never a prediction. */
export function formatElapsed(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  if (h > 0) return `${h}h ${String(m).padStart(2, "0")}m`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

let current: UpdateProgress = IDLE;
const listeners = new Set<() => void>();

export function getUpdateProgress(): UpdateProgress {
  return current;
}

export function dispatchUpdateEvent(event: UpdateEvent, now = Date.now()) {
  const next = nextProgress(current, event, now);
  // Field compare, not reference: a "read" rebuilds the object on every 2s poll
  // even when nothing changed, and subscribers should not re-render for that.
  if (
    next.stage === current.stage &&
    next.since === current.since &&
    next.failures === current.failures &&
    next.reloadAt === current.reloadAt
  ) {
    return;
  }
  current = next;
  for (const l of listeners) l();
}

/** Test seam: the store is module-level, so tests reset it between cases. */
export function resetUpdateProgress() {
  current = IDLE;
  for (const l of listeners) l();
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function useUpdateProgress(): UpdateProgress {
  return useSyncExternalStore(subscribe, getUpdateProgress);
}

/** True while this tab believes an update is underway, in any stage. */
export function isUpdateLatched(p: UpdateProgress): boolean {
  return p.stage !== "idle";
}

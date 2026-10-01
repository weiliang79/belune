import { beforeEach, describe, expect, it } from "vitest";
import {
  FAILURES_BEFORE_RESTARTING,
  IDLE,
  dispatchUpdateEvent,
  formatElapsed,
  getUpdateProgress,
  nextProgress,
  resetUpdateProgress,
  type UpdateEvent,
  type UpdateProgress,
} from "./update-progress";

const T0 = 1_000_000;

function run(events: UpdateEvent[], start: UpdateProgress = IDLE, now = T0) {
  return events.reduce((s, e) => nextProgress(s, e, now), start);
}

describe("nextProgress", () => {
  it("latches updating from a read that says so, anchored on the server's elapsed", () => {
    const s = nextProgress(
      IDLE,
      { type: "read", updating: true, updatingFor: 80 },
      T0,
    );
    expect(s.stage).toBe("updating");
    expect(s.since).toBe(T0 - 80_000);
  });

  it("an announce from the socket latches immediately", () => {
    const s = nextProgress(IDLE, { type: "announce" }, T0);
    expect(s).toMatchObject({ stage: "updating", since: T0 });
  });

  it("keeps the earliest anchor, so elapsed never jumps backwards", () => {
    // Pull window reports 120s, then the helper takes over and reports 5s.
    let s = nextProgress(
      IDLE,
      { type: "read", updating: true, updatingFor: 120 },
      T0,
    );
    s = nextProgress(
      s,
      { type: "read", updating: true, updatingFor: 5 },
      T0 + 2_000,
    );
    expect(s.since).toBe(T0 - 120_000);
  });

  it("ignores failures while nothing is latched", () => {
    const s = run(
      Array.from({ length: 10 }, () => ({ type: "failure" }) as const),
    );
    expect(s.stage).toBe("idle");
  });

  it("debounces: a blip does not become restarting", () => {
    const s = run([
      { type: "announce" },
      ...Array.from(
        { length: FAILURES_BEFORE_RESTARTING - 1 },
        () => ({ type: "failure" }) as const,
      ),
    ]);
    expect(s.stage).toBe("updating");
  });

  it("consecutive failures while latched become restarting", () => {
    const s = run([
      { type: "announce" },
      ...Array.from(
        { length: FAILURES_BEFORE_RESTARTING },
        () => ({ type: "failure" }) as const,
      ),
    ]);
    expect(s.stage).toBe("restarting");
  });

  it("a success in between resets the failure count", () => {
    const s = run([
      { type: "announce" },
      { type: "failure" },
      { type: "failure" },
      { type: "read", updating: true, updatingFor: 1 },
      { type: "failure" },
      { type: "failure" },
    ]);
    expect(s.stage).toBe("updating");
  });

  it("returns to updating if the API answers again still updating", () => {
    const s = run([
      { type: "announce" },
      ...Array.from(
        { length: FAILURES_BEFORE_RESTARTING },
        () => ({ type: "failure" }) as const,
      ),
      { type: "read", updating: true, updatingFor: 90 },
    ]);
    expect(s.stage).toBe("updating");
  });

  it("unlatches when the server says it is no longer updating — a failed update self-clears", () => {
    const s = run([
      { type: "announce" },
      { type: "read", updating: false, updatingFor: 0 },
    ]);
    expect(s).toEqual(IDLE);
  });

  it("unlatches from restarting too (restart came back on the same version)", () => {
    const s = run([
      { type: "announce" },
      ...Array.from(
        { length: FAILURES_BEFORE_RESTARTING },
        () => ({ type: "failure" }) as const,
      ),
      { type: "read", updating: false, updatingFor: 0 },
    ]);
    expect(s.stage).toBe("idle");
  });

  it("reloading is terminal", () => {
    const s = run([
      { type: "reload", version: "v2", reloadAt: T0 + 3000 },
      { type: "read", updating: false, updatingFor: 0 },
      { type: "failure" },
    ]);
    expect(s).toMatchObject({
      stage: "reloading",
      version: "v2",
      reloadAt: T0 + 3000,
    });
  });
});

describe("store", () => {
  beforeEach(resetUpdateProgress);

  it("does not change identity for a poll that changes nothing", () => {
    dispatchUpdateEvent({ type: "read", updating: true, updatingFor: 10 }, T0);
    const before = getUpdateProgress();
    dispatchUpdateEvent(
      { type: "read", updating: true, updatingFor: 12 },
      T0 + 2_000,
    );
    expect(getUpdateProgress()).toBe(before);
  });
});

describe("formatElapsed", () => {
  it.each([
    [0, "0s"],
    [45_000, "45s"],
    [80_000, "1m 20s"],
    [3_720_000, "1h 02m"],
    [-5, "0s"],
  ])("%d ms -> %s", (ms, want) => {
    expect(formatElapsed(ms)).toBe(want);
  });
});

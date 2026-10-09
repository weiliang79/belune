import { describe, expect, it } from "vitest";
import { dashboardTLSPollMs } from "./dashboard-tls-poll";

// Every value internal/tlsstatus/probe.go can produce, transcribed so a status
// added there without a decision here shows up as a test that was never
// written rather than as silence.
const ALL_STATUSES = [
  "unknown",
  "disabled",
  "pending",
  "active",
  "expiring",
  "expired",
  "failed",
  "local",
] as const;

describe("dashboardTLSPollMs — which statuses are watched", () => {
  it("polls only while the certificate can still change on its own", () => {
    const polled = ALL_STATUSES.filter(
      (s) => dashboardTLSPollMs(s, 0) !== false,
    );
    expect(polled).toEqual(["unknown", "pending", "failed"]);
  });

  it("stops once the certificate is active", () => {
    // The regression: this polled for ever, re-probing a certificate that
    // could not change, on the page an operator leaves open during an update.
    expect(dashboardTLSPollMs("active", 0)).toBe(false);
  });

  it("keeps watching a failure, because Caddy retries ACME by itself", () => {
    expect(dashboardTLSPollMs("failed", 0)).toBe(15_000);
  });

  it("does not poll before the first answer has arrived", () => {
    // The query has no data yet. Polling on undefined would start a loop
    // against an endpoint that has not answered once.
    expect(dashboardTLSPollMs(undefined, 0)).toBe(false);
  });

  it("stops on a status it does not recognise", () => {
    // A new server-side status must not default into a forever-poll.
    expect(dashboardTLSPollMs("renewing", 0)).toBe(false);
    expect(dashboardTLSPollMs("", 0)).toBe(false);
  });
});

describe("dashboardTLSPollMs — and for how long", () => {
  it("gives up after about ten minutes of watching", () => {
    // The case the status check alone does not fix: a dashboard domain whose
    // DNS is wrong sits at pending or failed indefinitely, so "poll while
    // unsettled" is still a forever-poll for the install that can least
    // afford it.
    expect(dashboardTLSPollMs("pending", 39)).toBe(15_000);
    expect(dashboardTLSPollMs("pending", 40)).toBe(false);
    expect(dashboardTLSPollMs("failed", 40)).toBe(false);
    expect(dashboardTLSPollMs("unknown", 40)).toBe(false);
  });

  it("bounds the whole thing at well under one request a minute on average", () => {
    // 40 polls at 15s is 10 minutes, after which it is zero — so an hour with
    // the tab open costs 40 requests, not 240.
    let polls = 0;
    let total = 0;
    while (dashboardTLSPollMs("pending", polls) !== false) {
      polls++;
      total++;
      if (total > 1_000) throw new Error("dashboardTLSPollMs never gives up");
    }
    expect(total).toBe(40);
    expect((total * 15_000) / 60_000).toBe(10); // minutes
  });

  it("is unaffected by the count once the status has settled", () => {
    expect(dashboardTLSPollMs("active", 0)).toBe(false);
    expect(dashboardTLSPollMs("active", 1_000)).toBe(false);
  });
});

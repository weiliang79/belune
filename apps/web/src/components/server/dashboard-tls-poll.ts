/** Roughly how long to keep watching a certificate that has not settled, in
 *  polls at POLL_MS. Ten minutes: long enough to cover DNS propagating and
 *  Caddy's first few ACME attempts, short enough that a dashboard domain whose
 *  DNS is simply wrong does not poll for the rest of the day. */
const MAX_UNSETTLED_POLLS = 40;

const POLL_MS = 15_000;

/** The statuses something server-side can still change on its own.
 *
 *  `pending` is a certificate being obtained, which legitimately takes minutes.
 *  `failed` is included because Caddy retries ACME in the background with its
 *  own backoff, so a failed domain really can become active with nobody
 *  touching the page. `unknown` is a row waiting on its first probe, which the
 *  TLS sweep resolves within a minute.
 *
 *  active / expiring / expired / disabled / local are settled: nothing will
 *  change them while the page is open, and expiry is days away, not minutes. */
const UNSETTLED = ["pending", "failed", "unknown"];

/**
 * How often to re-read the dashboard's TLS status, or false to stop.
 *
 * ⚠️ Only while it is still settling, and only for a while. Keyed on "is a
 * domain saved" this polled for as long as the tab stayed open — 4 requests a
 * minute, for ever, re-probing a certificate that was already active and could
 * not change — which on Server → Configuration was a seventh of the whole
 * measured page cost, on the page where an operator leaves a tab open watching
 * an update. Dropping the status check alone would have fixed the healthy case
 * and left the broken one: a dashboard domain whose DNS is wrong sits at
 * pending or failed indefinitely, which is where a forever-poll hurts most.
 *
 * So both: the status has to be one that can still change, AND the page has to
 * have been watching for less than MAX_UNSETTLED_POLLS. After that the card's
 * Recheck button is the way to ask again, which is the right shape for a
 * question nobody is actively waiting on.
 *
 * ⚠️ An UNRECOGNISED status stops polling rather than starting it. A status
 * added server-side is far more likely to be another settled one, and the
 * failure modes are not symmetric: the wrong default here costs a stale badge
 * until the operator presses Recheck, where the wrong default the other way
 * costs every open tab 4 requests a minute, which is how this got here.
 *
 * @param status  tls_status from the last answer, or undefined before the first
 * @param polls   successful fetches so far — React Query's dataUpdateCount
 */
export function dashboardTLSPollMs(
  status: string | undefined,
  polls: number,
): number | false {
  if (status === undefined || !UNSETTLED.includes(status)) return false;
  return polls < MAX_UNSETTLED_POLLS ? POLL_MS : false;
}

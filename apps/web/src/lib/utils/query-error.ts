import { ApiError } from "@/lib/api/client";

/**
 * Whether a thrown query error means "the resource is genuinely gone" rather
 * than "we could not reach the answer".
 *
 * The distinction is the whole point: a 404 is final and retrying it is
 * pointless, while a restarting API, a dropped connection or a 500 is a
 * transient failure where retrying is the entire remedy. Telling a reader
 * their project does not exist because the API was briefly down is the bug
 * this exists to prevent — and `update.sh` restarts the API on every upgrade,
 * so that window is routine, not exotic.
 *
 * Extracted as a predicate so the rule is unit-tested rather than only
 * verified by clicking through a failure that is awkward to stage.
 */
export function isNotFoundError(error: unknown): boolean {
  return error instanceof ApiError && error.status === 404;
}

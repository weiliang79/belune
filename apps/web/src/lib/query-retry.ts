import { ApiError } from "@/lib/api/client";

/**
 * Whether React Query should retry a failed read.
 *
 * ⚠️ Never on a 4xx. A 400, 403, 404 or 409 cannot change by being asked
 * again, so a retry only delays the error the caller is already going to see —
 * and on a **429** it is actively harmful: it spends another request from a
 * bucket that is by definition already empty, so one rate-limited page becomes
 * two and the window the operator has to wait out gets longer. That mattered
 * here, because a dashboard tab's own polling is what fills that bucket (see
 * sessionRateLimit in internal/server/routes.go).
 *
 * 401 is already handled inside the API client, which refreshes the session and
 * replays the request once; by the time a 401 surfaces to React Query the
 * refresh has failed too, so it is final.
 *
 * 5xx and network failures keep the single retry — those are the ones a second
 * attempt can genuinely win, and the one that made the global `retry: 1`
 * worth having.
 */
export function shouldRetryQuery(
  failureCount: number,
  error: unknown,
): boolean {
  if (error instanceof ApiError && error.status >= 400 && error.status < 500) {
    return false;
  }
  return failureCount < 1;
}

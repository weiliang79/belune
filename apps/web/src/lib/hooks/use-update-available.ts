import { useQuery } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as settingsApi from "@/lib/api/settings";
import { useVersion } from "./use-version";

/** "v0.1.7" / "0.1.7" compare the same — the manifest omits the prefix, the
 *  binary's own version (ldflags-stamped) carries it. */
function stripV(v: string) {
  return v.startsWith("v") ? v.slice(1) : v;
}

/** Numeric MAJOR.MINOR.PATCH compare — every published version is this shape,
 *  because the release workflow only registers stable tags in versions.json
 *  (prereleases are skipped), so nothing here needs pre-release ordering. */
function compareVersions(a: string, b: string): number {
  const pa = stripV(a)
    .split(".")
    .map((n) => parseInt(n, 10) || 0);
  const pb = stripV(b)
    .split(".")
    .map((n) => parseInt(n, 10) || 0);
  for (let i = 0; i < Math.max(pa.length, pb.length); i++) {
    const diff = (pa[i] ?? 0) - (pb[i] ?? 0);
    if (diff !== 0) return diff;
  }
  return 0;
}

/**
 * Whether a newer Belune release is available, plus the cached facts about it.
 *
 * Shared deliberately. The Server-page card and the sidebar's dot must agree —
 * a dot that keeps showing after the operator hit "Skip this version", or that
 * disagrees with the card one click away, is worse than no dot. One comparison,
 * one place to be correct.
 *
 * ⚠️ `enabled` is the caller's, and must be false for non-admins: the update
 * cache lives in `GET /api/settings`, which is admin + session, so a Member
 * would 403 on every render of whatever mounts this — and the sidebar mounts on
 * every page.
 *
 * Availability is re-derived from the LIVE running version rather than read
 * from a stored boolean, so a manual `update.sh` clears it immediately instead
 * of leaving a stale banner up until the next daily check.
 */
export function useUpdateAvailable(enabled: boolean) {
  const currentVersion = useVersion();
  const { data: settings } = useQuery({
    queryKey: queryKeys.settings,
    queryFn: settingsApi.getSettings,
    enabled,
  });

  const setting = (key: string) =>
    settings?.find((s) => s.key === key)?.value ?? "";

  const latestVersion = setting("update_latest_version");
  const skipVersion = setting("update_skip_version");

  // An unstamped local build ("dev") or a page still loading /api/version has
  // nothing meaningful to compare against.
  const validCurrent = Boolean(currentVersion) && currentVersion !== "dev";

  return {
    available:
      validCurrent &&
      Boolean(latestVersion) &&
      compareVersions(latestVersion, currentVersion) > 0 &&
      latestVersion !== skipVersion,
    validCurrent,
    currentVersion,
    latestVersion,
    skipVersion,
    breaking: setting("update_latest_breaking") === "true",
    requiresHostUpdate:
      setting("update_latest_requires_host_update") === "true",
    notesUrl: setting("update_latest_notes_url"),
    lastCheckedAt: setting("update_last_checked_at"),
    checkEnabled: setting("update_check_enabled") !== "false",
  };
}

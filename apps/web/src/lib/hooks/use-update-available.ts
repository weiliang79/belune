import { useQuery } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as settingsApi from "@/lib/api/settings";
import { useVersion } from "./use-version";

/** "v0.1.7" / "0.1.7" compare the same — the manifest omits the prefix, the
 *  binary's own version (ldflags-stamped) carries it. */
function stripV(v: string) {
  return v.startsWith("v") ? v.slice(1) : v;
}

/**
 * Semver 2.0 precedence, matching `golang.org/x/mod/semver` on the backend.
 *
 * ⚠️ The two MUST agree. The card and the sidebar dot decide "update available"
 * here; TriggerSelfUpdate decides "already up to date" with the Go library. If
 * they diverge, the UI offers an update the API refuses — or, as actually
 * happened on the first real self-update, hides one the API would accept.
 *
 * The previous version parsed each dotted part with parseInt, which silently
 * turned "8-rc1" and "8-rc2" into the same 8, so a newer prerelease read as
 * "up to date". The stable release path never hit it because versions.json
 * only registers stable tags — but any prerelease→prerelease update does.
 *
 * Rules (semver.org §11), reproduced rather than imported so there is no new
 * dependency for ~30 lines: compare MAJOR.MINOR.PATCH numerically; a version
 * WITH a pre-release is lower than the same version without; pre-release
 * identifiers compare left to right, numeric ones numerically, others
 * lexically, numeric lower than alphanumeric, and a shorter identifier list is
 * lower when it is a prefix of the longer. Build metadata (+…) is ignored.
 *
 * ⚠️ Consequence worth knowing: "rc10" < "rc2", because both are alphanumeric
 * and compare lexically. That is what Go does too, so it is correct here even
 * though it looks wrong — use "rc.10" if numeric ordering is ever wanted.
 */
export function compareVersions(a: string, b: string): number {
  const [aCore, aPre] = splitPre(stripV(a));
  const [bCore, bPre] = splitPre(stripV(b));

  const pa = aCore.split(".").map((n) => parseInt(n, 10) || 0);
  const pb = bCore.split(".").map((n) => parseInt(n, 10) || 0);
  for (let i = 0; i < Math.max(pa.length, pb.length); i++) {
    const diff = (pa[i] ?? 0) - (pb[i] ?? 0);
    if (diff !== 0) return diff;
  }

  // Same core. No pre-release outranks any pre-release.
  if (aPre === "" && bPre === "") return 0;
  if (aPre === "") return 1;
  if (bPre === "") return -1;

  const ia = aPre.split(".");
  const ib = bPre.split(".");
  for (let i = 0; i < Math.min(ia.length, ib.length); i++) {
    const x = ia[i] ?? "";
    const y = ib[i] ?? "";
    const xn = /^\d+$/.test(x);
    const yn = /^\d+$/.test(y);
    if (xn && yn) {
      const diff = parseInt(x, 10) - parseInt(y, 10);
      if (diff !== 0) return diff;
    } else if (xn !== yn) {
      return xn ? -1 : 1;
    } else if (x !== y) {
      return x < y ? -1 : 1;
    }
  }
  return ia.length - ib.length;
}

/** "0.1.8-rc2+build7" → ["0.1.8", "rc2"]. Build metadata never affects order. */
function splitPre(v: string): [string, string] {
  const noBuild = v.split("+")[0] ?? v;
  const dash = noBuild.indexOf("-");
  return dash === -1
    ? [noBuild, ""]
    : [noBuild.slice(0, dash), noBuild.slice(dash + 1)];
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

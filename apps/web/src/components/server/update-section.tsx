import { useState } from "react";
import { ExternalLinkIcon } from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { CopyButton } from "@/lib/components/copy-button";
import { useSettings, useUpdateSettings } from "@/lib/hooks/use-settings";
import { useTriggerSelfUpdate } from "@/lib/hooks/use-maintenance";
import { useTotpStatus } from "@/lib/hooks/use-totp";
import { useVersion } from "@/lib/hooks/use-version";
import { formatRelativeTime } from "@/lib/utils/format";

/** "v0.1.7" / "0.1.7" both compare the same — the manifest omits the prefix,
 * the binary's own version (ldflags-stamped) carries it. */
function stripV(v: string) {
  return v.startsWith("v") ? v.slice(1) : v;
}

/** Numeric MAJOR.MINOR.PATCH compare — every published version is this shape,
 * so nothing fancier (pre-release ordering, build metadata) is needed here. */
function compareVersions(a: string, b: string): number {
  const pa = stripV(a).split(".").map((n) => parseInt(n, 10) || 0);
  const pb = stripV(b).split(".").map((n) => parseInt(n, 10) || 0);
  for (let i = 0; i < Math.max(pa.length, pb.length); i++) {
    const diff = (pa[i] ?? 0) - (pb[i] ?? 0);
    if (diff !== 0) return diff;
  }
  return 0;
}

/**
 * Current vs latest published version, sourced from the daily update-check
 * worker's cache (worker/update_check_task.go) rather than a live fetch here —
 * belune.dev is reached once a day, server-side, never from the browser.
 *
 * The "update available" decision is re-derived here from the current
 * (useVersion, live) and cached-latest values rather than trusting a
 * precomputed flag, so a manual `update.sh` run doesn't leave a stale banner
 * showing until the next daily tick catches up.
 */
export function UpdateSection() {
  const currentVersion = useVersion();
  const { data: settings } = useSettings();
  const updateSettings = useUpdateSettings();
  const { data: totpStatus } = useTotpStatus();
  const totpEnabled = totpStatus?.enabled ?? false;
  const triggerUpdate = useTriggerSelfUpdate();

  const [confirmOpen, setConfirmOpen] = useState(false);
  const [password, setPassword] = useState("");
  const [code, setCode] = useState("");

  const setting = (key: string) => settings?.find((s) => s.key === key)?.value ?? "";

  const latestVersion = setting("update_latest_version");
  const breaking = setting("update_latest_breaking") === "true";
  const requiresHostUpdate = setting("update_latest_requires_host_update") === "true";
  const notesUrl = setting("update_latest_notes_url");
  const lastCheckedAt = setting("update_last_checked_at");
  const skipVersion = setting("update_skip_version");
  const checkEnabled = setting("update_check_enabled") !== "false";

  // An unstamped local build ("dev") or a page still loading /api/version has
  // nothing meaningful to compare against.
  const validCurrent = Boolean(currentVersion) && currentVersion !== "dev";
  const updateAvailable =
    validCurrent &&
    Boolean(latestVersion) &&
    compareVersions(latestVersion, currentVersion) > 0 &&
    latestVersion !== skipVersion;

  const toggleCheck = (next: boolean) => {
    toast.promise(
      updateSettings.mutateAsync([
        { key: "update_check_enabled", value: next ? "true" : "false" },
      ]),
      {
        loading: "Saving…",
        success: `Update checks ${next ? "enabled" : "disabled"}`,
        error: (err) => err.message,
      },
    );
  };

  const skipThisVersion = () => {
    toast.promise(
      updateSettings.mutateAsync([
        { key: "update_skip_version", value: latestVersion },
      ]),
      {
        loading: "Saving…",
        success: `v${latestVersion} will not be flagged again`,
        error: (err) => err.message,
      },
    );
  };

  const closeConfirm = () => {
    setConfirmOpen(false);
    setPassword("");
    setCode("");
  };

  const applyUpdate = () => {
    if (!password) return;
    triggerUpdate.mutate(
      { password, code: code || undefined },
      {
        onSuccess: (res) => {
          closeConfirm();
          toast.success(
            `Update to v${res.target} started — the dashboard will disconnect briefly while it restarts.`,
            { duration: 10_000 },
          );
        },
        onError: (err) => {
          toast.error(err instanceof Error ? err.message : "Failed to start the update");
        },
      },
    );
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2 text-sm">
          <span className="text-muted-foreground">Running</span>
          <span className="font-mono font-medium">
            {currentVersion || "…"}
          </span>
          {updateAvailable ? (
            <Badge variant="outline" className="border-status-building-line bg-status-building-soft text-status-building">
              Update available
            </Badge>
          ) : (
            validCurrent && <Badge variant="light">Up to date</Badge>
          )}
        </div>
        <div className="flex items-center gap-2">
          <span className="text-muted-foreground text-xs">
            Check automatically
          </span>
          <Switch
            aria-label="Check for updates automatically"
            checked={checkEnabled}
            disabled={updateSettings.isPending}
            onCheckedChange={toggleCheck}
          />
        </div>
      </div>

      {updateAvailable && (
        <div className="space-y-3 rounded-md border p-3">
          <div className="flex flex-wrap items-center gap-2">
            <p className="text-sm font-medium">
              v{latestVersion} is available
            </p>
            {breaking && <Badge variant="destructive">Breaking changes</Badge>}
            {requiresHostUpdate && (
              <Badge variant="outline">Requires host update</Badge>
            )}
            {notesUrl && (
              <a
                href={notesUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="text-muted-foreground inline-flex items-center gap-1 text-xs hover:underline"
              >
                Release notes
                <ExternalLinkIcon aria-hidden="true" className="h-3 w-3" />
              </a>
            )}
          </div>

          <div className="bg-muted flex items-center gap-2 rounded-md px-3 py-2">
            <code className="min-w-0 flex-1 font-mono text-sm">
              sudo bash scripts/update.sh
            </code>
            <CopyButton value="sudo bash scripts/update.sh" />
          </div>
          <p className="text-muted-foreground text-xs">
            Run from your install directory (e.g. <span className="font-mono">/opt/belune</span>). Takes a
            backup first, then applies the update.
          </p>

          {requiresHostUpdate ? (
            <p className="text-status-building text-xs">
              This release changes host-level configuration — the dashboard
              can't apply it. Use the command above instead.
            </p>
          ) : (
            <div className="flex items-center gap-2">
              <Button size="sm" onClick={() => setConfirmOpen(true)}>
                Update now
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={skipThisVersion}
                disabled={updateSettings.isPending}
              >
                Skip this version
              </Button>
            </div>
          )}
        </div>
      )}

      {lastCheckedAt && (
        <p className="text-muted-foreground text-xs">
          Checked {formatRelativeTime(lastCheckedAt)}
        </p>
      )}

      {/* Step-up password prompt — same shape as the host-shell gate. */}
      <Dialog
        open={confirmOpen}
        onOpenChange={(open) => !open && closeConfirm()}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Update to v{latestVersion}?</DialogTitle>
            <DialogDescription>
              Re-enter your Belune password to apply this update.
              {totpEnabled &&
                " Your authenticator code is required too."} A pre-update
              backup runs first. The dashboard will be briefly unreachable
              while it restarts.
              {breaking &&
                " This release includes breaking changes — read the release notes before continuing."}
            </DialogDescription>
          </DialogHeader>
          <Input
            type="password"
            autoFocus
            value={password}
            placeholder="Password"
            onChange={(e) => setPassword(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") applyUpdate();
            }}
          />
          {totpEnabled && (
            <Input
              inputMode="numeric"
              autoComplete="one-time-code"
              value={code}
              placeholder="Verification code"
              onChange={(e) => setCode(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") applyUpdate();
              }}
            />
          )}
          <DialogFooter>
            <Button variant="outline" onClick={closeConfirm}>
              Cancel
            </Button>
            <Button
              onClick={applyUpdate}
              disabled={
                triggerUpdate.isPending ||
                !password ||
                (totpEnabled && !code)
              }
            >
              {triggerUpdate.isPending ? "Starting…" : "Update now"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

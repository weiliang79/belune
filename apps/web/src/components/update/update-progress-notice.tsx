import { useEffect, useSyncExternalStore } from "react";
import { Loader2Icon } from "lucide-react";
import { toast } from "sonner";
import {
  formatElapsed,
  useUpdateProgress,
  type UpdateProgress,
} from "@/lib/update-progress";

const TOAST_ID = "platform-update";

// A one-second clock as an external store, so the time is refreshed on
// subscribe (an effect that set state to do it would render twice, and a
// `useState` initialiser would hold the time the page MOUNTED, minutes before
// an update starts).
let tickNow = Date.now();
function subscribeTick(onChange: () => void) {
  tickNow = Date.now();
  onChange();
  const t = window.setInterval(() => {
    tickNow = Date.now();
    onChange();
  }, 1_000);
  return () => window.clearInterval(t);
}
const subscribeNever = () => () => {};

/** The current time, re-rendering once a second while `active`. The elapsed
 *  counter and the reload countdown both hang off it. */
function useNow(active: boolean): number {
  return useSyncExternalStore(
    active ? subscribeTick : subscribeNever,
    () => tickNow,
  );
}

/**
 * Everything the user sees of a platform update, in one place.
 *
 *  updating   → a persistent, non-blocking toast with elapsed time. The API is
 *               up for most of an update (the pull and the backup), so nothing
 *               here may get in the way of working.
 *  restarting → a blocking screen: the API is not answering, so the page behind
 *               it can only fail.
 *  reloading  → the same screen with the reload countdown.
 *
 * Elapsed time, never a countdown to completion: nothing can know how long a
 * backup of this database takes, so any "X remaining" would be invented. The
 * one countdown here is the reload delay, which we control.
 *
 * The toast pattern lives here deliberately — it is the first persistent toast
 * in the app, so it is defined once rather than at call sites. The id makes
 * every tick an in-place update, so the 1s re-render cannot stack duplicates,
 * and it is not dismissible because dismissing it would only be undone a second
 * later.
 */
export function UpdateProgressNotice() {
  const progress = useUpdateProgress();
  const now = useNow(progress.stage !== "idle");

  const showToast = progress.stage === "updating";
  const elapsed = progress.since === null ? 0 : now - progress.since;
  const toastTitle = `Belune is updating · ${formatElapsed(elapsed)}`;

  useEffect(() => {
    if (!showToast) return;
    toast.loading(toastTitle, {
      id: TOAST_ID,
      duration: Infinity,
      dismissible: false,
      description:
        "You can keep working. This page reloads itself when it finishes.",
    });
  }, [showToast, toastTitle]);

  // Separate from the effect above so the dismiss runs only on leaving the
  // stage, not on every tick's cleanup (which would flicker the toast).
  useEffect(() => {
    if (!showToast) return;
    return () => {
      toast.dismiss(TOAST_ID);
    };
  }, [showToast]);

  if (progress.stage !== "restarting" && progress.stage !== "reloading") {
    return null;
  }
  return <BlockingScreen progress={progress} now={now} />;
}

function BlockingScreen({
  progress,
  now,
}: {
  progress: UpdateProgress;
  now: number;
}) {
  const reloading = progress.stage === "reloading";
  const secondsLeft = Math.max(
    0,
    Math.ceil(((progress.reloadAt ?? now) - now) / 1_000),
  );

  return (
    <div
      role="alertdialog"
      aria-live="polite"
      aria-label={reloading ? "Belune updated" : "Belune is restarting"}
      className="bg-background/80 fixed inset-0 z-[100] flex items-center justify-center backdrop-blur-sm"
    >
      <div className="flex max-w-sm flex-col items-center gap-3 px-6 text-center">
        <Loader2Icon className="text-brand-text size-6 animate-spin" />
        <p className="text-foreground font-medium">
          {reloading
            ? `Updated to ${progress.version} — reloading in ${secondsLeft}s`
            : "Belune is restarting…"}
        </p>
        {!reloading && (
          <p className="text-muted-foreground text-sm">
            Updating ·{" "}
            {formatElapsed(progress.since === null ? 0 : now - progress.since)}.
            The dashboard will pick up again on its own.
          </p>
        )}
      </div>
    </div>
  );
}

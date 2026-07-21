import { Badge } from "@/components/ui/badge";
import { formatRelativeTime } from "@/lib/utils/format";
import { cn } from "@/lib/utils";
import type { Application } from "@/lib/types";

/**
 * Says when the running container no longer matches the saved configuration,
 * and which button fixes it.
 *
 * This replaces the unconditional "takes effect on the next deploy" notes that
 * used to sit under every config form. Those were shown whether or not anything
 * had actually changed, and stayed put after a deploy applied the change — so
 * they carried no information. The server derives `pending_change` from two
 * markers stamped on save and cleared by the worker, so this only appears when
 * there is genuinely something outstanding.
 *
 * The wording names actions that exist on this page. Never "Rebuild": that
 * button already means something narrower — rebuild the *pinned* commit — and
 * would not pick up a changed branch or image.
 */
export function PendingChangeBadge({
  app,
  className,
  pulse = true,
}: {
  app: Application;
  className?: string;
  /**
   * Whether the dot pulses. Defaults to true, which suits a form section where
   * the badge stands alone. Set false anywhere the badge sits beside a
   * StatusPill: a pulsing amber dot is that component's established signal for a
   * *transient* state (building/deploying), so pulsing here would make a
   * standing "config has drifted" notice read as work in progress.
   */
  pulse?: boolean;
}) {
  if (!app.pending_change) return null;

  const isSource = app.pending_change === "source";
  const changedAt = isSource ? app.source_changed_at : app.config_changed_at;
  const since = changedAt ? formatRelativeTime(changedAt) : null;

  return (
    <Badge
      variant="outline"
      className={cn(
        "border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-400",
        className,
      )}
      title={
        isSource
          ? `The build source changed${since ? ` ${since}` : ""}. A reload will not pick this up — it recreates the container from the image that is already there, so a deploy is needed to build or pull the new one.`
          : `Configuration changed${since ? ` ${since}` : ""}. Reload recreates the container from the current image, which is enough to apply it.`
      }
    >
      {/* Pulsing dot: signals this is a live, outstanding state rather than a
          static label — it keeps drawing the eye until the change is applied. */}
      <span aria-hidden="true" className="relative flex size-1.5">
        {pulse && (
          <span className="absolute inline-flex size-full animate-ping rounded-full bg-amber-500 opacity-75" />
        )}
        <span className="relative inline-flex size-1.5 rounded-full bg-amber-500" />
      </span>
      {isSource ? "Redeploy Needed" : "Reload Needed"}
    </Badge>
  );
}

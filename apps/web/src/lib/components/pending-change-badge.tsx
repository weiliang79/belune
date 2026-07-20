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
}: {
  app: Application;
  className?: string;
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
      {isSource ? "Deploy to apply" : "Reload to apply"}
    </Badge>
  );
}

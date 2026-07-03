import { cn } from "@/lib/utils";

/**
 * A small "Live" status pill: a pulsing dot plus a label. When `active`, the dot
 * bounces (ping) to signal a live/streaming connection; otherwise it renders a
 * static muted dot with the idle label.
 */
export function LiveIndicator({
  active,
  activeLabel = "Live",
  idleLabel = "Disconnected",
  className,
}: {
  active: boolean;
  activeLabel?: string;
  idleLabel?: string;
  className?: string;
}) {
  return (
    <div className={cn("flex items-center gap-2", className)}>
      <span className="relative flex size-2">
        {active && (
          <span className="bg-status-ready absolute inline-flex size-full animate-ping rounded-full opacity-75" />
        )}
        <span
          className={cn(
            "relative inline-flex size-2 rounded-full",
            active ? "bg-status-ready" : "bg-text-faint",
          )}
        />
      </span>
      <span className="text-muted-foreground text-sm">
        {active ? activeLabel : idleLabel}
      </span>
    </div>
  );
}

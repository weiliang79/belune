import { cn } from "@/lib/utils";

export interface StatusSegment {
  label: string;
  count: number;
  /** Tailwind background class for the segment + legend dot (e.g. bg-status-ready). */
  className: string;
}

/**
 * Horizontal stacked distribution bar with a count legend. Segments with a
 * zero count are omitted from the bar but kept in the legend.
 */
export function StatusBar({
  segments,
  className,
}: {
  segments: StatusSegment[];
  className?: string;
}) {
  const total = segments.reduce((sum, s) => sum + s.count, 0);

  return (
    <div className={className}>
      <div className="bg-elev flex h-2 w-full overflow-hidden rounded-full">
        {total > 0 &&
          segments.map((s) =>
            s.count > 0 ? (
              <div
                key={s.label}
                className={s.className}
                style={{ width: `${(s.count / total) * 100}%` }}
              />
            ) : null,
          )}
      </div>
      <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1">
        {segments.map((s) => (
          <div key={s.label} className="flex items-center gap-1.5 text-xs">
            <span className={cn("size-2 rounded-full", s.className)} />
            <span className="font-mono">{s.count}</span>
            <span className="text-text-muted">{s.label}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

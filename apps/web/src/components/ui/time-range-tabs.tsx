import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import type { TimeRange } from "@/lib/utils/time-range";

/** Segmented 24h / 7d / 30d / All-time control. Controlled by `value`. */
export function TimeRangeTabs({
  value,
  onChange,
  className,
}: {
  value: TimeRange;
  onChange: (range: TimeRange) => void;
  className?: string;
}) {
  return (
    <ToggleGroup
      variant="outline"
      size="sm"
      value={[value]}
      onValueChange={(v) => v.length > 0 && onChange(v[0] as TimeRange)}
      className={className}
      aria-label="Time range"
    >
      <ToggleGroupItem value="24h">24h</ToggleGroupItem>
      <ToggleGroupItem value="7d">7d</ToggleGroupItem>
      <ToggleGroupItem value="30d">30d</ToggleGroupItem>
      <ToggleGroupItem value="all">All time</ToggleGroupItem>
    </ToggleGroup>
  );
}

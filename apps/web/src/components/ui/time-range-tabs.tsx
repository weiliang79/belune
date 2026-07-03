import {
  SegmentedControl,
  SegmentedControlItem,
} from "@/components/ui/segmented-control";
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
    <SegmentedControl
      size="sm"
      value={value}
      onValueChange={(v) => onChange(v as TimeRange)}
      className={className}
      aria-label="Time range"
    >
      <SegmentedControlItem value="24h">24h</SegmentedControlItem>
      <SegmentedControlItem value="7d">7d</SegmentedControlItem>
      <SegmentedControlItem value="30d">30d</SegmentedControlItem>
      <SegmentedControlItem value="all">All time</SegmentedControlItem>
    </SegmentedControl>
  );
}

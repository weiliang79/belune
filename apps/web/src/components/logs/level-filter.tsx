import {
  SegmentedControl,
  SegmentedControlItem,
} from "@/components/ui/segmented-control";
import { LEVELS, LEVEL_LABELS, type LogLevel } from "@/lib/logs/level";

// "" = All (no level filter). The segmented control needs a non-empty token per
// segment, so "All" uses the "all" token mapped to "" at the boundary.
export type LevelFilterValue = "" | LogLevel;

export function LevelFilter({
  value,
  onChange,
}: {
  value: LevelFilterValue;
  onChange: (value: LevelFilterValue) => void;
}) {
  return (
    <SegmentedControl
      size="sm"
      value={value || "all"}
      onValueChange={(v) => onChange(v === "all" ? "" : (v as LogLevel))}
      aria-label="Log level"
    >
      <SegmentedControlItem value="all">All</SegmentedControlItem>
      {LEVELS.map((lvl) => (
        <SegmentedControlItem key={lvl} value={lvl}>
          {LEVEL_LABELS[lvl]}
        </SegmentedControlItem>
      ))}
    </SegmentedControl>
  );
}

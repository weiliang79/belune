import type { ReactNode } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";

type Tone = "default" | "ready" | "attention" | "error";

const VALUE_TONE: Record<Tone, string> = {
  default: "text-foreground",
  ready: "text-status-ready",
  attention: "text-status-building",
  error: "text-status-error",
};

/** One at-a-glance metric: label, large value, optional hint and accent tone. */
export function StatCard({
  label,
  value,
  hint,
  tone = "default",
  icon,
}: {
  label: string;
  value: ReactNode;
  hint?: ReactNode;
  tone?: Tone;
  icon?: ReactNode;
}) {
  return (
    <Card>
      <CardContent className="p-4">
        <div className="flex items-center justify-between gap-2">
          <p className="text-text-faint text-xs font-medium">{label}</p>
          {icon}
        </div>
        <p
          className={cn(
            "mt-1.5 text-2xl font-semibold tracking-tight",
            VALUE_TONE[tone],
          )}
        >
          {value}
        </p>
        {hint != null && (
          <p className="text-muted-foreground mt-0.5 truncate text-xs">
            {hint}
          </p>
        )}
      </CardContent>
    </Card>
  );
}

/** A labelled usage meter (used / total) for the host-resources card. */
export function MeterRow({
  label,
  percent,
  detail,
}: {
  label: string;
  percent: number;
  detail?: string;
}) {
  const clamped = Math.max(0, Math.min(100, percent));
  const tone =
    clamped >= 90
      ? "bg-status-error"
      : clamped >= 75
        ? "bg-status-building"
        : "bg-status-ready";
  return (
    <div>
      <div className="mb-1 flex items-baseline justify-between text-xs">
        <span className="text-text-faint">{label}</span>
        <span className="font-mono">
          {detail ? `${detail} · ` : ""}
          {clamped.toFixed(0)}%
        </span>
      </div>
      <div
        className="bg-elev h-1.5 overflow-hidden rounded-full"
        role="progressbar"
        aria-label={label}
        aria-valuenow={Math.round(clamped)}
        aria-valuemin={0}
        aria-valuemax={100}
      >
        <div
          className={cn("h-full rounded-full transition-all", tone)}
          style={{ width: `${clamped}%` }}
        />
      </div>
    </div>
  );
}

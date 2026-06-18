import { cn } from "@/lib/utils";

/**
 * Status categories mapped onto the design-system status tokens.
 * - ready    → brand accent (running / healthy / succeeded)
 * - building → amber, with a pulsing dot (transient, in-progress states)
 * - error    → red (failed / crashed / unhealthy)
 * - neutral  → muted (stopped / inactive / unknown)
 */
type StatusTone = "ready" | "building" | "error" | "neutral";

const TONE_BY_STATUS: Record<string, StatusTone> = {
  // ready / positive
  running: "ready",
  active: "ready",
  ready: "ready",
  healthy: "ready",
  succeeded: "ready",
  success: "ready",
  completed: "ready",
  // building / transient
  building: "building",
  creating: "building",
  deploying: "building",
  provisioning: "building",
  pending: "building",
  queued: "building",
  restarting: "building",
  // error
  error: "error",
  failed: "error",
  crashed: "error",
  unhealthy: "error",
  exited: "error",
  canceled: "error",
  cancelled: "error",
  // neutral
  stopped: "neutral",
  inactive: "neutral",
  paused: "neutral",
  unknown: "neutral",
};

const TONE_CLASSES: Record<StatusTone, string> = {
  ready: "bg-status-ready-soft text-status-ready ring-status-ready-line",
  building:
    "bg-status-building-soft text-status-building ring-status-building-line",
  error: "bg-status-error-soft text-status-error ring-status-error-line",
  neutral: "bg-elev text-text-muted ring-border-strong",
};

const DOT_CLASSES: Record<StatusTone, string> = {
  ready: "bg-status-ready",
  building: "bg-status-building",
  error: "bg-status-error",
  neutral: "bg-text-faint",
};

function toneOf(status: string): StatusTone {
  return TONE_BY_STATUS[status.toLowerCase()] ?? "neutral";
}

export interface StatusPillProps {
  status: string;
  /** Override the displayed label (defaults to the status, title-cased). */
  label?: string;
  className?: string;
}

export function StatusPill({ status, label, className }: StatusPillProps) {
  const tone = toneOf(status);
  const text = label ?? status.charAt(0).toUpperCase() + status.slice(1);
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset",
        TONE_CLASSES[tone],
        className,
      )}
    >
      <span className="relative flex size-1.5">
        {tone === "building" && (
          <span
            aria-hidden="true"
            className={cn(
              "absolute inline-flex size-full animate-ping rounded-full opacity-75",
              DOT_CLASSES[tone],
            )}
          />
        )}
        <span
          aria-hidden="true"
          className={cn(
            "relative inline-flex size-1.5 rounded-full",
            DOT_CLASSES[tone],
          )}
        />
      </span>
      {text}
    </span>
  );
}

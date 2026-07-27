import { cn } from "@/lib/utils";

/**
 * Status categories mapped onto the design-system status tokens.
 * - ready    → brand accent (running / healthy / succeeded)
 * - building → amber (transient, in-progress states — and any partial/degraded
 *              count that reuses the same amber via a `tone` override)
 * - error    → red (failed / crashed / unhealthy)
 * - neutral  → muted (stopped / inactive / unknown)
 */
export type StatusTone = "ready" | "building" | "error" | "neutral";

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

// Mirror the Badge variant styles (border + color/10 fill + solid text): the
// positive/ready tone reuses `primary` and `error` reuses `destructive` so
// pills and badges read as one system. `building` and `neutral` keep their own
// hues but follow the same border+fill structure.
const TONE_CLASSES: Record<StatusTone, string> = {
  ready: "border-primary/20 bg-primary/10 text-primary",
  building:
    "border-status-building/20 bg-status-building/10 text-status-building",
  error: "border-destructive/20 bg-destructive/10 text-destructive",
  neutral: "border-border-strong bg-elev text-text-muted",
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
  /**
   * Override the tone derived from `status`. Use when the colour reflects a
   * computed condition (e.g. a running/total ratio) rather than the status word.
   */
  tone?: StatusTone;
  /**
   * Whether the dot pulses. Defaults to on for the `building` tone (a transient,
   * in-progress state) and off for all others. Set explicitly to force a steady
   * dot on an amber pill that represents a settled state, not progress.
   */
  pulse?: boolean;
  className?: string;
}

export function StatusPill({
  status,
  label,
  tone: toneOverride,
  pulse,
  className,
}: StatusPillProps) {
  const tone = toneOverride ?? toneOf(status);
  const showPulse = pulse ?? tone === "building";
  const text = label ?? status.charAt(0).toUpperCase() + status.slice(1);
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-xs font-medium",
        TONE_CLASSES[tone],
        className,
      )}
    >
      <span className="relative flex size-1.5">
        {showPulse && (
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

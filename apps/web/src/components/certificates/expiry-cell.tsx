import { EXPIRY_WARNING_DAYS, daysUntil } from "@/lib/expiry";

/**
 * A certificate's expiry date, escalating to a warning as it nears.
 *
 * Shared by the Certificates page and an application's domains table: the same
 * date means the same thing in both places, and an operator who learns the
 * amber "14d left" in one should not meet a different dialect in the other.
 *
 * `quiet` suppresses the warning. Caddy's internal certificates last 12 hours
 * and are renewed automatically, so a local domain would sit permanently on a
 * red "0d left" — an alarm that means nothing and trains the operator to ignore
 * the one that does.
 */
export function ExpiryCell({
  notAfter,
  quiet,
}: {
  notAfter: string | null | undefined;
  quiet?: boolean;
}) {
  const days = daysUntil(notAfter);
  if (days === null) {
    return <span className="text-muted-foreground">—</span>;
  }

  const formatted = new Date(notAfter!).toLocaleDateString();
  if (quiet) {
    return <span className="text-muted-foreground">{formatted}</span>;
  }
  if (days < 0) {
    return (
      <span className="text-status-error font-semibold">
        Expired {formatted}
      </span>
    );
  }
  if (days <= EXPIRY_WARNING_DAYS) {
    return (
      <span className="text-status-building">
        {formatted} · {days}d left
      </span>
    );
  }
  return <span>{formatted}</span>;
}

import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import type { Database } from "@/lib/types";

/**
 * Says a database's container has been removed from the host while the record
 * and data volume are intact — the state a Reload recreates. Sits beside the
 * status badge, mirroring the application PendingChangeBadge so "Reload Needed"
 * reads the same across resource types.
 *
 * Databases have no build source or saved-config drift of their own, so the only
 * outstanding state is the missing container (server-derived `container_missing`,
 * set only in the steady stopped/failed states). Kept separate from
 * PendingChangeBadge, which is typed to Application and carries that page's
 * config-vs-source wording.
 */
export function DatabaseReloadBadge({
  db,
  className,
  pulse = true,
}: {
  db: Database;
  className?: string;
  /**
   * Whether the dot pulses. Defaults to true (suits the standalone detail
   * header). Set false where it sits in a dense row beside a StatusPill.
   */
  pulse?: boolean;
}) {
  if (!db.container_missing) return null;

  return (
    <Badge
      variant="outline"
      className={cn(
        "border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-400",
        className,
      )}
      title="The database container was removed from the host. Reload recreates it from the stored configuration — the data volume is preserved."
    >
      <span aria-hidden="true" className="relative flex size-1.5">
        {pulse && (
          <span className="absolute inline-flex size-full animate-ping rounded-full bg-amber-500 opacity-75" />
        )}
        <span className="relative inline-flex size-1.5 rounded-full bg-amber-500" />
      </span>
      Reload Needed
    </Badge>
  );
}

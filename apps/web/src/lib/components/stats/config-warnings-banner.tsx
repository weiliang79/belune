import { ShieldAlertIcon } from "lucide-react";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { useStats } from "@/lib/hooks/use-stats";
import { cn } from "@/lib/utils";

/**
 * Configuration findings — settings the server accepted but that weaken the
 * install. Rendered as a banner rather than folded into the "Needs attention"
 * count: that number is how many workloads are broken right now, and a config
 * finding is a different kind of thing that would corrupt it.
 *
 * The API only returns these to admins, so this renders nothing for members
 * without needing a role check of its own. Warning tone, not destructive —
 * nothing is currently failing, and overstating it trains operators to dismiss
 * the banner.
 */
export function ConfigWarningsBanner({ className }: { className?: string }) {
  const { data: stats } = useStats();
  const warnings = stats?.config_warnings ?? [];

  if (warnings.length === 0) return null;

  return (
    <div className={cn("space-y-2", className)}>
      {warnings.map((w) => (
        <Alert key={w.code} variant="warning">
          <ShieldAlertIcon aria-hidden="true" />
          <AlertTitle>{w.message}</AlertTitle>
          <AlertDescription>{w.remedy}</AlertDescription>
        </Alert>
      ))}
    </div>
  );
}

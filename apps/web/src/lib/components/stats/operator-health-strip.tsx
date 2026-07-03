import {
  ActivityIcon,
  RocketIcon,
  TriangleAlertIcon,
  ServerIcon,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { useStats } from "@/lib/hooks/use-stats";
import { formatBytes } from "@/lib/utils/format";
import { cn } from "@/lib/utils";
import { StatCard, MetricCard, MeterRow } from "./stat-card";

function pct(part: number, total: number) {
  return total > 0 ? Math.round((part / total) * 100) : 0;
}

/**
 * Operator-health stat strip. Members see app health, deploy success, and
 * attention items scoped to their projects; admins additionally see host
 * resources. No trend arrows — every number is computed, not invented.
 */
export function OperatorHealthStrip({ className }: { className?: string }) {
  const { data: stats, isLoading } = useStats();

  if (isLoading || !stats) {
    return (
      <div
        className={cn(
          "grid items-start gap-4 sm:grid-cols-2 lg:grid-cols-4",
          className,
        )}
      >
        {[1, 2, 3, 4].map((i) => (
          <Card key={i}>
            <CardContent className="space-y-2 p-4">
              <Skeleton className="h-3 w-20" />
              <Skeleton className="h-7 w-16" />
              <Skeleton className="h-3 w-24" />
            </CardContent>
          </Card>
        ))}
      </div>
    );
  }

  const { app_health, deploy_7d, needs_attention, host } = stats;

  const healthTone =
    app_health.total === 0
      ? "default"
      : app_health.running === app_health.total
        ? "ready"
        : "attention";

  const deployRate = pct(deploy_7d.succeeded, deploy_7d.total);
  const attention = needs_attention.total;
  const notRunning = Math.max(0, app_health.total - app_health.running);

  const cols = host ? "lg:grid-cols-4" : "lg:grid-cols-3";

  return (
    <div
      className={cn("grid items-stretch gap-4 sm:grid-cols-2", cols, className)}
    >
      <StatCard
        label="App health"
        icon={<ActivityIcon className="size-3.5" />}
        tone={healthTone}
        value={
          <span className="font-mono">
            {app_health.running}
            <span className="text-text-faint">/{app_health.total}</span>
          </span>
        }
        hint={app_health.total === 0 ? "No services yet" : "services running"}
        footer={
          app_health.total === 0 ? undefined : (
            <>
              <Badge variant="light">{app_health.running} running</Badge>
              {notRunning > 0 && (
                <Badge variant="destructive">{notRunning} down</Badge>
              )}
            </>
          )
        }
      />

      {host && (
        <MetricCard
          label="Host resources"
          icon={<ServerIcon className="size-3.5" />}
        >
          <div className="mt-2.5 space-y-2.5">
            <MeterRow label="CPU" percent={host.cpu_percent} />
            <MeterRow
              label="Memory"
              percent={pct(host.memory_used, host.memory_total)}
              detail={`${formatBytes(host.memory_used)}/${formatBytes(host.memory_total)}`}
            />
            <MeterRow
              label="Disk"
              percent={pct(host.disk_used, host.disk_total)}
              detail={`${formatBytes(host.disk_used)}/${formatBytes(host.disk_total)}`}
            />
          </div>
        </MetricCard>
      )}

      <StatCard
        label="Needs attention"
        icon={<TriangleAlertIcon className="size-3.5" />}
        tone={attention === 0 ? "ready" : "error"}
        value={attention}
        hint={attention === 0 ? "All clear" : attention === 1 ? "issue" : "issues"}
        footer={
          attention === 0 ? undefined : (
            <>
              {needs_attention.error_services > 0 && (
                <Badge variant="destructive">
                  {needs_attention.error_services} errored
                </Badge>
              )}
              {needs_attention.failed_deploys > 0 && (
                <Badge variant="destructive">
                  {needs_attention.failed_deploys} failed deploys
                </Badge>
              )}
              {needs_attention.failed_backups > 0 && (
                <Badge variant="destructive">
                  {needs_attention.failed_backups} failed backups
                </Badge>
              )}
            </>
          )
        }
      />

      <StatCard
        label="Deploy success · 7d"
        icon={<RocketIcon className="size-3.5" />}
        tone={
          deploy_7d.total === 0
            ? "default"
            : deployRate === 100
              ? "ready"
              : "attention"
        }
        value={deploy_7d.total === 0 ? "—" : `${deployRate}%`}
        hint={
          deploy_7d.total === 0
            ? "No deploys in 7 days"
            : `${deploy_7d.succeeded} of ${deploy_7d.total} deploys`
        }
        footer={
          deploy_7d.total === 0 ? undefined : (
            <>
              <Badge variant="light">{deploy_7d.succeeded} succeeded</Badge>
              {deploy_7d.failed > 0 && (
                <Badge variant="destructive">{deploy_7d.failed} failed</Badge>
              )}
            </>
          )
        }
      />
    </div>
  );
}

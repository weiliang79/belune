import type { ReactNode } from "react";
import { HardDriveIcon, ServerIcon } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { useDockerOverview } from "@/lib/hooks/use-docker";
import { formatBytes } from "@/lib/utils/format";
import type { DockerDiskUsageEntry } from "@/lib/types";
import { sizeLabel } from "./utils";

export function DockerOverviewTab({ enabled }: { enabled: boolean }) {
  const { data, isLoading } = useDockerOverview(enabled);

  if (isLoading || !data) {
    return (
      <div className="space-y-6">
        <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-20 rounded-xl" />
          ))}
        </div>
        <Skeleton className="h-48 rounded-xl" />
        <Skeleton className="h-56 rounded-xl" />
      </div>
    );
  }

  const { info, disk_usage: du, counts } = data;

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
        <CountCard
          title="Containers"
          value={counts.containers_total}
          subtitle={`${counts.containers_running} running`}
        />
        <CountCard title="Images" value={counts.images} />
        <CountCard title="Volumes" value={du ? counts.volumes : null} />
        <CountCard
          title="CPUs"
          value={info.ncpu}
          subtitle={formatBytes(info.mem_total) + " RAM"}
        />
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <ServerIcon aria-hidden="true" className="size-4" />
            Daemon
          </CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-x-8 gap-y-3 sm:grid-cols-2">
          <InfoRow label="Docker version" value={info.server_version || "—"} />
          <InfoRow label="Host" value={info.name || "—"} />
          <InfoRow
            label="OS"
            value={`${info.operating_system || info.os_type} (${info.architecture})`}
          />
          <InfoRow label="Kernel" value={info.kernel_version || "—"} />
          <InfoRow label="Storage driver" value={info.storage_driver || "—"} />
          <InfoRow label="Logging driver" value={info.logging_driver || "—"} />
          <InfoRow label="Cgroup driver" value={info.cgroup_driver || "—"} />
          <InfoRow label="Root dir" value={info.docker_root_dir || "—"} mono />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <HardDriveIcon aria-hidden="true" className="size-4" />
            Disk Usage
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-text-faint text-left">
                  <th className="pb-2 font-medium">Type</th>
                  <th className="pb-2 text-right font-medium">Items</th>
                  <th className="pb-2 text-right font-medium">Size</th>
                  <th className="pb-2 text-right font-medium">Reclaimable</th>
                </tr>
              </thead>
              <tbody className="divide-border divide-y">
                {du ? (
                  <>
                    <UsageRow label="Images" entry={du.images} />
                    <UsageRow label="Containers" entry={du.containers} />
                    <UsageRow label="Local volumes" entry={du.volumes} />
                    <UsageRow label="Build cache" entry={du.build_cache} />
                  </>
                ) : (
                  <tr>
                    <td
                      colSpan={4}
                      className="text-muted-foreground py-4 text-center text-xs"
                    >
                      Still measuring — `docker system df` walks every image,
                      container and volume, which can take a while on a small
                      host. It will appear here shortly.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
          <p className="text-muted-foreground mt-3 text-xs">
            Reclaimable space can be freed with the platform cleanup on the
            Server page (dangling images and unreferenced non-data volumes
            only).
          </p>
        </CardContent>
      </Card>
    </div>
  );
}

function CountCard({
  title,
  value,
  subtitle,
}: {
  title: string;
  // null while the figure is still being computed in the background.
  value: number | null;
  subtitle?: string;
}) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <p className="text-muted-foreground text-sm font-medium">{title}</p>
      </CardHeader>
      <CardContent>
        <p className="text-3xl font-bold">{value ?? "—"}</p>
        {subtitle && <p className="text-text-faint mt-1 text-xs">{subtitle}</p>}
      </CardContent>
    </Card>
  );
}

function InfoRow({
  label,
  value,
  mono,
}: {
  label: string;
  value: ReactNode;
  mono?: boolean;
}) {
  return (
    <div className="flex items-center justify-between gap-4 border-b py-1 last:border-b-0 sm:border-b-0">
      <span className="text-muted-foreground text-sm">{label}</span>
      <span
        className={mono ? "truncate font-mono text-xs" : "truncate text-sm"}
      >
        {value}
      </span>
    </div>
  );
}

function UsageRow({
  label,
  entry,
}: {
  label: string;
  entry: DockerDiskUsageEntry;
}) {
  return (
    <tr>
      <td className="py-2">{label}</td>
      <td className="py-2 text-right tabular-nums">{entry.count}</td>
      <td className="py-2 text-right tabular-nums">{sizeLabel(entry.size)}</td>
      <td className="text-text-muted py-2 text-right tabular-nums">
        {sizeLabel(entry.reclaimable)}
      </td>
    </tr>
  );
}

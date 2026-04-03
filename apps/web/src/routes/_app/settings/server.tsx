import { createFileRoute } from "@tanstack/react-router";
import { RouteError } from "@/lib/components/route-error";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  useMetrics,
  useTriggerCleanup,
  useHostHistoricalMetrics,
  useHostMetricsStream,
} from "@/lib/hooks/use-metrics";
import { useSettings, useUpdateSettings } from "@/lib/hooks/use-settings";
import { useFeatures } from "@/lib/hooks/use-features";
import { toast } from "sonner";
import { useState, useMemo } from "react";
import type { HostMetricPoint } from "@/lib/types";
import { UPlotAreaChart } from "@/components/ui/uplot-area-chart";

export const Route = createFileRoute("/_app/settings/server")({
  component: ServerSettingsPage,
  errorComponent: RouteError,
});

function formatTime(iso: string, range: string) {
  const d = new Date(iso);
  if (range === "1h" || range === "3h" || range === "24h")
    return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  return d.toLocaleDateString([], {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function formatBytes(bytes: number | null) {
  if (bytes == null) return "N/A";
  const gb = bytes / (1024 * 1024 * 1024);
  if (gb >= 1) return `${gb.toFixed(1)} GB`;
  const mb = bytes / (1024 * 1024);
  return `${mb.toFixed(0)} MB`;
}

function ServerSettingsPage() {
  const { data: metrics, isLoading } = useMetrics();
  const { data: features } = useFeatures();
  const cleanup = useTriggerCleanup();
  const { data: historicalData } = useHostHistoricalMetrics("1h");
  const { data: streamData, connected: streamConnected } =
    useHostMetricsStream(true);
  const hostMetrics = useMemo(() => {
    const ONE_SECOND = 1_000;
    const THIRTY_MIN = 30 * 60 * 1_000;
    // eslint-disable-next-line react-hooks/purity
    const now = Date.now();
    const maxStart = Math.floor((now - THIRTY_MIN) / ONE_SECOND) * ONE_SECOND;

    // Both sources are 1s granularity — merge into a single 1s lookup
    const dataMap = new Map<number, HostMetricPoint>();
    for (const point of [...(historicalData ?? []), ...(streamData ?? [])]) {
      const t =
        Math.floor(new Date(point.recorded_at).getTime() / ONE_SECOND) *
        ONE_SECOND;
      dataMap.set(t, point);
    }

    // Start from the oldest real data point so there's no empty void on the
    // left, capped at 30 minutes back
    const oldestDataTs =
      dataMap.size > 0 ? Math.min(...dataMap.keys()) : maxStart;
    const start = Math.max(oldestDataTs, maxStart);

    // Generate grid at 1s resolution, using null for missing slots so
    // uPlot spans gaps instead of dropping to zero between real data points
    const grid: HostMetricPoint[] = [];
    for (let t = start; t <= now; t += ONE_SECOND) {
      grid.push(
        dataMap.get(t) ?? {
          recorded_at: new Date(t).toISOString(),
          cpu_percent: null,
          memory_used: null,
          memory_total: null,
          disk_used: null,
          disk_total: null,
        },
      );
    }
    return grid;
  }, [historicalData, streamData]);
  const { data: settings } = useSettings();
  const updateSettings = useUpdateSettings();
  const [retentionDays, setRetentionDays] = useState<string>("");

  const currentRetention =
    settings?.find((s) => s.key === "metrics_retention_days")?.value ?? "30";

  const handleCleanup = () => {
    toast.promise(cleanup.mutateAsync(undefined), {
      loading: "Running cleanup...",
      success: "Cleanup task queued",
      error: "Failed to trigger cleanup",
    });
  };

  const handleSaveRetention = () => {
    const days = retentionDays || currentRetention;
    toast.promise(
      updateSettings
        .mutateAsync([{ key: "metrics_retention_days", value: days }])
        .then(() => {
          setRetentionDays("");
        }),
      {
        loading: "Saving...",
        success: "Retention setting saved",
        error: "Failed to save retention setting",
      },
    );
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Settings</h1>
        <p className="text-muted-foreground">
          Manage your account and platform settings.
        </p>
      </div>

      {isLoading ? (
        <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Card key={i}>
              <CardHeader className="pb-2">
                <div className="bg-muted h-4 w-20 animate-pulse rounded" />
              </CardHeader>
              <CardContent>
                <div className="bg-muted h-8 w-12 animate-pulse rounded" />
              </CardContent>
            </Card>
          ))}
        </div>
      ) : metrics ? (
        <>
          <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
            <StatCard title="Projects" value={metrics.projects} />
            <StatCard title="Applications" value={metrics.applications} />
            <StatCard title="Databases" value={metrics.databases} />
            <StatCard title="Deployments" value={metrics.deployments} />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <Card>
              <CardHeader>
                <CardTitle>Containers</CardTitle>
              </CardHeader>
              <CardContent className="flex h-full flex-col justify-center">
                <div className="flex items-center gap-4">
                  <div className="flex items-center gap-2">
                    <Badge variant="default">
                      {metrics.containers.running}
                    </Badge>
                    <span className="text-muted-foreground text-sm">
                      Running
                    </span>
                  </div>
                  <div className="flex items-center gap-2">
                    <Badge variant="secondary">
                      {metrics.containers.stopped}
                    </Badge>
                    <span className="text-muted-foreground text-sm">
                      Stopped
                    </span>
                  </div>
                  <div className="flex items-center gap-2">
                    <Badge variant="outline">{metrics.containers.total}</Badge>
                    <span className="text-muted-foreground text-sm">Total</span>
                  </div>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Services</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-sm font-medium">BuildKit</p>
                    <p className="text-muted-foreground text-sm">
                      Required for Railpack builds.
                    </p>
                  </div>
                  <Badge
                    variant={
                      features?.buildkit_available ? "default" : "destructive"
                    }
                  >
                    {features?.buildkit_available ? "Connected" : "Unreachable"}
                  </Badge>
                </div>
              </CardContent>
            </Card>
          </div>

          <Card>
            <CardHeader>
              <div className="flex items-center gap-2">
                <CardTitle>Host Metrics</CardTitle>
                <Badge variant={streamConnected ? "default" : "secondary"}>
                  {streamConnected ? "LIVE" : "Connecting..."}
                </Badge>
              </div>
            </CardHeader>
            <CardContent>
              {hostMetrics && hostMetrics.length > 0 ? (
                <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
                  <HostChart
                    title="CPU Usage (%)"
                    data={hostMetrics}
                    dataKey="cpu_percent"
                    range="1h"
                    color="hsl(221, 83%, 53%)"
                    formatter={(v: number) => `${v.toFixed(1)}%`}
                    domain={[0, 100]}
                  />
                  <HostChart
                    title="Memory Usage"
                    data={hostMetrics}
                    dataKey="memory_used"
                    range="1h"
                    color="hsl(262, 83%, 58%)"
                    formatter={(v: number) => formatBytes(v)}
                    domain={[
                      0,
                      Math.max(...hostMetrics.map((m) => m.memory_total ?? 0)),
                    ]}
                  />
                  <HostChart
                    title="Disk Usage"
                    data={hostMetrics}
                    dataKey="disk_used"
                    range="1h"
                    color="hsl(142, 71%, 45%)"
                    formatter={(v: number) => formatBytes(v)}
                    domain={[
                      0,
                      Math.max(...hostMetrics.map((m) => m.disk_total ?? 0)),
                    ]}
                  />
                </div>
              ) : (
                <p className="text-muted-foreground py-8 text-center text-sm">
                  No metrics data available yet. Data is collected every second.
                </p>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Metrics Retention</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="flex items-center gap-4">
                <div className="flex items-center gap-2">
                  <Input
                    type="number"
                    min={1}
                    max={365}
                    placeholder={currentRetention}
                    value={retentionDays}
                    onChange={(e) => setRetentionDays(e.target.value)}
                    className="w-24"
                  />
                  <span className="text-muted-foreground text-sm">days</span>
                </div>
                <Button
                  size="sm"
                  onClick={handleSaveRetention}
                  disabled={updateSettings.isPending}
                >
                  {updateSettings.isPending ? "Saving..." : "Save"}
                </Button>
                <span className="text-muted-foreground text-xs">
                  Current: {currentRetention} days
                </span>
              </div>
              <p className="text-muted-foreground mt-2 text-xs">
                1-second data is kept for 1h, rolled up to 1-minute for 24h,
                5-minute for 7 days, then hourly until the retention limit.
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Maintenance</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm font-medium">Cleanup old deployments</p>
                  <p className="text-muted-foreground text-sm">
                    Remove old deployment records, images, and dangling volumes.
                    Keeps the 3 most recent deployments per service.
                  </p>
                </div>
                <Button
                  variant="outline"
                  onClick={handleCleanup}
                  disabled={cleanup.isPending}
                >
                  {cleanup.isPending ? "Running..." : "Run Cleanup"}
                </Button>
              </div>
            </CardContent>
          </Card>
        </>
      ) : null}
    </div>
  );
}

function StatCard({ title, value }: { title: string; value: number }) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <p className="text-muted-foreground text-sm font-medium">{title}</p>
      </CardHeader>
      <CardContent>
        <p className="text-3xl font-bold">{value}</p>
      </CardContent>
    </Card>
  );
}

function HostChart({
  title,
  data,
  dataKey,
  range,
  color,
  formatter,
  domain,
}: {
  title: string;
  data: HostMetricPoint[];
  dataKey: string;
  range: string;
  color: string;
  formatter: (value: number) => string;
  domain?: [number, number];
}) {
  return (
    <div>
      <p className="mb-2 text-sm font-medium">{title}</p>
      <UPlotAreaChart
        timestamps={data.map((p) => new Date(p.recorded_at).getTime())}
        values={data.map(
          (p) => p[dataKey as keyof HostMetricPoint] as number | null,
        )}
        color={color}
        yFormatter={formatter}
        xFormatter={(ts) => formatTime(new Date(ts).toISOString(), range)}
        yDomain={domain}
        height={200}
      />
    </div>
  );
}

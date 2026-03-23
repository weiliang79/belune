import { createFileRoute } from "@tanstack/react-router";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { SettingsNav } from "@/lib/components/settings-nav";
import {
  useMetrics,
  useTriggerCleanup,
  useHostHistoricalMetrics,
} from "@/lib/hooks/use-metrics";
import { useSettings, useUpdateSettings } from "@/lib/hooks/use-settings";
import { useFeatures } from "@/lib/hooks/use-features";
import { toast } from "sonner";
import { useState } from "react";
import {
  ResponsiveContainer,
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
} from "recharts";
import type { HostMetricPoint } from "@/lib/types";

export const Route = createFileRoute("/_app/settings/server")({
  component: ServerSettingsPage,
});

const RANGE_OPTIONS = ["1h", "24h", "7d", "30d"] as const;

function formatTime(iso: string, range: string) {
  const d = new Date(iso);
  if (range === "1h" || range === "24h") return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  return d.toLocaleDateString([], { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" });
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
  const [hostRange, setHostRange] = useState<string>("1h");
  const { data: hostMetrics } = useHostHistoricalMetrics(hostRange);
  const { data: settings } = useSettings();
  const updateSettings = useUpdateSettings();
  const [retentionDays, setRetentionDays] = useState<string>("");

  const currentRetention = settings?.find((s) => s.key === "metrics_retention_days")?.value ?? "30";

  const handleCleanup = () => {
    cleanup.mutate(undefined, {
      onSuccess: () => toast.success("Cleanup task queued"),
      onError: () => toast.error("Failed to trigger cleanup"),
    });
  };

  const handleSaveRetention = () => {
    const days = retentionDays || currentRetention;
    updateSettings.mutate(
      [{ key: "metrics_retention_days", value: days }],
      {
        onSuccess: () => {
          toast.success("Retention setting saved");
          setRetentionDays("");
        },
        onError: () => toast.error("Failed to save retention setting"),
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

      <SettingsNav />

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
              <CardContent className="h-full flex flex-col justify-center">
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
              <div className="flex items-center justify-between">
                <CardTitle>Host Metrics</CardTitle>
                <div className="flex gap-1">
                  {RANGE_OPTIONS.map((r) => (
                    <Button
                      key={r}
                      size="sm"
                      variant={hostRange === r ? "default" : "outline"}
                      onClick={() => setHostRange(r)}
                    >
                      {r}
                    </Button>
                  ))}
                </div>
              </div>
            </CardHeader>
            <CardContent>
              {hostMetrics && hostMetrics.length > 0 ? (
                <div className="space-y-6">
                  <HostChart
                    title="CPU Usage (%)"
                    data={hostMetrics}
                    dataKey="cpu_percent"
                    range={hostRange}
                    formatter={(v: number) => `${v.toFixed(1)}%`}
                    domain={[0, 100]}
                  />
                  <HostChart
                    title="Memory Usage"
                    data={hostMetrics}
                    dataKey="memory_used"
                    range={hostRange}
                    formatter={(v: number) => formatBytes(v)}
                  />
                  <HostChart
                    title="Disk Usage"
                    data={hostMetrics}
                    dataKey="disk_used"
                    range={hostRange}
                    formatter={(v: number) => formatBytes(v)}
                  />
                </div>
              ) : (
                <p className="text-muted-foreground py-8 text-center text-sm">
                  No metrics data available yet. Data is collected every minute.
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
                1-minute data is kept for 24h, downsampled to 5-minute for 7
                days, then hourly until the retention limit.
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
  formatter,
  domain,
}: {
  title: string;
  data: HostMetricPoint[];
  dataKey: string;
  range: string;
  formatter: (value: number) => string;
  domain?: [number, number];
}) {
  return (
    <div>
      <p className="text-sm font-medium mb-2">{title}</p>
      <ResponsiveContainer width="100%" height={200}>
        <LineChart data={data}>
          <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
          <XAxis
            dataKey="recorded_at"
            tickFormatter={(v) => formatTime(v, range)}
            className="text-xs"
            tick={{ fontSize: 11 }}
          />
          <YAxis
            tickFormatter={formatter}
            className="text-xs"
            tick={{ fontSize: 11 }}
            domain={domain}
            width={70}
          />
          <Tooltip
            labelFormatter={(v) => new Date(v as string).toLocaleString()}
            formatter={(value) => [formatter(Number(value)), title]}
          />
          <Line
            type="monotone"
            dataKey={dataKey}
            stroke="hsl(var(--primary))"
            strokeWidth={1.5}
            dot={false}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}

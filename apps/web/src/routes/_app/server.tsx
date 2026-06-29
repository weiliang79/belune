import { createFileRoute } from "@tanstack/react-router";
import { RouteError } from "@/lib/components/route-error";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { StatusPill } from "@/components/ui/status-pill";
import { Sparkline } from "@/components/ui/sparkline";
import {
  Select,
  SelectTrigger,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import {
  useMetrics,
  useTriggerCleanup,
  useHostHistoricalMetrics,
  useHostMetricsStream,
  useServerServices,
} from "@/lib/hooks/use-metrics";
import { useSettings, useUpdateSettings } from "@/lib/hooks/use-settings";
import { toast } from "sonner";
import { useMemo, useState } from "react";
import type { HostMetricPoint, SettingEntry } from "@/lib/types";
import { UPlotAreaChart } from "@/components/ui/uplot-area-chart";
import { cn } from "@/lib/utils";

export const Route = createFileRoute("/_app/server")({
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

function pct(part: number | null, total: number | null) {
  if (!part || !total) return 0;
  return Math.round((part / total) * 100);
}

/** nominal < 75% · elevated 75–90% · critical ≥ 90%. */
function loadStatus(percent: number) {
  if (percent >= 90) return { label: "critical", className: "text-status-error" };
  if (percent >= 75)
    return { label: "elevated", className: "text-status-building" };
  return { label: "nominal", className: "text-status-ready" };
}

const CPU_COLOR = "hsl(221, 83%, 53%)";
const MEM_COLOR = "hsl(262, 83%, 58%)";
const DISK_COLOR = "hsl(142, 71%, 45%)";

// Preset retention windows; "0" = keep forever.
const RETENTION_PRESETS = [
  { label: "7 days", value: "7" },
  { label: "30 days", value: "30" },
  { label: "90 days", value: "90" },
  { label: "1 year", value: "365" },
  { label: "Forever", value: "0" },
];

const RETENTION_FIELDS = [
  {
    key: "metrics_retention_days",
    label: "Container metrics",
    desc: "CPU, memory, network time-series",
    fallback: "14",
  },
  {
    key: "app_log_retention_days",
    label: "Application logs",
    desc: "stdout / stderr per container",
    fallback: "7",
  },
  {
    key: "audit_log_retention_days",
    label: "Audit log",
    desc: "Member actions and system events",
    fallback: "0",
  },
  {
    key: "deploy_history_retention_days",
    label: "Deploy history",
    desc: "Build logs and deployment records",
    fallback: "90",
  },
] as const;

function ServerSettingsPage() {
  const { data: metrics, isLoading } = useMetrics();
  const { data: services } = useServerServices();
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

  // Most recent point with real data, for the compact metric cards.
  const latest = useMemo(
    () => [...hostMetrics].reverse().find((p) => p.cpu_percent != null),
    [hostMetrics],
  );

  const { data: settings } = useSettings();
  const updateSettings = useUpdateSettings();

  const handleCleanup = () => {
    toast.promise(cleanup.mutateAsync(undefined), {
      loading: "Running cleanup...",
      success: "Cleanup task queued",
      error: "Failed to trigger cleanup",
    });
  };

  const handleSaveRetention = (key: string, value: string) => {
    toast.promise(updateSettings.mutateAsync([{ key, value }]), {
      loading: "Saving...",
      success: "Retention setting saved",
      error: "Failed to save retention setting",
    });
  };

  const currentInstanceName =
    settings?.find((s) => s.key === "instance_name")?.value ?? "";
  const [instanceNameDraft, setInstanceNameDraft] = useState<string | null>(null);
  const instanceNameValue = instanceNameDraft ?? currentInstanceName;

  const handleSaveInstanceName = () => {
    toast.promise(
      updateSettings
        .mutateAsync([{ key: "instance_name", value: instanceNameValue.trim() }])
        .then(() => setInstanceNameDraft(null)),
      {
        loading: "Saving...",
        success: "Instance name saved",
        error: "Failed to save instance name",
      },
    );
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Server</h1>
        <p className="text-muted-foreground text-sm">
          Platform health, resources, and maintenance.
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Instance</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          <Label htmlFor="instance-name">Instance name</Label>
          <div className="flex max-w-md items-center gap-2">
            <Input
              id="instance-name"
              value={instanceNameValue}
              onChange={(e) => setInstanceNameDraft(e.target.value)}
              placeholder="Self-Hosted PaaS"
            />
            <Button
              onClick={handleSaveInstanceName}
              disabled={
                updateSettings.isPending ||
                instanceNameValue.trim() === currentInstanceName.trim()
              }
            >
              Save
            </Button>
          </div>
          <p className="text-muted-foreground text-xs">
            Shown in the sidebar and used as the default GitHub App name when
            connecting a provider.
          </p>
        </CardContent>
      </Card>

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
            <StatCard
              title="Applications"
              value={metrics.applications}
              subtitle={`${metrics.containers.by_type?.application?.running ?? 0} running`}
            />
            <StatCard title="Databases" value={metrics.databases} />
            <StatCard
              title="Deployments"
              value={metrics.deployments}
              subtitle="all time"
            />
          </div>

          <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle>Containers</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center gap-4">
                  <ContainerStat
                    value={metrics.containers.running}
                    label="Running"
                    variant="default"
                  />
                  <ContainerStat
                    value={metrics.containers.stopped}
                    label="Stopped"
                    variant="secondary"
                  />
                  <ContainerStat
                    value={metrics.containers.error ?? 0}
                    label="Error"
                    variant={
                      (metrics.containers.error ?? 0) > 0
                        ? "destructive"
                        : "outline"
                    }
                  />
                </div>
                {Object.keys(metrics.containers.by_type ?? {}).length > 0 && (
                  <div className="space-y-1.5 border-t pt-3">
                    {Object.entries(metrics.containers.by_type ?? {}).map(
                      ([type, c]) => (
                        <div
                          key={type}
                          className="flex items-center justify-between text-sm"
                        >
                          <span className="text-muted-foreground capitalize">
                            {type}
                          </span>
                          <span className="font-mono">
                            {c.running}
                            <span className="text-text-faint">/{c.total}</span>
                          </span>
                        </div>
                      ),
                    )}
                  </div>
                )}
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <div className="flex items-center justify-between">
                  <CardTitle>Services</CardTitle>
                  {services && (
                    <Badge
                      variant={
                        services.healthy === services.total
                          ? "default"
                          : "destructive"
                      }
                    >
                      {services.healthy}/{services.total} healthy
                    </Badge>
                  )}
                </div>
              </CardHeader>
              <CardContent className="space-y-3">
                {services?.services.map((s) => (
                  <div
                    key={s.name}
                    className="flex items-center justify-between"
                  >
                    <div>
                      <p className="text-sm font-medium">{s.name}</p>
                      <p className="text-muted-foreground text-sm">
                        {s.description}
                      </p>
                    </div>
                    <StatusPill
                      status={s.status === "running" ? "running" : "error"}
                      label={s.status === "running" ? "Healthy" : "Down"}
                    />
                  </div>
                ))}
                {!services && (
                  <p className="text-muted-foreground text-sm">
                    Checking services…
                  </p>
                )}
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
            <CardContent className="space-y-6">
              {latest && (
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
                  <HostMetricCard
                    label="CPU"
                    value={`${(latest.cpu_percent ?? 0).toFixed(0)}%`}
                    percent={latest.cpu_percent ?? 0}
                    values={hostMetrics.map((p) => p.cpu_percent)}
                    color={CPU_COLOR}
                  />
                  <HostMetricCard
                    label="Memory"
                    value={formatBytes(latest.memory_used)}
                    percent={pct(latest.memory_used, latest.memory_total)}
                    values={hostMetrics.map((p) => p.memory_used)}
                    color={MEM_COLOR}
                  />
                  <HostMetricCard
                    label="Disk"
                    value={formatBytes(latest.disk_used)}
                    percent={pct(latest.disk_used, latest.disk_total)}
                    values={hostMetrics.map((p) => p.disk_used)}
                    color={DISK_COLOR}
                  />
                </div>
              )}

              {hostMetrics && hostMetrics.length > 0 ? (
                <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
                  <HostChart
                    title="CPU Usage (%)"
                    data={hostMetrics}
                    dataKey="cpu_percent"
                    range="1h"
                    color={CPU_COLOR}
                    formatter={(v: number) => `${v.toFixed(1)}%`}
                    domain={[0, 100]}
                  />
                  <HostChart
                    title="Memory Usage"
                    data={hostMetrics}
                    dataKey="memory_used"
                    range="1h"
                    color={MEM_COLOR}
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
                    color={DISK_COLOR}
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
            <CardContent className="space-y-4">
              {RETENTION_FIELDS.map((field) => (
                <RetentionRow
                  key={field.key}
                  field={field}
                  settings={settings}
                  disabled={updateSettings.isPending}
                  onSave={handleSaveRetention}
                />
              ))}
              <p className="text-muted-foreground border-t pt-3 text-xs">
                1-second data is kept for 1h, rolled up to 1-minute for 24h,
                5-minute for 7 days, then hourly until the retention limit.
                "Forever" disables pruning for that category.
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

function StatCard({
  title,
  value,
  subtitle,
}: {
  title: string;
  value: number;
  subtitle?: string;
}) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <p className="text-muted-foreground text-sm font-medium">{title}</p>
      </CardHeader>
      <CardContent>
        <p className="text-3xl font-bold">{value}</p>
        {subtitle && <p className="text-text-faint mt-1 text-xs">{subtitle}</p>}
      </CardContent>
    </Card>
  );
}

function ContainerStat({
  value,
  label,
  variant,
}: {
  value: number;
  label: string;
  variant: "default" | "secondary" | "outline" | "destructive";
}) {
  return (
    <div className="flex items-center gap-2">
      <Badge variant={variant}>{value}</Badge>
      <span className="text-muted-foreground text-sm">{label}</span>
    </div>
  );
}

function HostMetricCard({
  label,
  value,
  percent,
  values,
  color,
}: {
  label: string;
  value: string;
  percent: number;
  values: (number | null)[];
  color: string;
}) {
  const s = loadStatus(percent);
  return (
    <Card>
      <CardContent className="p-4">
        <div className="flex items-center justify-between">
          <p className="text-text-faint text-xs font-medium tracking-wide uppercase">
            {label}
          </p>
          <span className={cn("text-xs font-medium", s.className)}>
            {s.label}
          </span>
        </div>
        <p className="mt-1 font-mono text-2xl font-semibold">{value}</p>
        <Sparkline className="mt-2" height={36} values={values} color={color} />
      </CardContent>
    </Card>
  );
}

function RetentionRow({
  field,
  settings,
  disabled,
  onSave,
}: {
  field: { key: string; label: string; desc: string; fallback: string };
  settings: SettingEntry[] | undefined;
  disabled: boolean;
  onSave: (key: string, value: string) => void;
}) {
  const current =
    settings?.find((s) => s.key === field.key)?.value ?? field.fallback;

  // Surface the current value even if it isn't one of the presets.
  const presets = [...RETENTION_PRESETS];
  if (!presets.some((p) => p.value === current)) {
    presets.unshift({
      label: current === "0" ? "Forever" : `${current} days`,
      value: current,
    });
  }
  const currentLabel =
    presets.find((p) => p.value === current)?.label ?? current;

  return (
    <div className="flex items-center justify-between gap-4">
      <div>
        <p className="text-sm font-medium">{field.label}</p>
        <p className="text-muted-foreground text-xs">{field.desc}</p>
      </div>
      <Select
        value={current}
        onValueChange={(v) => v && v !== current && onSave(field.key, v)}
        disabled={disabled}
      >
        <SelectTrigger className="w-32">
          {/* Render the label explicitly: base-ui's items-based lookup
              mis-resolves dynamically-prepended non-preset values. */}
          <span className="truncate">{currentLabel}</span>
        </SelectTrigger>
        <SelectContent>
          {presets.map((p) => (
            <SelectItem key={p.value} value={p.value}>
              {p.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
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

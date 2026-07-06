import { createFileRoute, useNavigate } from "@tanstack/react-router";
import {
  CalendarIcon,
  ClockIcon,
  CpuIcon,
  HardDriveIcon,
  MemoryStickIcon,
  ServerIcon,
} from "lucide-react";
import { RouteError } from "@/lib/components/route-error";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { LiveIndicator } from "@/components/ui/live-indicator";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { StatusPill } from "@/components/ui/status-pill";
import { Sparkline } from "@/components/ui/sparkline";
import { MetricCard } from "@/lib/components/stats/stat-card";
import { PageHeader } from "@/components/ui/page-header";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  Select,
  SelectTrigger,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import {
  useMetrics,
  useHostHistoricalMetrics,
  useHostMetricsStream,
  useHostMetricsRange,
  useServerServices,
} from "@/lib/hooks/use-metrics";
import { useSettings, useUpdateSettings } from "@/lib/hooks/use-settings";
import { toast } from "sonner";
import { useMemo, useState, type ReactNode } from "react";
import type { HostMetricPoint, SettingEntry } from "@/lib/types";
import { UPlotAreaChart } from "@/components/ui/uplot-area-chart";
import { SystemBackupsPanel } from "@/components/backups/system-backups-panel";
import { MaintenanceSection } from "@/components/server/maintenance-section";
import { cn } from "@/lib/utils";
import { formatDateTimeShort } from "@/lib/utils/format";

type ServerTab = "metric" | "configuration" | "backups";
type HostMetricsView = "overview" | "detail";
type CustomRange = { from: string; to: string };

export const Route = createFileRoute("/_app/server")({
  component: ServerSettingsPage,
  errorComponent: RouteError,
  validateSearch: (search: Record<string, unknown>) => ({
    tab:
      search.tab === "configuration" || search.tab === "backups"
        ? (search.tab as ServerTab)
        : undefined,
  }),
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

// Live host-metrics window: the chart shows the most recent 10 minutes.
const TEN_MIN_MS = 10 * 60 * 1000;

// Day-based retention presets for the log rows. "0" = keep forever.
const RETENTION_PRESETS = [
  { label: "3 days", value: "3" },
  { label: "7 days", value: "7" },
  { label: "30 days", value: "30" },
  { label: "90 days", value: "90" },
  { label: "1 year", value: "365" },
  { label: "Forever", value: "0" },
];

// Host metrics are 1-second data, so they use a short hours-based window.
const HOST_METRICS_PRESETS = [
  { label: "1 day", value: "24" },
  { label: "2 days", value: "48" },
];

const RETENTION_FIELDS = [
  {
    key: "app_log_retention_days",
    label: "Application logs",
    desc: "stdout / stderr per container",
    fallback: "7",
  },
  {
    key: "request_log_retention_days",
    label: "Request logs",
    desc: "HTTP access logs per application",
    fallback: "3",
  },
] as const;

function ServerSettingsPage() {
  const { tab } = Route.useSearch();
  const navigate = useNavigate({ from: "/server" });
  const activeTab: ServerTab =
    tab === "configuration" || tab === "backups" ? tab : "metric";
  const setTab = (next: ServerTab) =>
    navigate({
      search: () => ({ tab: next === "metric" ? undefined : next }),
    });

  const { data: metrics, isLoading } = useMetrics();
  const { data: services } = useServerServices();

  // Host Metrics viewer state: Overview (sparklines) vs Detail (charts), and the
  // time window — null = Live (last 10 min, streamed), otherwise a static snapshot.
  const [hostView, setHostView] = useState<HostMetricsView>("overview");
  const [customRange, setCustomRange] = useState<CustomRange | null>(null);

  // Overview is always live; only Detail honors the custom time range, so the
  // live sources stay enabled regardless of the selected window.
  const { data: historicalData } = useHostHistoricalMetrics("1h");
  const { data: streamData, connected: streamConnected } =
    useHostMetricsStream(true);
  const { data: rangeData } = useHostMetricsRange(
    customRange?.from,
    customRange?.to,
  );

  // Live 10-minute window: Overview sparklines, and Detail when unfiltered.
  const liveMetrics = useMemo(() => {
    const ONE_SECOND = 1_000;
    // eslint-disable-next-line react-hooks/purity
    const now = Date.now();
    const maxStart = Math.floor((now - TEN_MIN_MS) / ONE_SECOND) * ONE_SECOND;

    // Both sources are 1s granularity — merge into a single 1s lookup
    const dataMap = new Map<number, HostMetricPoint>();
    for (const point of [...(historicalData ?? []), ...(streamData ?? [])]) {
      const t =
        Math.floor(new Date(point.recorded_at).getTime() / ONE_SECOND) *
        ONE_SECOND;
      dataMap.set(t, point);
    }

    // Start from the oldest real data point so there's no empty void on the
    // left, capped at the 10-minute live window
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

  // Detail plots the custom window when filtered, else the live grid.
  const detailMetrics = customRange ? (rangeData ?? []) : liveMetrics;

  // Most recent live point, for the Overview metric cards.
  const latest = useMemo(
    () => [...liveMetrics].reverse().find((p) => p.cpu_percent != null),
    [liveMetrics],
  );

  const { data: settings } = useSettings();
  const updateSettings = useUpdateSettings();

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

  // In a custom window, format x-axis labels with date+time when the span exceeds
  // a day; otherwise time-only keeps short windows readable.
  const chartRange = useMemo(() => {
    if (!customRange) return "1h";
    const spanMs =
      new Date(customRange.to).getTime() - new Date(customRange.from).getTime();
    return spanMs > 24 * 60 * 60 * 1000 ? "7d" : "24h";
  }, [customRange]);

  // Overview is always live; Detail is live unless a custom window is applied.
  const showingLive = hostView === "overview" || customRange === null;

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<ServerIcon className="size-5" />}
        title="Server"
        description="Platform health, resources, and maintenance."
      />

      <nav className="flex gap-1 border-b">
        {(
          [
            { value: "metric", label: "Overview" },
            { value: "backups", label: "Backups" },
            { value: "configuration", label: "Configuration" },
          ] as const
        ).map((t) => (
          <button
            key={t.value}
            type="button"
            onClick={() => setTab(t.value)}
            className={cn(
              "border-b-2 px-4 py-2 text-sm font-medium transition-colors",
              activeTab === t.value
                ? "border-primary text-foreground"
                : "text-muted-foreground hover:text-foreground border-transparent",
            )}
          >
            {t.label}
          </button>
        ))}
      </nav>

      {activeTab === "backups" ? (
        <SystemBackupsPanel />
      ) : activeTab === "configuration" ? (
        <div className="space-y-6">
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

          <Card>
            <CardHeader>
              <CardTitle>Metrics Retention</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <RetentionRow
                field={{
                  key: "host_metrics_retention_hours",
                  label: "Host metrics (1-second)",
                  desc: "CPU / memory / disk time-series",
                  fallback: "24",
                }}
                presets={HOST_METRICS_PRESETS}
                settings={settings}
                disabled={updateSettings.isPending}
                onSave={handleSaveRetention}
              />
              {RETENTION_FIELDS.map((field) => (
                <RetentionRow
                  key={field.key}
                  field={field}
                  presets={RETENTION_PRESETS}
                  settings={settings}
                  disabled={updateSettings.isPending}
                  onSave={handleSaveRetention}
                />
              ))}
              <p className="text-muted-foreground border-t pt-3 text-xs">
                Host metrics are stored at 1-second granularity and pruned hourly
                to the selected window. Logs are pruned daily.
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Maintenance</CardTitle>
            </CardHeader>
            <CardContent>
              <MaintenanceSection />
            </CardContent>
          </Card>
        </div>
      ) : isLoading ? (
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
        <div className="space-y-6">
          <Card>
            <CardHeader>
              <div className="flex flex-wrap items-center justify-between gap-3">
                <CardTitle>Host Metrics</CardTitle>
                <div className="flex items-center gap-2">
                  {/* The time filter only applies to Detail; Overview stays live. */}
                  {hostView === "detail" && (
                    <HostRangeControl
                      value={customRange}
                      onChange={setCustomRange}
                    />
                  )}
                  <ToggleGroup
                    variant="outline"
                    size="sm"
                    value={[hostView]}
                    onValueChange={(v) =>
                      v.length > 0 && setHostView(v[0] as HostMetricsView)
                    }
                    aria-label="Host metrics view"
                  >
                    <ToggleGroupItem value="overview">Overview</ToggleGroupItem>
                    <ToggleGroupItem value="detail">Detail</ToggleGroupItem>
                  </ToggleGroup>
                  {showingLive ? (
                    <LiveIndicator
                      active={streamConnected}
                      idleLabel="Connecting…"
                    />
                  ) : (
                    <Badge variant="outline">Snapshot</Badge>
                  )}
                </div>
              </div>
            </CardHeader>
            <CardContent className="space-y-6">
              {hostView === "overview" ? (
                latest ? (
                  <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
                    <HostMetricCard
                      label="CPU"
                      icon={<CpuIcon className="size-3.5" />}
                      value={`${(latest.cpu_percent ?? 0).toFixed(0)}%`}
                      percent={latest.cpu_percent ?? 0}
                      values={liveMetrics.map((p) => p.cpu_percent)}
                      color={CPU_COLOR}
                    />
                    <HostMetricCard
                      label="Memory"
                      icon={<MemoryStickIcon className="size-3.5" />}
                      value={formatBytes(latest.memory_used)}
                      percent={pct(latest.memory_used, latest.memory_total)}
                      values={liveMetrics.map((p) => p.memory_used)}
                      color={MEM_COLOR}
                    />
                    <HostMetricCard
                      label="Disk"
                      icon={<HardDriveIcon className="size-3.5" />}
                      value={formatBytes(latest.disk_used)}
                      percent={pct(latest.disk_used, latest.disk_total)}
                      values={liveMetrics.map((p) => p.disk_used)}
                      color={DISK_COLOR}
                    />
                  </div>
                ) : (
                  <HostMetricsEmpty live />
                )
              ) : detailMetrics.length > 0 ? (
                <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
                  <HostChart
                    title="CPU Usage (%)"
                    data={detailMetrics}
                    dataKey="cpu_percent"
                    range={chartRange}
                    color={CPU_COLOR}
                    formatter={(v: number) => `${v.toFixed(1)}%`}
                    domain={[0, 100]}
                  />
                  <HostChart
                    title="Memory Usage"
                    data={detailMetrics}
                    dataKey="memory_used"
                    range={chartRange}
                    color={MEM_COLOR}
                    formatter={(v: number) => formatBytes(v)}
                    domain={[
                      0,
                      Math.max(...detailMetrics.map((m) => m.memory_total ?? 0)),
                    ]}
                  />
                  <HostChart
                    title="Disk Usage"
                    data={detailMetrics}
                    dataKey="disk_used"
                    range={chartRange}
                    color={DISK_COLOR}
                    formatter={(v: number) => formatBytes(v)}
                    domain={[
                      0,
                      Math.max(...detailMetrics.map((m) => m.disk_total ?? 0)),
                    ]}
                  />
                </div>
              ) : (
                <HostMetricsEmpty live={customRange === null} />
              )}
            </CardContent>
          </Card>

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
        </div>
      ) : null}
    </div>
  );
}

function HostMetricsEmpty({ live }: { live: boolean }) {
  return (
    <p className="text-muted-foreground py-8 text-center text-sm">
      {live
        ? "No metrics data available yet. Data is collected every second."
        : "No metrics in the selected window."}
    </p>
  );
}

/** datetime-local string (local time, minute precision) for a Date. */
function toLocalInputValue(d: Date) {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(
    d.getHours(),
  )}:${pad(d.getMinutes())}`;
}

function formatRangeLabel(iso: string) {
  return formatDateTimeShort(iso);
}

function HostRangeControl({
  value,
  onChange,
}: {
  value: CustomRange | null;
  onChange: (next: CustomRange | null) => void;
}) {
  const [open, setOpen] = useState(false);
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");

  const handleOpenChange = (next: boolean) => {
    if (next) {
      const now = new Date();
      const start = value ? new Date(value.from) : new Date(now.getTime() - TEN_MIN_MS);
      const end = value ? new Date(value.to) : now;
      setFrom(toLocalInputValue(start));
      setTo(toLocalInputValue(end));
    }
    setOpen(next);
  };

  const apply = () => {
    if (!from || !to) return;
    const fromIso = new Date(from).toISOString();
    const toIso = new Date(to).toISOString();
    if (new Date(fromIso) >= new Date(toIso)) {
      toast.error("Start time must be before end time");
      return;
    }
    onChange({ from: fromIso, to: toIso });
    setOpen(false);
  };

  const goLive = () => {
    onChange(null);
    setOpen(false);
  };

  return (
    <Popover open={open} onOpenChange={handleOpenChange}>
      <PopoverTrigger
        render={<Button variant="outline" size="sm" className="gap-1.5" />}
      >
        <CalendarIcon className="size-3.5 shrink-0" />
        <span className="max-w-[200px] truncate">
          {value
            ? `${formatRangeLabel(value.from)} – ${formatRangeLabel(value.to)}`
            : "Live (10m)"}
        </span>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-72 space-y-3">
        <div className="space-y-1.5">
          <Label htmlFor="metric-from" className="text-xs">
            Start
          </Label>
          <Input
            id="metric-from"
            type="datetime-local"
            value={from}
            onChange={(e) => setFrom(e.target.value)}
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="metric-to" className="text-xs">
            End
          </Label>
          <Input
            id="metric-to"
            type="datetime-local"
            value={to}
            onChange={(e) => setTo(e.target.value)}
          />
        </div>
        <div className="flex items-center justify-between gap-2 pt-1">
          <Button variant="ghost" size="sm" onClick={goLive} disabled={!value}>
            Live
          </Button>
          <Button size="sm" onClick={apply}>
            Apply
          </Button>
        </div>
      </PopoverContent>
    </Popover>
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
  icon,
  value,
  percent,
  values,
  color,
}: {
  label: string;
  icon?: ReactNode;
  value: string;
  percent: number;
  values: (number | null)[];
  color: string;
}) {
  const s = loadStatus(percent);
  return (
    <MetricCard
      label={label}
      icon={icon}
      aside={
        <span className={cn("text-xs font-medium capitalize", s.className)}>
          {s.label}
        </span>
      }
    >
      <p className="mt-1 font-mono text-2xl font-semibold">{value}</p>
      <Sparkline className="mt-2" height={36} values={values} color={color} />
    </MetricCard>
  );
}

function RetentionRow({
  field,
  presets,
  settings,
  disabled,
  onSave,
}: {
  field: { key: string; label: string; desc: string; fallback: string };
  presets: { label: string; value: string }[];
  settings: SettingEntry[] | undefined;
  disabled: boolean;
  onSave: (key: string, value: string) => void;
}) {
  const current =
    settings?.find((s) => s.key === field.key)?.value ?? field.fallback;

  // Surface the current value even if it isn't one of the presets.
  const options = [...presets];
  if (!options.some((p) => p.value === current)) {
    options.unshift({ label: current, value: current });
  }
  const currentLabel =
    options.find((p) => p.value === current)?.label ?? current;

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
          <span className="truncate capitalize">{currentLabel}</span>
        </SelectTrigger>
        <SelectContent>
          {options.map((p) => (
            <SelectItem
              key={p.value}
              value={p.value}
              icon={<ClockIcon />}
              className="capitalize"
            >
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
        syncKey="host-metrics-detail"
      />
    </div>
  );
}

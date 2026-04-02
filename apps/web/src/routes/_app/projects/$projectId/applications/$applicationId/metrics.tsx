import { createFileRoute } from "@tanstack/react-router";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { useAppMetricsContext } from "@/lib/contexts/app-metrics-context";
import type { AppMetricPoint } from "@/lib/types";
import { UPlotAreaChart } from "@/components/ui/uplot-area-chart";

export const Route = createFileRoute(
  "/_app/projects/$projectId/applications/$applicationId/metrics",
)({
  component: ApplicationMetricsPage,
});

function formatTime(ts: number) {
  return new Date(ts).toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

function formatBytes(bytes: number | null) {
  if (bytes == null) return "N/A";
  const gb = bytes / (1024 * 1024 * 1024);
  if (gb >= 1) return `${gb.toFixed(1)} GB`;
  const mb = bytes / (1024 * 1024);
  if (mb >= 1) return `${mb.toFixed(0)} MB`;
  const kb = bytes / 1024;
  return `${kb.toFixed(0)} KB`;
}

function ApplicationMetricsPage() {
  const { data: streamData, connected } = useAppMetricsContext();

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <CardTitle>Application Metrics</CardTitle>
            <Badge variant={connected ? "default" : "secondary"}>
              {connected ? "LIVE" : "Connecting..."}
            </Badge>
          </div>
          <p className="text-muted-foreground text-xs">
            Real-time data — collected while you view this page
          </p>
        </div>
      </CardHeader>
      <CardContent>
        {streamData && streamData.length > 0 ? (
          <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
            <AppChart
              title="CPU Usage (%)"
              data={streamData}
              dataKey="cpu_percent"
              color="hsl(221, 83%, 53%)"
              formatter={(v: number) => `${v.toFixed(1)}%`}
            />
            <AppChart
              title="Memory Usage"
              data={streamData}
              dataKey="memory_usage"
              color="hsl(262, 83%, 58%)"
              formatter={(v: number) => formatBytes(v)}
            />
            <AppChart
              title="Network RX"
              data={streamData}
              dataKey="network_rx_bytes"
              color="hsl(142, 71%, 45%)"
              formatter={(v: number) => formatBytes(v)}
            />
            <AppChart
              title="Network TX"
              data={streamData}
              dataKey="network_tx_bytes"
              color="hsl(47, 100%, 50%)"
              formatter={(v: number) => formatBytes(v)}
            />
          </div>
        ) : (
          <p className="text-muted-foreground py-8 text-center text-sm">
            Waiting for metrics data. Data streams in real-time every 2 seconds
            for running applications.
          </p>
        )}
      </CardContent>
    </Card>
  );
}

function AppChart({
  title,
  data,
  dataKey,
  color = "hsl(var(--primary))",
  formatter,
}: {
  title: string;
  data: AppMetricPoint[];
  dataKey: string;
  color?: string;
  formatter: (value: number) => string;
}) {
  return (
    <div>
      <p className="mb-2 text-sm font-medium">{title}</p>
      <UPlotAreaChart
        timestamps={data.map((p) => new Date(p.recorded_at).getTime())}
        values={data.map(
          (p) => p[dataKey as keyof AppMetricPoint] as number | null,
        )}
        color={color}
        yFormatter={formatter}
        xFormatter={(ts) => formatTime(ts)}
        height={200}
      />
    </div>
  );
}

import { createFileRoute } from "@tanstack/react-router";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import {
  useApplicationMetrics,
  useAppMetricsStream,
} from "@/lib/hooks/use-metrics";
import type { AppMetricPoint } from "@/lib/types";
import { UPlotAreaChart } from "@/components/ui/uplot-area-chart";
import { useMemo } from "react";

export const Route = createFileRoute(
  "/_app/projects/$projectId/applications/$applicationId/metrics",
)({
  component: ApplicationMetricsPage,
});

function formatTime(ts: number) {
  return new Date(ts).toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
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
  const { projectId, applicationId } = Route.useParams();
  const { data: historicalData } = useApplicationMetrics(
    projectId,
    applicationId,
    "1h",
  );
  const { data: streamData, connected: streamConnected } = useAppMetricsStream(
    projectId,
    applicationId,
    true,
  );

  const metrics = useMemo(() => {
    const ONE_SECOND = 1_000;
    const THIRTY_MIN = 30 * 60 * 1_000;
    // eslint-disable-next-line react-hooks/purity
    const now = Date.now();
    const start = Math.floor((now - THIRTY_MIN) / ONE_SECOND) * ONE_SECOND;

    const dataMap = new Map<number, AppMetricPoint>();
    for (const point of [...(historicalData ?? []), ...(streamData ?? [])]) {
      const t =
        Math.floor(new Date(point.recorded_at).getTime() / ONE_SECOND) *
        ONE_SECOND;
      dataMap.set(t, point);
    }

    const grid: AppMetricPoint[] = [];
    for (let t = start; t <= now; t += ONE_SECOND) {
      grid.push(
        dataMap.get(t) ?? {
          recorded_at: new Date(t).toISOString(),
          cpu_percent: 0,
          memory_usage: 0,
          memory_limit: 0,
          network_rx_bytes: 0,
          network_tx_bytes: 0,
        },
      );
    }
    return grid;
  }, [historicalData, streamData]);

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <CardTitle>Application Metrics</CardTitle>
          <Badge variant={streamConnected ? "default" : "secondary"}>
            {streamConnected ? "LIVE" : "Connecting..."}
          </Badge>
        </div>
      </CardHeader>
      <CardContent>
        {metrics && metrics.length > 0 ? (
          <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
            <AppChart
              title="CPU Usage (%)"
              data={metrics}
              dataKey="cpu_percent"
              color="hsl(221, 83%, 53%)"
              formatter={(v: number) => `${v.toFixed(1)}%`}
            />
            <AppChart
              title="Memory Usage"
              data={metrics}
              dataKey="memory_usage"
              color="hsl(262, 83%, 58%)"
              formatter={(v: number) => formatBytes(v)}
            />
            <AppChart
              title="Network RX"
              data={metrics}
              dataKey="network_rx_bytes"
              color="hsl(142, 71%, 45%)"
              formatter={(v: number) => formatBytes(v)}
            />
            <AppChart
              title="Network TX"
              data={metrics}
              dataKey="network_tx_bytes"
              color="hsl(47, 100%, 50%)"
              formatter={(v: number) => formatBytes(v)}
            />
          </div>
        ) : (
          <p className="text-muted-foreground py-8 text-center text-sm">
            No metrics data available yet. Data is collected every second for
            running applications.
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

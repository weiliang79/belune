import { createFileRoute } from "@tanstack/react-router";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { useApplicationMetrics } from "@/lib/hooks/use-metrics";
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
import type { AppMetricPoint } from "@/lib/types";

export const Route = createFileRoute(
  "/_app/projects/$projectId/applications/$applicationId/metrics",
)({
  component: ApplicationMetricsPage,
});

const RANGE_OPTIONS = ["1h", "24h", "7d", "30d"] as const;

function formatTime(iso: string, range: string) {
  const d = new Date(iso);
  if (range === "1h" || range === "24h")
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
  if (mb >= 1) return `${mb.toFixed(0)} MB`;
  const kb = bytes / 1024;
  return `${kb.toFixed(0)} KB`;
}

function ApplicationMetricsPage() {
  const { projectId, applicationId } = Route.useParams();
  const [range, setRange] = useState<string>("1h");
  const { data: metrics } = useApplicationMetrics(
    projectId,
    applicationId,
    range,
  );

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>Application Metrics</CardTitle>
          <div className="flex gap-1">
            {RANGE_OPTIONS.map((r) => (
              <Button
                key={r}
                size="sm"
                variant={range === r ? "default" : "outline"}
                onClick={() => setRange(r)}
              >
                {r}
              </Button>
            ))}
          </div>
        </div>
      </CardHeader>
      <CardContent>
        {metrics && metrics.length > 0 ? (
          <div className="space-y-6">
            <AppChart
              title="CPU Usage (%)"
              data={metrics}
              dataKey="cpu_percent"
              range={range}
              formatter={(v: number) => `${v.toFixed(1)}%`}
              domain={[0, "auto"]}
            />
            <AppChart
              title="Memory Usage"
              data={metrics}
              dataKey="memory_usage"
              range={range}
              formatter={(v: number) => formatBytes(v)}
            />
            <AppChart
              title="Network RX"
              data={metrics}
              dataKey="network_rx_bytes"
              range={range}
              formatter={(v: number) => formatBytes(v)}
              color="hsl(var(--chart-2, 142 71% 45%))"
            />
            <AppChart
              title="Network TX"
              data={metrics}
              dataKey="network_tx_bytes"
              range={range}
              formatter={(v: number) => formatBytes(v)}
              color="hsl(var(--chart-3, 47 100% 50%))"
            />
          </div>
        ) : (
          <p className="text-muted-foreground py-8 text-center text-sm">
            No metrics data available yet. Data is collected every minute for
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
  range,
  formatter,
  domain,
  color = "hsl(var(--primary))",
}: {
  title: string;
  data: AppMetricPoint[];
  dataKey: string;
  range: string;
  formatter: (value: number) => string;
  domain?: [number | string, number | string];
  color?: string;
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
            stroke={color}
            strokeWidth={1.5}
            dot={false}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}

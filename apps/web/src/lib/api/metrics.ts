import type { MetricsOverview, HostMetricPoint } from "@/lib/types";
import { api } from "./client";

export function getMetrics() {
  return api.get<MetricsOverview>("/metrics");
}

export function triggerCleanup(retainCount?: number) {
  return api.post<{ status: string }>("/cleanup", { retain_count: retainCount ?? 3 });
}

export function getHostHistoricalMetrics(range: string) {
  return api.get<HostMetricPoint[]>(`/metrics/host?range=${range}`);
}

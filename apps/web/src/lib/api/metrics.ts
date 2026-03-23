import type { MetricsOverview, HostMetricPoint, AppMetricPoint } from "@/lib/types";
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

export function getApplicationMetrics(projectId: string, applicationId: string, range: string) {
  return api.get<AppMetricPoint[]>(
    `/projects/${projectId}/applications/${applicationId}/metrics?range=${range}`,
  );
}

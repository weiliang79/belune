import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useState } from "react";
import { queryKeys } from "./query-keys";
import { useSSEWithReconnect } from "./use-sse";
import * as metricsApi from "@/lib/api/metrics";
import type { HostMetricPoint, AppMetricPoint } from "@/lib/types";

const STREAM_WINDOW_MS = 30 * 60 * 1000; // 30 minutes

export function useMetrics() {
  return useQuery({
    queryKey: queryKeys.metrics,
    queryFn: metricsApi.getMetrics,
    refetchInterval: 30_000,
  });
}

export function useTriggerCleanup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (retainCount?: number) => metricsApi.triggerCleanup(retainCount),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.metrics }),
  });
}

export function useHostHistoricalMetrics(range: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.hostMetrics(range),
    queryFn: () => metricsApi.getHostHistoricalMetrics(range),
    refetchInterval: 60_000,
    enabled,
  });
}

export function useHostMetricsStream(enabled: boolean) {
  const [data, setData] = useState<HostMetricPoint[]>([]);

  const handleMessage = useCallback((raw: string) => {
    const point: HostMetricPoint = JSON.parse(raw);
    setData((prev) => {
      const cutoff = new Date(Date.now() - STREAM_WINDOW_MS).toISOString();
      return [...prev, point].filter((p) => p.recorded_at >= cutoff);
    });
  }, []);

  const url = enabled ? "/api/metrics/host/stream" : null;
  const { connected } = useSSEWithReconnect(url, handleMessage);

  useEffect(() => {
    if (!enabled) setData([]);
  }, [enabled]);

  return { data, connected };
}

export function useAppMetricsStream(
  projectId: string,
  applicationId: string,
  enabled: boolean,
) {
  const [data, setData] = useState<AppMetricPoint[]>([]);

  const handleMessage = useCallback((raw: string) => {
    const point: AppMetricPoint = JSON.parse(raw);
    setData((prev) => {
      const cutoff = new Date(Date.now() - STREAM_WINDOW_MS).toISOString();
      return [...prev, point].filter((p) => p.recorded_at >= cutoff);
    });
  }, []);

  const url = enabled
    ? `/api/projects/${projectId}/applications/${applicationId}/metrics/stream`
    : null;
  const { connected } = useSSEWithReconnect(url, handleMessage);

  useEffect(() => {
    if (!enabled) setData([]);
  }, [enabled]);

  return { data, connected };
}

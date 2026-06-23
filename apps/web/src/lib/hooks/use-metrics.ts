import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useState } from "react";
import { queryKeys } from "./query-keys";
import { useChannel } from "./use-websocket";
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

export function useServerServices() {
  return useQuery({
    queryKey: queryKeys.serverServices,
    queryFn: metricsApi.getServerServices,
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

  const handleMessage = useCallback((_event: string, raw: unknown) => {
    try {
      const point = (typeof raw === "string" ? JSON.parse(raw) : raw) as HostMetricPoint;
      setData((prev) => {
        const cutoff = new Date(Date.now() - STREAM_WINDOW_MS).toISOString();
        return [...prev, point].filter((p) => p.recorded_at >= cutoff);
      });
    } catch (err) {
      // Per-message parse failure: log without toasting (would spam the stream).
      console.debug("host metrics stream: failed to parse message", err);
    }
  }, []);

  const channel = enabled ? "metrics:host" : null;
  const { connected } = useChannel(channel, handleMessage);

  useEffect(() => {
    if (!enabled) setData([]);
  }, [enabled]);

  return { data, connected };
}

export function useAppMetricsStream(
  _projectId: string,
  applicationId: string,
  enabled: boolean,
) {
  const [data, setData] = useState<AppMetricPoint[]>([]);

  const handleMessage = useCallback((_event: string, raw: unknown) => {
    try {
      const point = (typeof raw === "string" ? JSON.parse(raw) : raw) as AppMetricPoint;
      setData((prev) => {
        const cutoff = new Date(Date.now() - STREAM_WINDOW_MS).toISOString();
        return [...prev, point].filter((p) => p.recorded_at >= cutoff);
      });
    } catch (err) {
      // Per-message parse failure: log without toasting (would spam the stream).
      console.debug("app metrics stream: failed to parse message", err);
    }
  }, []);

  const channel = enabled ? `metrics:app:${applicationId}` : null;
  const { connected } = useChannel(channel, handleMessage);

  useEffect(() => {
    if (!enabled) setData([]);
  }, [enabled]);

  return { data, connected };
}

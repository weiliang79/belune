import { useQuery } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import { api } from "@/lib/api/client";
import type { ProjectMetrics } from "@/lib/types";

/**
 * Per-service runtime snapshot (CPU/mem/uptime/domain/port), polled while open.
 *
 * ⚠️ `refetchIntervalMs` is the caller's because this is the single most
 * expensive query in the app and not every call site needs live numbers. At the
 * 5s default it is 12 requests/minute, measured — more than a third of what a
 * whole page spent before this, and the largest line in every page the
 * request-budget drill looked at (see sessionRateLimit in
 * internal/server/routes.go). A call site rendering CPU and memory bars earns
 * that; one rendering an uptime string does not.
 */
export function useProjectMetrics(projectId: string, refetchIntervalMs = 5000) {
  return useQuery({
    queryKey: queryKeys.projectMetrics(projectId),
    queryFn: () => api.get<ProjectMetrics>(`/projects/${projectId}/metrics`),
    refetchInterval: refetchIntervalMs,
  });
}

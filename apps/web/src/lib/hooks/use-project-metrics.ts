import { useQuery } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import { api } from "@/lib/api/client";
import type { ProjectMetrics } from "@/lib/types";

/** Per-service runtime snapshot (CPU/mem/uptime/domain/port), polled while open. */
export function useProjectMetrics(projectId: string) {
  return useQuery({
    queryKey: queryKeys.projectMetrics(projectId),
    queryFn: () => api.get<ProjectMetrics>(`/projects/${projectId}/metrics`),
    refetchInterval: 5000,
  });
}

import { useQuery } from "@tanstack/react-query";
import * as logsApi from "@/lib/api/container-logs";
import type {
  ContainerLogParams,
  ContainerLogSource,
} from "@/lib/api/container-logs";
import { queryKeys } from "./query-keys";

export function useContainerLogs(
  source: ContainerLogSource,
  projectId: string,
  sourceId: string,
  params?: ContainerLogParams,
) {
  return useQuery({
    queryKey: queryKeys.containerLogs.history(source, projectId, sourceId, params),
    queryFn: () => logsApi.listContainerLogs(source, projectId, sourceId, params),
  });
}

export function useContainerLogSessions(
  source: ContainerLogSource,
  projectId: string,
  sourceId: string,
) {
  return useQuery({
    queryKey: queryKeys.containerLogs.sessions(source, projectId, sourceId),
    queryFn: () => logsApi.listContainerLogSessions(source, projectId, sourceId),
    // Sessions change as new deploys land; keep them reasonably fresh but don't
    // hammer — a manual refresh or navigation will pick up new ones.
    refetchInterval: 30_000,
  });
}

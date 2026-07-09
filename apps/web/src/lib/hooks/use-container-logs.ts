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

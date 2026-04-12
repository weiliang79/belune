import { useQuery } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as logsApi from "@/lib/api/application-logs";
import type { ApplicationLogParams } from "@/lib/api/application-logs";

export function useApplicationLogs(
  projectId: string,
  applicationId: string,
  params?: ApplicationLogParams,
) {
  return useQuery({
    queryKey: queryKeys.applicationLogs.history(projectId, applicationId, params),
    queryFn: () => logsApi.listApplicationLogs(projectId, applicationId, params),
  });
}

import { useQuery } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as logsApi from "@/lib/api/application-logs";

export function useApplicationLogs(
  projectId: string,
  applicationId: string,
  params?: { limit?: number; offset?: number },
) {
  return useQuery({
    queryKey: queryKeys.applicationLogs.history(projectId, applicationId),
    queryFn: () => logsApi.listApplicationLogs(projectId, applicationId, params),
  });
}

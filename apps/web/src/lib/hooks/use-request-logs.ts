import { useQuery } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as requestLogsApi from "@/lib/api/request-logs";

export function useRequestLogs(params?: { limit?: number; offset?: number }) {
  return useQuery({
    queryKey: queryKeys.requestLogs(params?.limit, params?.offset),
    queryFn: () => requestLogsApi.listRequestLogs(params),
  });
}

import { useQuery } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as requestLogsApi from "@/lib/api/request-logs";
import type { RequestLogFilters } from "@/lib/api/request-logs";

export function useRequestLogs(params?: RequestLogFilters) {
  return useQuery({
    queryKey: queryKeys.requestLogs(params),
    queryFn: () => requestLogsApi.listRequestLogs(params),
  });
}

export function useRequestSummary(params?: RequestLogFilters) {
  return useQuery({
    queryKey: queryKeys.requestSummary(params),
    queryFn: () => requestLogsApi.getRequestSummary(params),
  });
}

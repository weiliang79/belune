import { useQuery } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as auditApi from "@/lib/api/audit-logs";

export function useAuditLogs(params: { limit: number; offset: number }) {
  return useQuery({
    queryKey: queryKeys.auditLogs(params),
    queryFn: () => auditApi.listAuditLogs(params),
  });
}

import { useQuery } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as auditApi from "@/lib/api/audit-logs";
import type { AuditLogFilters } from "@/lib/api/audit-logs";

export function useAuditLogs(params: AuditLogFilters) {
  return useQuery({
    queryKey: queryKeys.auditLogs(params),
    queryFn: () => auditApi.listAuditLogs(params),
  });
}

export function useAuditActions() {
  return useQuery({
    queryKey: queryKeys.auditActions,
    queryFn: auditApi.listAuditActions,
    staleTime: 60_000,
  });
}

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { queryKeys } from "./query-keys";
import * as quotasApi from "@/lib/api/quotas";
import type { QuotaLimits, QuotaScope } from "@/lib/api/quotas";

export function useQuotas() {
  return useQuery({
    queryKey: queryKeys.quotas.all,
    queryFn: quotasApi.listQuotas,
    refetchInterval: 15000,
  });
}

export function useUpsertQuota() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      scope,
      scopeId,
      limits,
    }: {
      scope: QuotaScope;
      scopeId: string;
      limits: QuotaLimits;
    }) => quotasApi.upsertQuota(scope, scopeId, limits),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.quotas.all }),
    onError: (err) => toast.error(err.message),
  });
}

export function useDeleteQuota() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ scope, scopeId }: { scope: QuotaScope; scopeId: string }) =>
      quotasApi.deleteQuota(scope, scopeId),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.quotas.all }),
    onError: (err) => toast.error(err.message),
  });
}

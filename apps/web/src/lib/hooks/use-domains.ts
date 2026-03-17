import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as domainsApi from "@/lib/api/domains";

export function useDomains(projectId: string, serviceId: string) {
  return useQuery({
    queryKey: queryKeys.domains.all(projectId, serviceId),
    queryFn: () => domainsApi.listDomains(projectId, serviceId),
  });
}

export function useAddDomain(projectId: string, serviceId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { hostname: string; ssl_enabled: boolean }) =>
      domainsApi.addDomain(projectId, serviceId, data),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: queryKeys.domains.all(projectId, serviceId),
      }),
  });
}

export function useRemoveDomain(projectId: string, serviceId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (domainId: string) =>
      domainsApi.removeDomain(projectId, serviceId, domainId),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: queryKeys.domains.all(projectId, serviceId),
      }),
  });
}

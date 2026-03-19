import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as domainsApi from "@/lib/api/domains";

export function useDomains(projectId: string, applicationId: string) {
  return useQuery({
    queryKey: queryKeys.domains.all(projectId, applicationId),
    queryFn: () => domainsApi.listDomains(projectId, applicationId),
  });
}

export function useAddDomain(projectId: string, applicationId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { hostname: string; ssl_enabled: boolean }) =>
      domainsApi.addDomain(projectId, applicationId, data),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: queryKeys.domains.all(projectId, applicationId),
      }),
  });
}

export function useRemoveDomain(projectId: string, applicationId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (domainId: string) =>
      domainsApi.removeDomain(projectId, applicationId, domainId),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: queryKeys.domains.all(projectId, applicationId),
      }),
  });
}

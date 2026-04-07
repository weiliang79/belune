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
    mutationFn: (data: Parameters<typeof domainsApi.addDomain>[2]) =>
      domainsApi.addDomain(projectId, applicationId, data),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: queryKeys.domains.all(projectId, applicationId),
      }),
  });
}

export function useUpdateDomain(projectId: string, applicationId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      domainId,
      ...data
    }: { domainId: string } & Parameters<typeof domainsApi.updateDomain>[3]) =>
      domainsApi.updateDomain(projectId, applicationId, domainId, data),
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

export function useRouteFeatures(
  projectId: string,
  applicationId: string,
  domainId: string,
) {
  return useQuery({
    queryKey: queryKeys.routeFeatures(projectId, applicationId, domainId),
    queryFn: () =>
      domainsApi.listRouteFeatures(projectId, applicationId, domainId),
    enabled: !!domainId,
  });
}

export function useUpsertRouteFeature(
  projectId: string,
  applicationId: string,
) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      domainId,
      ...data
    }: {
      domainId: string;
      feature_type: string;
      config: Record<string, unknown>;
      enabled: boolean;
    }) => domainsApi.upsertRouteFeature(projectId, applicationId, domainId, data),
    onSuccess: () => {
      qc.invalidateQueries({
        queryKey: queryKeys.domains.all(projectId, applicationId),
      });
    },
  });
}

export function useDeleteRouteFeature(
  projectId: string,
  applicationId: string,
) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      domainId,
      featureId,
    }: {
      domainId: string;
      featureId: string;
    }) => domainsApi.deleteRouteFeature(projectId, applicationId, domainId, featureId),
    onSuccess: () => {
      qc.invalidateQueries({
        queryKey: queryKeys.domains.all(projectId, applicationId),
      });
    },
  });
}
